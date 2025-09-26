// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/zookeeper/probe.sh
var zookeeperProbe string

//go:embed static/zookeeper/run.sh
var zookeeperRun string

//go:embed static/zookeeper/fluent-bit.conf.tmpl
var zkFluentBitForwarderConfig string

//go:embed static/zookeeper/logback.xml
var zkLogbackConfig string

const zkSSLPortName = "zkssl"
const zkSSLPort = 2281

const ZookeeperIdent = "zookeeper"
const zkPIMountPath = "/config-scripts"

func createZKLogForwarderSidecar(r *SFController, annotations map[string]string) ([]apiv1.Volume, apiv1.Container) {

	fbForwarderConfig := make(map[string]string)
	var loggingParams = logging.CreateForwarderConfigTemplateParams("zookeeper", r.cr.Spec.FluentBitLogForwarding)

	fbForwarderConfig["fluent-bit.conf"], _ = utils.ParseString(
		zkFluentBitForwarderConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{[]logging.FluentBitLabel{}, loggingParams})
	r.EnsureConfigMap("fluentbit-zk-cfg", fbForwarderConfig)

	volume := base.MkVolumeCM("zk-log-forwarder-config",
		"fluentbit-zk-cfg-config-map")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      ZookeeperIdent + "-logs",
			MountPath: "/watch/",
		},
		{
			Name:      "zk-log-forwarder-config",
			MountPath: "/fluent-bit/etc/",
		},
	}
	sidecar, storageEmptyDir := logging.CreateFluentBitSideCarContainer("zookeeper", []logging.FluentBitLabel{}, volumeMounts, r.isOpenShift)
	annotations["zk-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	annotations["zk-fluent-bit-image"] = sidecar.Image
	return []apiv1.Volume{volume, storageEmptyDir}, sidecar
}

func (r *SFController) DeployZookeeper() bool {
	cmData := make(map[string]string)
	cmData["probe.sh"] = zookeeperProbe
	cmData["run.sh"] = zookeeperRun
	cmData["logback.xml"] = zkLogbackConfig
	r.EnsureConfigMap(ZookeeperIdent+"-pi", cmData)

	configChecksumable := zookeeperProbe + "\n" + zookeeperRun + "\n" + zkLogbackConfig

	annotations := map[string]string{
		"config-hash": utils.Checksum([]byte(configChecksumable)),
		"image":       base.ZookeeperImage(),
		"serial":      "8",
	}

	volumeMountsStatsExporter := []apiv1.VolumeMount{
		{
			Name:      ZookeeperIdent + "-data",
			MountPath: "/data",
		},
		{
			Name:      ZookeeperIdent + "-logs",
			MountPath: "/var/log",
		},
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
			Name:      ZookeeperIdent + "-conf",
			MountPath: "/conf",
		},
		{
			Name:      ZookeeperIdent + "-pi",
			MountPath: zkPIMountPath,
		},
	}
	volumeMounts = append(volumeMounts, volumeMountsStatsExporter...)

	srv := base.MkServicePod(ZookeeperIdent, r.ns, ZookeeperIdent+"-0", []int32{zkSSLPort}, ZookeeperIdent, r.cr.Spec.ExtraLabels)
	r.EnsureService(&srv)

	storageConfig := r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage)
	logStorageConfig := base.StorageConfig{
		Size:             utils.Qty1Gi(),
		StorageClassName: storageConfig.StorageClassName,
		ExtraAnnotations: storageConfig.ExtraAnnotations,
	}
	zk := r.mkHeadlessStatefulSet(
		ZookeeperIdent, base.ZookeeperImage(), storageConfig, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.isOpenShift)
	// We overwrite the VolumeClaimTemplates set by MkHeadlessStatefulSet to keep the previous volume name
	// Previously the default PVC created by MkHeadlessStatefulSet was not used by Zookeeper (not mounted). So to avoid having two Volumes
	// and to ensure data persistence during the upgrade we keep the previous naming 'ZookeeperIdent + "-data"' and we discard the one
	// created by MkHeadlessStatefulSet.
	zk.Spec.VolumeClaimTemplates = []apiv1.PersistentVolumeClaim{
		base.MkPVC(ZookeeperIdent+"-data", r.ns, storageConfig, apiv1.ReadWriteOnce),
		base.MkPVC(ZookeeperIdent+"-logs", r.ns, logStorageConfig, apiv1.ReadWriteOnce),
	}
	zk.Spec.Template.Spec.Containers[0].Command = []string{"/config-scripts/run.sh"}
	zk.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	zk.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret("zookeeper-server-tls"),
		base.MkEmptyDirVolume(ZookeeperIdent + "-conf"),
		{
			Name: ZookeeperIdent + "-pi",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: ZookeeperIdent + "-pi-config-map",
					},
					Items: []apiv1.KeyToPath{
						{Key: "run.sh", Path: "run.sh", Mode: &utils.Execmod},
						{Key: "probe.sh", Path: "probe.sh", Mode: &utils.Execmod},
						{Key: "logback.xml", Path: "logback.xml", Mode: &utils.Readmod},
					},
				},
			},
		},
	}
	zk.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessCMDProbe([]string{"/bin/bash", "/config-scripts/probe.sh"})
	zk.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLivenessCMDProbe([]string{"/bin/bash", "/config-scripts/probe.sh"})
	zk.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zkSSLPort, zkSSLPortName),
	}
	base.SetContainerLimitsHighProfile(&zk.Spec.Template.Spec.Containers[0])
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zookeeper.Limits, &zk.Spec.Template.Spec.Containers[0])

	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolumes, fbSidecar := createZKLogForwarderSidecar(r, annotations)
		zk.Spec.Template.Spec.Containers = append(zk.Spec.Template.Spec.Containers, fbSidecar)
		zk.Spec.Template.Spec.Volumes = append(zk.Spec.Template.Spec.Volumes, fbVolumes...)
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(ZookeeperIdent, volumeMountsStatsExporter, r.isOpenShift)
	zk.Spec.Template.Spec.Containers = append(zk.Spec.Template.Spec.Containers, statsExporter)

	zk.Spec.Template.ObjectMeta.Annotations = annotations

	zk.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	current, changed := r.ensureStatefulset(storageConfig.StorageClassName, zk)
	if changed {
		r.zkChanged = true
		return false
	}

	// VolumeClaimTemplates cannot be updated on a statefulset. If we are missing the logs PVC we must
	// recreate from scratch the statefulset. The new statefulset should mount the old PVC again.
	if len(current.Spec.VolumeClaimTemplates) < 2 {
		logging.LogI("Zookeeper volume claim templates changed, recreating statefulset ...")
		r.DeleteR(current)
		return false
	}

	pvcReadiness := r.reconcileExpandPVC(ZookeeperIdent+"-data-"+ZookeeperIdent+"-0", r.cr.Spec.Zookeeper.Storage)

	isReady := r.IsStatefulSetReady(current) && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, ZookeeperIdent, isReady)

	if isReady && r.zkChanged {
		logging.LogI("Running reconnect-zk.py on the scheduler to force a reconnection.")
		r.RunPodCmd("zuul-scheduler-0", "zuul-scheduler", []string{"/usr/local/bin/reconnect-zk.py"})
		r.zkChanged = false
	}
	return isReady
}
