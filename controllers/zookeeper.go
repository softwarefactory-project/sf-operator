// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"strconv"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/cert"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/zookeeper/ok.sh
var zookeeperOk string

//go:embed static/zookeeper/ready.sh
var zookeeperReady string

//go:embed static/zookeeper/run.sh
var zookeeperRun string

//go:embed static/zookeeper/fluent-bit.conf.tmpl
var zkFluentBitForwarderConfig string

//go:embed static/zookeeper/logback.xml
var zkLogbackConfig string

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

func createZKLogForwarderSidecar(r *SFController, annotations map[string]string) (apiv1.Volume, apiv1.Container) {

	fbForwarderConfig := make(map[string]string)
	fbForwarderConfig["fluent-bit.conf"], _ = utils.ParseString(
		zkFluentBitForwarderConfig,
		struct {
			ExtraKeys              []logging.FluentBitLabel
			FluentBitHTTPInputHost string
			FluentBitHTTPInputPort string
		}{[]logging.FluentBitLabel{}, r.cr.Spec.FluentBitLogForwarding.HTTPInputHost, strconv.Itoa(int(r.cr.Spec.FluentBitLogForwarding.HTTPInputPort))})
	r.EnsureConfigMap("fluentbit-zk-cfg", fbForwarderConfig)

	volume := base.MkVolumeCM("zk-log-forwarder-config",
		"fluentbit-zk-cfg-config-map")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      zkIdent + "-logs",
			MountPath: "/watch/",
		},
		{
			Name:      "zk-log-forwarder-config",
			MountPath: "/fluent-bit/etc/",
		},
	}
	sidecar := logging.CreateFluentBitSideCarContainer("zookeeper", []logging.FluentBitLabel{}, volumeMounts)
	annotations["zk-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	return volume, sidecar
}

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
	cmData["run.sh"] = zookeeperRun
	cmData["logback.xml"] = zkLogbackConfig
	r.EnsureConfigMap(zkIdent+"-pi", cmData)

	configChecksumable := zookeeperOk + "\n" + zookeeperReady + "\n" + zookeeperRun + "\n" + zkLogbackConfig

	annotations := map[string]string{
		"configuration": utils.Checksum([]byte(configChecksumable)),
		"image":         base.ZookeeperImage,
		"serial":        "2",
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
		{
			Name:      zkIdent + "-logs",
			MountPath: "/var/log",
		},
	}

	srv := base.MkServicePod(zkIdent, r.ns, zkIdent+"-0", []int32{zkSSLPort}, zkIdent)
	r.EnsureService(&srv)

	srvZK := base.MkHeadlessServicePod(zkIdent, r.ns, zkIdent+"-0", []int32{zkSSLPort, zkElectionPort, zkServerPort}, zkIdent)
	r.EnsureService(&srvZK)

	storageConfig := r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage)
	logStorageConfig := base.StorageConfig{
		Size:             utils.Qty1Gi(),
		StorageClassName: storageConfig.StorageClassName,
	}
	zk := r.mkHeadlessSatefulSet(
		zkIdent, base.ZookeeperImage, storageConfig, apiv1.ReadWriteOnce)
	// We overwrite the VolumeClaimTemplates set by MkHeadlessStatefulSet to keep the previous volume name
	// Previously the default PVC created by MkHeadlessStatefulSet was not used by Zookeeper (not mounted). So to avoid having two Volumes
	// and to ensure data persistence during the upgrade we keep the previous naming 'zkIdent + "-data"' and we discard the one
	// created by MkHeadlessStatefulSet.
	zk.Spec.VolumeClaimTemplates = []apiv1.PersistentVolumeClaim{
		base.MkPVC(zkIdent+"-data", r.ns, storageConfig, apiv1.ReadWriteOnce),
		base.MkPVC(zkIdent+"-logs", r.ns, logStorageConfig, apiv1.ReadWriteOnce),
	}
	zk.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "/config-scripts/run.sh"}
	zk.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

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

	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolume, fbSidecar := createZKLogForwarderSidecar(r, annotations)
		zk.Spec.Template.Spec.Containers = append(zk.Spec.Template.Spec.Containers, fbSidecar)
		zk.Spec.Template.Spec.Volumes = append(zk.Spec.Template.Spec.Volumes, fbVolume)
	}
	zk.Spec.Template.ObjectMeta.Annotations = annotations

	current := appsv1.StatefulSet{}
	if r.GetM(zkIdent, &current) {
		// VolumeClaimTemplates cannot be updated on a statefulset. If we are missing the logs PVC we must
		// recreate from scratch the statefulset. The new statefulset should mount the old PVC again.
		if len(current.Spec.VolumeClaimTemplates) < 2 {
			r.log.V(1).Info("Zookeeper volume claim templates changed, recreating statefulset ...")
			r.DeleteR(&current)
			return false
		}
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
