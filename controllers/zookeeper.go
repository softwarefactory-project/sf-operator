// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"fmt"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

//go:embed static/zookeeper/probe.sh
var zookeeperProbe string

//go:embed static/zookeeper/run.sh
var zookeeperRun string

//go:embed static/zookeeper/fluent-bit.conf.tmpl
var zkFluentBitForwarderConfig string

//go:embed static/zookeeper/logback.xml
var zkLogbackConfig string

const zkClientPort = 2281
const zkElectionPort = 3888
const zkServerPort = 2888

const ZookeeperIdent = "zookeeper"
const ZookeeperReplicas = 1

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
	sidecar, storageEmptyDir := logging.CreateFluentBitSideCarContainer("zookeeper", []logging.FluentBitLabel{}, volumeMounts, r.IsOpenShift)
	annotations["zk-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	annotations["zk-fluent-bit-image"] = sidecar.Image
	return []apiv1.Volume{volume, storageEmptyDir}, sidecar
}

func (r *SFController) DeployZookeeper() bool {
	// Setup the Certificate Authority for Zookeeper/Zuul/Nodepool usage
	r.EnsureZookeeperCertificates(ZookeeperIdent, ZookeeperReplicas)

	cmData := make(map[string]string)
	cmData["probe.sh"] = zookeeperProbe
	cmData["run.sh"] = zookeeperRun
	cmData["logback.xml"] = zkLogbackConfig
	r.EnsureConfigMap(ZookeeperIdent+"-pi", cmData)

	configChecksumable := zookeeperProbe + "\n" + zookeeperRun + "\n" + zkLogbackConfig

	annotations := map[string]string{
		"config-hash": utils.Checksum([]byte(configChecksumable)),
		"serial":      "9",
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

	// TODO Use base.MkHeadlessService here if possible
	srvHeadless := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ZookeeperIdent + "-headless",
			Namespace: r.Ns,
			Labels:    r.cr.Spec.ExtraLabels,
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app": "sf",
				"run": ZookeeperIdent,
			},
			Ports: []apiv1.ServicePort{
				{
					Name:       "election",
					Protocol:   apiv1.ProtocolTCP,
					Port:       zkElectionPort,
					TargetPort: intstr.FromInt(zkElectionPort),
				},
				{
					Name:       "server",
					Protocol:   apiv1.ProtocolTCP,
					Port:       zkServerPort,
					TargetPort: intstr.FromInt(zkServerPort),
				},
				{
					Name:       "client",
					Protocol:   apiv1.ProtocolTCP,
					Port:       zkClientPort,
					TargetPort: intstr.FromInt(zkClientPort),
				},
			},
		}}
	r.EnsureService(&srvHeadless)
	// TODO we keep the original service but this could be removed. Zookeeper is assumed to also work behind a load balancer.
	srv := base.MkService(ZookeeperIdent, r.Ns, ZookeeperIdent, []int32{zkClientPort}, ZookeeperIdent, r.cr.Spec.ExtraLabels)
	r.EnsureService(&srv)
	storageConfig := r.getStorageConfOrDefault(r.cr.Spec.Zookeeper.Storage)
	logStorageConfig := base.StorageConfig{
		Size:             utils.Qty1Gi(),
		StorageClassName: storageConfig.StorageClassName,
		ExtraAnnotations: storageConfig.ExtraAnnotations,
	}
	zk := r.mkHeadlessStatefulSet(
		ZookeeperIdent, base.ZookeeperImage(), storageConfig, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.IsOpenShift)
	// We overwrite the VolumeClaimTemplates set by MkHeadlessStatefulSet to keep the previous volume name
	// Previously the default PVC created by MkHeadlessStatefulSet was not used by Zookeeper (not mounted). So to avoid having two Volumes
	// and to ensure data persistence during the upgrade we keep the previous naming 'ZookeeperIdent + "-data"' and we discard the one
	// created by MkHeadlessStatefulSet.
	zk.Spec.VolumeClaimTemplates = []apiv1.PersistentVolumeClaim{
		base.MkPVC(ZookeeperIdent+"-data", r.Ns, storageConfig, apiv1.ReadWriteOnce),
		base.MkPVC(ZookeeperIdent+"-logs", r.Ns, logStorageConfig, apiv1.ReadWriteOnce),
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
		base.MkContainerPort(zkClientPort, "client"),
		base.MkContainerPort(zkElectionPort, "election"),
		base.MkContainerPort(zkServerPort, "server"),
	}
	zk.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		base.MkEnvVar("ZK_REPLICAS", fmt.Sprintf("%d", ZookeeperReplicas)),
	}

	// Delay termination with a sleep to give time to remaining replicas to react to potential leader loss
	execAction := apiv1.ExecAction{
		Command: []string{"/bin/sh", "-c", "pkill java; sleep 60"},
	}
	lfHandler := apiv1.LifecycleHandler{
		Exec: &execAction,
	}
	terminationSettings := apiv1.Lifecycle{
		PreStop: &lfHandler,
	}
	zk.Spec.Template.Spec.Containers[0].Lifecycle = &terminationSettings
	// Grace period is twice the sleep to be sure
	zk.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To[int64](120)

	zk.Spec.Replicas = utils.Int32Ptr(ZookeeperReplicas)
	base.SetContainerLimitsHighProfile(&zk.Spec.Template.Spec.Containers[0])
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zookeeper.Limits, &zk.Spec.Template.Spec.Containers[0])

	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolumes, fbSidecar := createZKLogForwarderSidecar(r, annotations)
		zk.Spec.Template.Spec.Containers = append(zk.Spec.Template.Spec.Containers, fbSidecar)
		zk.Spec.Template.Spec.Volumes = append(zk.Spec.Template.Spec.Volumes, fbVolumes...)
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(ZookeeperIdent, volumeMountsStatsExporter, r.IsOpenShift)
	zk.Spec.Template.Spec.Containers = append(zk.Spec.Template.Spec.Containers, statsExporter)

	zk.Spec.Template.ObjectMeta.Annotations = annotations

	zk.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	replicaCount := int32(ZookeeperReplicas)
	current, changed := r.ensureStatefulset(zk, &replicaCount)
	if changed {
		return false
	}

	// VolumeClaimTemplates cannot be updated on a statefulset. If we are missing the logs PVC we must
	// recreate from scratch the statefulset. The new statefulset should mount the old PVC again.
	if len(current.Spec.VolumeClaimTemplates) < 2 {
		logging.LogI("Zookeeper volume claim templates changed, recreating statefulset ...")
		r.DeleteR(current)
		return false
	}

	pvcReadiness := true
	for i := range ZookeeperReplicas {
		var pvcName = fmt.Sprintf("%s-data-%s-%d", ZookeeperIdent, ZookeeperIdent, i)
		pvcReadiness = pvcReadiness && r.reconcileExpandPVC(pvcName, r.cr.Spec.Zookeeper.Storage)
	}

	isReady := r.IsStatefulSetReady(current) && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, ZookeeperIdent, isReady)

	return isReady
}
