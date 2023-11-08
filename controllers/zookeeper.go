// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/cert"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
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

func (r *SFController) DeployZookeeper() bool {
	dnsNames := r.MkClientDNSNames(zkIdent)
	privateKey := certv1.CertificatePrivateKey{
		Encoding: certv1.PKCS8,
	}
	certificate := cert.MkCertificate(
		"zookeeper-server", r.ns, "ca-issuer", dnsNames, "zookeeper-server-tls", &privateKey)
	certClient := cert.MkCertificate(
		"zookeeper-client", r.ns, "ca-issuer", dnsNames, "zookeeper-client-tls", &privateKey)
	r.GetOrCreate(&certificate)
	r.GetOrCreate(&certClient)

	if !cert.IsCertificateReady(&certificate) || !cert.IsCertificateReady(&certClient) {
		return false
	}

	cmData := make(map[string]string)
	cmData["ok.sh"] = zookeeperOk
	cmData["ready.sh"] = zookeeperReady
	cmData["run.sh"] = zookeepeRun
	r.EnsureConfigMap(zkIdent+"-pi", cmData)

	annotations := map[string]string{
		"ok":     utils.Checksum([]byte(zookeeperOk)),
		"ready":  utils.Checksum([]byte(zookeeperReady)),
		"run":    utils.Checksum([]byte(zookeepeRun)),
		"serial": "1",
	}

	volumeMounts := []apiv1.VolumeMount{
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

	servicePorts := []int32{zkSSLPort}
	srv := base.MkService(zkIdent, r.ns, zkIdent, servicePorts, zkIdent)
	r.GetOrCreate(&srv)

	headlessPorts := []int32{zkSSLPort, zkElectionPort, zkServerPort}
	srvZK := base.MkHeadlessService(zkIdent, r.ns, zkIdent, headlessPorts, zkIdent)
	r.GetOrCreate(&srvZK)

	replicas := int32(1)
	storageConfig := r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage)
	zk := r.mkHeadlessSatefulSet(
		zkIdent, base.ZookeeperImage, storageConfig, replicas, apiv1.ReadWriteOnce)
	// We overwrite the VolumeClaimTemplates set by MkHeadlessStatefulSet to keep the previous volume name
	// Previously the default PVC created by MkHeadlessStatefulSet was not used by Zookeeper (not mounted). So to avoid having two Volumes
	// and to ensure data persistence during the upgrade we keep the previous naming 'zkIdent + "-data"' and we discard the one
	// created by MkHeadlessStatefulSet.
	zk.Spec.VolumeClaimTemplates = []apiv1.PersistentVolumeClaim{base.MkPVC(zkIdent+"-data", r.ns, storageConfig, apiv1.ReadWriteOnce)}
	zk.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "/config-scripts/run.sh"}
	zk.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	zk.Spec.Template.ObjectMeta.Annotations = annotations
	zk.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM(zkIdent+"-pi", zkIdent+"-pi-config-map"),
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret("zookeeper-server-tls"),
		base.MkEmptyDirVolume(zkIdent + "-conf"),
	}
	zk.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessCMDProbe([]string{"/bin/bash", "/config-scripts/ready.sh"})
	zk.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessCMDProbe([]string{"/bin/bash", "/config-scripts/ok.sh"})
	zk.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zkPort, zkPortName),
		base.MkContainerPort(zkSSLPort, zkSSLPortName),
		base.MkContainerPort(zkElectionPort, zkElectionPortName),
		base.MkContainerPort(zkServerPort, zkServerPortName),
	}

	current := appsv1.StatefulSet{}
	if r.GetM(zkIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Zookeeper configuration changed, rollout pods ...")
			current.Spec.Template = zk.DeepCopy().Spec.Template
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zk
		r.CreateR(&current)
	}

	pvcReadiness := r.reconcileExpandPVC(zkIdent+"-data-"+zkIdent+"-0", r.cr.Spec.Zookeeper.Storage)

	isReady := r.IsStatefulSetReady(&current) && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, zkIdent, isReady)

	return isReady
}
