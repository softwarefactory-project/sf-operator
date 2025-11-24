// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
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
const ZookeeperReplicas = 3
const ZookeeperMaxUnavailable = 1

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
	hasChanged := r.EnsureZookeeperCertificates(ZookeeperIdent, ZookeeperReplicas)
	if hasChanged {
		logging.LogI("Zookeeper certs were updated, nuking ZK clients...")
		r.nukeZKClients()
	}

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
	pdb := base.MkPodDisruptionBudget(ZookeeperIdent, r.Ns, ZookeeperMaxUnavailable, map[string]string{"app": "sf", "run": ZookeeperIdent}, nil)
	r.EnsurePodDisruptionBudget(&pdb)
	zk := r.mkHeadlessStatefulSet(
		ZookeeperIdent, base.ZookeeperImage(), storageConfig, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.IsOpenShift)
	zk.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}
	zk.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
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

	// Node Anti-Affinity
	//
	// TODO: changes to this should appear in the dry-run/verbose output, and respect user-defined affinities
	//
	// Also note that to disable this after it's been enabled (but why would you ?) you will have to
	// 1. remove the setting in the SF CR
	// 2. manually edit the zookeeper statefulset to remove the PodAntiAffinity setting
	// This is because we do not want to interfere with custom-set rules that may be set OoB.
	aaEnabled := false
	if r.cr.Spec.Zookeeper.NodeAntiAffinityEnabled != nil {
		aaEnabled = *r.cr.Spec.Zookeeper.NodeAntiAffinityEnabled
	}
	if aaEnabled {
		zkAALabelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "sf",
				"run": ZookeeperIdent,
			},
		}
		zkRequiredDuringSchedulingIgnoredDuringExecution := []apiv1.PodAffinityTerm{
			{
				LabelSelector: &zkAALabelSelector,
				TopologyKey:   "kubernetes.io/hostname",
			},
		}
		zkPodAntiAffinity := apiv1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: zkRequiredDuringSchedulingIgnoredDuringExecution,
		}
		zkAffinity := apiv1.Affinity{
			PodAntiAffinity: &zkPodAntiAffinity,
		}
		zk.Spec.Template.Spec.Affinity = &zkAffinity
	}

	// we want to ensure the replica count is always at 3
	replicaCount := int32(ZookeeperReplicas)

	// If we are bumping from 1 to 3 replicas we need to backup zookeeper first.
	// We also need to do it **only** if the zookeeper statefulset is present.
	// TODO(v0.0.67 or above) The section below should be removed once the replica bump is effective.
	crt := appsv1.StatefulSet{}
	if r.GetOrDie(zk.ObjectMeta.Name, &crt) && !r.checkStatefulsetReplicaCount(zk, &replicaCount) {
		logging.LogI("Zookeeper data must be dumped before replica count increase. Fetching data...")
		buff, errDump := r.dumpAllZKData()
		if errDump != nil {
			logging.LogE(errDump, "Please fix issue with dumping process before reconciling SF resource")
			os.Exit(1)
		}
		logging.LogI("Data successfully dumped. Waiting for zookeeper ensemble to be up...")
		if isZookeeperUp := WaitFor(func() bool {
			sts, changed := r.ensureStatefulset(zk, &replicaCount)
			// wait until sts has been applied, then for all pods to be up
			if changed {
				return false
			}
			readyPods := sts.Status.AvailableReplicas
			return readyPods == replicaCount
		}, true); !isZookeeperUp {
			panic("Zookeeper statefulset failed to deploy. Check kubectl logs to find out why")
		}

		logging.LogI("Re-injecting original data into all replicas...")
		saved := make([]byte, len(buff.Bytes()))
		copy(saved, buff.Bytes())
		if err := r.restoreAllZKData(buff.Bytes()); err != nil {
			logging.LogE(err, "A copy of the original snapshot will be saved to `/tmp/zk-snapshot` before exiting.")
			os.WriteFile("/tmp/zk-snapshot", saved, 0644)
			os.Exit(1)
		}
	}
	// TODO remove above

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

func (r *SFController) dumpAllZKData() (*bytes.Buffer, error) {
	// TODO the admin server is running with default auth values! This is however not a big issue since the admin port is
	// not available from outside of the container.
	cmdArgs := []string{
		"curl",
		"-s",
		"-H",
		"Authorization: digest admin:admin",
		"http://localhost:8080/commands/snapshot?streaming=true",
		"--output",
		"-",
	}
	return r.RunPodCmd(ZookeeperIdent+"-0", ZookeeperIdent, cmdArgs)
}

func (r *SFController) restoreAllZKData(data []byte) error {
	cmdArgs := []string{
		"curl",
		"-s",
		"-H",
		"Authorization: digest admin:admin",
		"-H",
		"Content-Type: application/octet-stream",
		"-X",
		"POST",
		"http://localhost:8080/commands/restore",
		"--data-binary",
		"@/tmp/backup",
	}

	for i := range ZookeeperReplicas {
		var pod = fmt.Sprintf("%s-%d", ZookeeperIdent, i)
		var errCopy error
		copyDone := false
		copyAttempt := 0
		// Give some time to the container to be ready
		for !copyDone {
			copyAttempt += 1
			// There's no "seek" for bytes.buffers and the byte slice is fully consumed each time
			// we read the buffer, so we have to recreate a new buffer with a copy of the data.
			buffer := bytes.NewBuffer(data)
			_, errCopy = r.CopyFileToContainer(pod, ZookeeperIdent, "/tmp/backup", buffer)
			if errCopy != nil {
				logging.LogE(errCopy, fmt.Sprintf("Attempt %d/60 for pod %s...", copyAttempt, pod))
				time.Sleep(time.Second)
			} else {
				copyDone = true
			}
			if copyAttempt >= 60 {
				copyDone = true
			}
		}
		if errCopy != nil {
			return errCopy
		}
		logging.LogI(fmt.Sprintf("Pod %s dump copy operation complete", pod))
		// Zookeeper-0 might not be ready to listen on the admin interface, give it some time
		attempt := 0
		stdoutFlag := ""
		// the restore operation returns a json like
		// {
		//		"last_zxid": 1234,
		//		"command": "restore",
		//		"error": null,
		// }
		// the "error" field is unreliable and can be null despite a failure, for example when attempting
		// a restore during a rate limiting period (5 mins default, see https://zookeeper.apache.org/doc/current/zookeeperSnapshotAndRestore.html)
		// so "last_zxid" is a much better indicator that the restore transaction succeeded.
		for !strings.Contains(stdoutFlag, "last_zxid") {
			attempt += 1
			stdout, err := r.RunPodCmd(pod, ZookeeperIdent, cmdArgs)
			stdoutFlag = stdout.String()
			if err != nil {
				logging.LogE(err, fmt.Sprintf("Pod %s restore operation failed (attempt %d/50): %s, retrying...", pod, attempt, stdoutFlag))
			} else {
				logging.LogD(fmt.Sprintf("Pod %s restore operation returned: %s", pod, stdoutFlag))
			}
			if attempt > 50 {
				panic(fmt.Sprintf("pod %s is not ready", pod))
			}
			time.Sleep(time.Second * time.Duration(attempt))
		}
		r.RunPodCmd(pod, ZookeeperIdent, []string{"rm", "/tmp/backup"})
		logging.LogI(fmt.Sprintf("Pod %s restore operation operation complete", pod))
	}
	return nil
}
