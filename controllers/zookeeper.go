// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/zookeeper/ok.sh
var zookeeper_ok string

//go:embed static/zookeeper/ready.sh
var zookeeper_ready string

//go:embed static/zookeeper/run.sh
var zookeeper_run string

const ZOOKEEPER_PORT_NAME = "zk"
const ZOOKEEPER_PORT = 2181

const ZOOKEEPER_SSL_PORT_NAME = "zkssl"
const ZOOKEEPER_SSL_PORT = 2281

const ZOOKEEPER_ELECTION_PORT_NAME = "zkelection"
const ZOOKEEPER_ELECTION_PORT = 3888

const ZOOKEEPER_SERVER_PORT_NAME = "zkserver"
const ZOOKEEPER_SERVER_PORT = 2888

const ZK_IDENT = "zookeeper"
const ZK_PI_MOUNT_PATH = "/config-scripts"

func (r *SFController) DeployZookeeper() bool {
	cert := r.create_client_certificate("zookeeper-server", "ca-issuer", "zookeeper-server-tls", ZK_IDENT)
	cert_client := r.create_client_certificate("zookeeper-client", "ca-issuer", "zookeeper-client-tls", ZK_IDENT)
	r.GetOrCreate(&cert)
	r.GetOrCreate(&cert_client)

	cm_data := make(map[string]string)
	cm_data["ok.sh"] = zookeeper_ok
	cm_data["ready.sh"] = zookeeper_ready
	cm_data["run.sh"] = zookeeper_run
	r.EnsureConfigMap(ZK_IDENT+"-pi", cm_data)

	annotations := map[string]string{
		"ok":    checksum([]byte(zookeeper_ok)),
		"ready": checksum([]byte(zookeeper_ready)),
		"run":   checksum([]byte(zookeeper_run)),
	}

	volumes := []apiv1.VolumeMount{
		{
			Name:      "zookeeper-server-tls",
			MountPath: "/tls/server",
			ReadOnly:  true,
		},
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      ZK_IDENT + "-data",
			MountPath: "/data",
		},
		{
			Name:      ZK_IDENT + "-conf",
			MountPath: "/conf",
		},
		{
			Name:      ZK_IDENT + "-pi",
			MountPath: ZK_PI_MOUNT_PATH,
		},
	}

	container := apiv1.Container{
		Name:            ZK_IDENT,
		Image:           "quay.io/software-factory/" + ZK_IDENT + ":3.8.0-5",
		Command:         []string{"/bin/bash", "/config-scripts/run.sh"},
		SecurityContext: create_security_context(false),
		VolumeMounts:    volumes,
	}

	service_ports := []int32{ZOOKEEPER_SSL_PORT}
	srv := r.create_service(ZK_IDENT, ZK_IDENT, service_ports, ZK_IDENT)
	r.GetOrCreate(&srv)

	headless_ports := []int32{ZOOKEEPER_SSL_PORT, ZOOKEEPER_ELECTION_PORT, ZOOKEEPER_SERVER_PORT}
	srv_zk := r.create_headless_service(ZK_IDENT, ZK_IDENT, headless_ports, ZK_IDENT)
	r.GetOrCreate(&srv_zk)

	zk := r.create_headless_statefulset(ZK_IDENT, "", get_storage_classname(r.cr.Spec))
	zk.Spec.VolumeClaimTemplates = append(
		zk.Spec.VolumeClaimTemplates,
		r.create_pvc(ZK_IDENT+"-data", get_storage_classname(r.cr.Spec)))
	zk.Spec.Template.Spec.Containers = []apiv1.Container{container}
	zk.Spec.Template.ObjectMeta.Annotations = annotations
	zk.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm(ZK_IDENT+"-pi", ZK_IDENT+"-pi-config-map"),
		create_volume_secret("zookeeper-client-tls"),
		create_volume_secret("zookeeper-server-tls"),
		create_empty_dir(ZK_IDENT + "-conf"),
	}
	zk.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"/bin/bash", "/config-scripts/ready.sh"})
	zk.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_cmd_probe([]string{"/bin/bash", "/config-scripts/ok.sh"})
	zk.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(ZOOKEEPER_PORT, ZOOKEEPER_PORT_NAME),
		create_container_port(ZOOKEEPER_SSL_PORT, ZOOKEEPER_SSL_PORT_NAME),
		create_container_port(ZOOKEEPER_ELECTION_PORT, ZOOKEEPER_ELECTION_PORT_NAME),
		create_container_port(ZOOKEEPER_SERVER_PORT, ZOOKEEPER_SERVER_PORT_NAME),
	}

	r.GetOrCreate(&zk)
	zk_dirty := false
	if !map_equals(&zk.Spec.Template.ObjectMeta.Annotations, &annotations) {
		zk.Spec.Template.ObjectMeta.Annotations = annotations
		zk_dirty = true
	}
	if zk_dirty {
		r.UpdateR(&zk)
	}

	return r.IsStatefulSetReady(&zk)
}
