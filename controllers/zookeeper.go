// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/zookeeper/ok.sh
var zookeeperOk string

//go:embed static/zookeeper/ready.sh
var zookeeperReady string

//go:embed static/zookeeper/run.sh
var zookeepeRun string

const zkPortName = "zk"
const zkPort = 2181

const zkSSLPortName = "zkssl"
const zkSSLPort = 2281

const zkElectionPortName = "zkelection"
const zkElectionPort = 3888

const zkServerPortName = "zkserver"
const zkServerPort = 2888

const zkIdent = "zookeeper"
const zkPIMountPath = "/config-scripts"

const zkImage = "quay.io/software-factory/" + zkIdent + ":3.8.0-5"

func (r *SFController) DeployZookeeper() bool {
	dnsNames := r.MKClientDNSNames(zkIdent)
	privateKey := certv1.CertificatePrivateKey{
		Encoding: certv1.PKCS8,
	}
	cert := MKCertificate(
		"zookeeper-server", r.ns, "ca-issuer", dnsNames, "zookeeper-server-tls", &privateKey)
	certClient := MKCertificate(
		"zookeeper-client", r.ns, "ca-issuer", dnsNames, "zookeeper-client-tls", &privateKey)
	r.GetOrCreate(&cert)
	r.GetOrCreate(&certClient)

	if !isCertificateReady(&cert) || !isCertificateReady(&certClient) {
		return false
	}

	cmData := make(map[string]string)
	cmData["ok.sh"] = zookeeperOk
	cmData["ready.sh"] = zookeeperReady
	cmData["run.sh"] = zookeepeRun
	r.EnsureConfigMap(zkIdent+"-pi", cmData)

	annotations := map[string]string{
		"ok":    checksum([]byte(zookeeperOk)),
		"ready": checksum([]byte(zookeeperReady)),
		"run":   checksum([]byte(zookeepeRun)),
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
			Name:      zkIdent + "-data",
			MountPath: "/data",
		},
		{
			Name:      zkIdent + "-conf",
			MountPath: "/conf",
		},
		{
			Name:      zkIdent + "-pi",
			MountPath: zkPIMountPath,
		},
	}

	container := MkContainer(zkIdent, zkImage)
	container.Command = []string{"/bin/bash", "/config-scripts/run.sh"}
	container.VolumeMounts = volumes

	servicePorts := []int32{zkSSLPort}
	srv := r.mkService(zkIdent, zkIdent, servicePorts, zkIdent)
	r.GetOrCreate(&srv)

	headlessPorts := []int32{zkSSLPort, zkElectionPort, zkServerPort}
	srvZK := r.mkHeadlessService(zkIdent, zkIdent, headlessPorts, zkIdent)
	r.GetOrCreate(&srvZK)

	replicas := int32(1)
	zk := r.mkHeadlessSatefulSet(zkIdent, "", r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage), replicas, apiv1.ReadWriteOnce)
	zk.Spec.VolumeClaimTemplates = append(
		zk.Spec.VolumeClaimTemplates,
		r.MkPVC(zkIdent+"-data", r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage), apiv1.ReadWriteOnce))
	zk.Spec.Template.Spec.Containers = []apiv1.Container{container}
	zk.Spec.Template.ObjectMeta.Annotations = annotations
	zk.Spec.Template.Spec.Volumes = []apiv1.Volume{
		MKVolumeCM(zkIdent+"-pi", zkIdent+"-pi-config-map"),
		mkVolumeSecret("zookeeper-client-tls"),
		mkVolumeSecret("zookeeper-server-tls"),
		mkEmptyDirVolume(zkIdent + "-conf"),
	}
	zk.Spec.Template.Spec.Containers[0].ReadinessProbe = MkReadinessCMDProbe([]string{"/bin/bash", "/config-scripts/ready.sh"})
	zk.Spec.Template.Spec.Containers[0].LivenessProbe = MkReadinessCMDProbe([]string{"/bin/bash", "/config-scripts/ok.sh"})
	zk.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		MKContainerPort(zkPort, zkPortName),
		MKContainerPort(zkSSLPort, zkSSLPortName),
		MKContainerPort(zkElectionPort, zkElectionPortName),
		MKContainerPort(zkServerPort, zkServerPortName),
	}

	r.GetOrCreate(&zk)
	zkDirty := false
	if !mapEquals(&zk.Spec.Template.ObjectMeta.Annotations, &annotations) {
		zk.Spec.Template.ObjectMeta.Annotations = annotations
		zkDirty = true
	}
	if zkDirty {
		if !r.UpdateR(&zk) {
			return false
		}
	}

	isStatefulSet := r.IsStatefulSetReady(&zk)
	updateConditions(&r.cr.Status.Conditions, zkIdent, isStatefulSet)

	return isStatefulSet
}
