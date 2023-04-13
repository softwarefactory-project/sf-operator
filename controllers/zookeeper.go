// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/zookeeper/ok
var zookeeper_ok string

//go:embed static/zookeeper/ready
var zookeeper_ready string

//go:embed static/zookeeper/run
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
const ZK_DATA_MOUNT_PATH = "/data"

func (r *SFController) DeployZookeeper() bool {
	cert := r.create_client_certificate(r.ns, "zookeeper-server", "ca-issuer", "zookeeper-server-tls", "zookeeper")
	cert_client := r.create_client_certificate(r.ns, "zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper")
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
			MountPath: ZK_DATA_MOUNT_PATH,
		},
		{
			Name:      ZK_IDENT + "-pi",
			MountPath: ZK_PI_MOUNT_PATH,
		},
	}

	container := apiv1.Container{
		Name:    "zookeeper",
		Image:   "quay.io/software-factory/" + ZK_IDENT + ":3.8.0-2",
		Command: []string{"/bin/bash", "/config-scripts/run.sh"},
		Env: []apiv1.EnvVar{
			create_env("ZK_REPLICAS", "1"),
			create_env("JMXAUTH", "false"),
			create_env("JMXDISABLE", "false"),
			create_env("JMXPORT", "1099"),
			create_env("JMXSSL", "false"),
			create_env("ZK_SYNC_LIMIT", "10"),
			create_env("ZK_TICK_TIME", "2000"),
			create_env("ZOO_AUTOPURGE_PURGEINTERVAL", "0"),
			create_env("ZOO_AUTOPURGE_SNAPRETAINCOUNT", "3"),
			create_env("ZOO_INIT_LIMIT", "5"),
			create_env("ZOO_MAX_CLIENT_CNXNS", "60"),
			create_env("ZOO_PORT", "2181"),
			create_env("ZOO_STANDALONE_ENABLED", "false"),
			create_env("ZOO_TICK_TIME", "2000"),
		},
		SecurityContext: &defaultContainerSecurityContext,
		VolumeMounts:    volumes,
	}

	service_ports := []int32{ZOOKEEPER_SSL_PORT}
	srv := create_service(r.ns, "zookeeper", "zookeeper", service_ports, "zookeeper")
	r.GetOrCreate(&srv)

	headless_ports := []int32{ZOOKEEPER_SSL_PORT, ZOOKEEPER_ELECTION_PORT, ZOOKEEPER_SERVER_PORT}
	srv_zk := create_headless_service(r.ns, "zookeeper", "zookeeper", headless_ports, "zookeeper")
	r.GetOrCreate(&srv_zk)

	zk := create_headless_statefulset(r.ns, "zookeeper", "", get_storage_classname(r.cr.Spec))
	zk.Spec.VolumeClaimTemplates = append(
		zk.Spec.VolumeClaimTemplates,
		create_pvc(r.ns, ZK_IDENT+"-data", get_storage_classname(r.cr.Spec)))
	zk.Spec.Template.Spec.Containers = []apiv1.Container{container}
	zk.Spec.Template.ObjectMeta.Annotations = annotations
	zk.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm(ZK_IDENT+"-pi", ZK_IDENT+"-pi-config-map"),
		create_volume_secret("zookeeper-client-tls"),
		create_volume_secret("zookeeper-server-tls"),
	}
	zk.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"/bin/bash", "/config-scripts/ready.sh"})
	zk.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_cmd_probe([]string{"/bin/bash", "/config-scripts/ok.sh"})
	zk.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(ZOOKEEPER_PORT, ZOOKEEPER_PORT_NAME),
		create_container_port(ZOOKEEPER_SSL_PORT, ZOOKEEPER_SSL_PORT_NAME),
		create_container_port(ZOOKEEPER_ELECTION_PORT, ZOOKEEPER_ELECTION_PORT_NAME),
		create_container_port(ZOOKEEPER_SERVER_PORT, ZOOKEEPER_SERVER_PORT_NAME),
	}
	zk.Spec.Template.Spec.SecurityContext = &defaultPodSecurityContext

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
