// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	_ "embed"
	"encoding/base64"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

const logserverIdent = "logserver"
const httpdPort = 8080
const httpdPortName = "logserver-httpd"

//go:embed static/logserver/logserver-entrypoint.sh
var logserverEntrypoint string

const sshdPort = 2222
const sshdPortName = "logserver-sshd"

//go:embed static/logserver/run.sh
var logserverRun string

const purgelogIdent = "purgelogs"
const purgelogsLogsDir = "/home/logs"

//go:embed static/logserver/logserver.conf
var logserverConf string

type LogServerReconciler struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
}

func (r *SFController) ensureLogserverPodMonitor() bool {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "sf",
			"run": logserverIdent,
		},
	}
	nePort := sfmonitoring.GetTruncatedPortName(logserverIdent, sfmonitoring.NodeExporterPortNameSuffix)
	desiredLsPodmonitor := sfmonitoring.MkPodMonitor(logserverIdent+"-monitor", r.ns, []string{nePort}, selector)
	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version": "1",
	}
	desiredLsPodmonitor.ObjectMeta.Annotations = annotations
	currentLspm := monitoringv1.PodMonitor{}
	if !r.GetM(desiredLsPodmonitor.Name, &currentLspm) {
		r.CreateR(&desiredLsPodmonitor)
		return false
	} else {
		if !utils.MapEquals(&currentLspm.ObjectMeta.Annotations, &annotations) {
			utils.LogI("Logserver PodMonitor configuration changed, updating...")
			currentLspm.Spec = desiredLsPodmonitor.Spec
			currentLspm.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentLspm)
			return false
		}
	}
	return true
}

func (r *SFController) ensureLogserverPromRule() bool {
	lsDiskRuleGroup := sfmonitoring.MkDiskUsageRuleGroup(r.ns, logserverIdent)
	// We keep the logserver's PromRule management here for standalone logservers
	desiredLsPromRule := sfmonitoring.MkPrometheusRuleCR(logserverIdent+"-default.rules", r.ns)
	desiredLsPromRule.Spec.Groups = append(desiredLsPromRule.Spec.Groups, lsDiskRuleGroup)

	var checksumable string
	for _, group := range desiredLsPromRule.Spec.Groups {
		for _, rule := range group.Rules {
			checksumable += sfmonitoring.MkAlertRuleChecksumString(rule)
		}
	}

	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version":       "2",
		"rulesChecksum": utils.Checksum([]byte(checksumable)),
	}

	desiredLsPromRule.ObjectMeta.Annotations = annotations
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredLsPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredLsPromRule)
		return false
	} else {
		if !utils.MapEquals(&currentPromRule.ObjectMeta.Annotations, &annotations) {
			utils.LogI("Logserver default Prometheus rules changed, updating...")
			currentPromRule.Spec = desiredLsPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

func (r *SFController) DeployLogserver() bool {

	r.EnsureSSHKeySecret(logserverIdent + "-keys")

	cmData := make(map[string]string)
	cmData["logserver.conf"] = logserverConf
	cmData["run.sh"] = logserverRun

	lgEntryScriptName := logserverIdent + "-entrypoint.sh"
	cmData[lgEntryScriptName] = logserverEntrypoint

	r.EnsureConfigMap(logserverIdent, cmData)

	// Create service exposed by logserver
	svc := base.MkServicePod(
		logserverIdent, r.ns, logserverIdent+"-0",
		[]int32{httpdPort, sshdPort, sfmonitoring.NodeExporterPort}, logserverIdent)
	r.EnsureService(&svc)

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/etc/httpd/conf.d/logserver.conf",
			ReadOnly:  true,
			SubPath:   "logserver.conf",
		},
		{
			Name:      logserverIdent,
			MountPath: "/var/www/html/logs",
		},
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/usr/bin/" + lgEntryScriptName,
			SubPath:   lgEntryScriptName,
		},
	}

	// Create the statefulset
	sts := r.mkStatefulSet(logserverIdent, base.HTTPDImage(),
		BaseGetStorageConfOrDefault(r.cr.Spec.Logserver.Storage, r.cr.Spec.StorageClassName), apiv1.ReadWriteOnce)

	// Setup the main container
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	var mod int32 = 256 // decimal for 0400 octal
	sts.Spec.Template.Spec.Volumes = []apiv1.Volume{
		{
			Name: logserverIdent + "-config-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: logserverIdent + "-config-map",
					},
					DefaultMode: &utils.Execmod,
				},
			},
		},
		{
			Name: logserverIdent + "-keys",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  logserverIdent + "-keys",
					DefaultMode: &mod,
				},
			},
		},
	}

	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(httpdPort, httpdPortName),
	}

	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/", httpdPort)
	sts.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/", httpdPort)
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/", httpdPort)
	sts.Spec.Template.Spec.Containers[0].Command = []string{
		"/usr/bin/" + lgEntryScriptName,
	}

	// Setup the sidecar container for sshd
	sshdContainer := base.MkContainer(sshdPortName, base.SSHDImage())
	sshdContainer.Command = []string{"bash", "/conf/run.sh"}
	sshdContainer.LivenessProbe = base.MkReadinessTCPProbe(sshdPort)
	sshdContainer.StartupProbe = base.MkReadinessTCPProbe(sshdPort)

	pubKey, err := r.GetSecretDataFromKey("zuul-ssh-key", "pub")
	if err != nil {
		return false
	}
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	sshdContainer.Env = []apiv1.EnvVar{
		base.MkEnvVar("AUTHORIZED_KEY", pubKeyB64),
	}
	sshdContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: "/home/data/rsync",
		},
		{
			Name:      logserverIdent + "-keys",
			MountPath: "/var/ssh-keys",
			ReadOnly:  true,
		},
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/conf",
		},
	}
	sshdContainer.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(sshdPort, sshdPortName),
	}

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, sshdContainer)

	retentionDays := r.cr.Spec.Logserver.RetentionDays
	if retentionDays == 0 {
		retentionDays = 60
	}

	loopDelay := r.cr.Spec.Logserver.LoopDelay
	if loopDelay == 0 {
		loopDelay = 3600
	}

	purgelogsContainer := base.MkContainer(purgelogIdent, base.PurgelogsImage())
	purgelogsContainer.Command = []string{
		"/usr/local/bin/purgelogs",
		"--retention-days",
		strconv.Itoa(retentionDays),
		"--loop", strconv.Itoa(loopDelay),
		"--log-path-dir",
		purgelogsLogsDir,
		"--debug"}
	purgelogsContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: purgelogsLogsDir,
		},
	}

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, purgelogsContainer)

	// add volumestats exporter
	volumeMountsStatsExporter := []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: "/home/data/rsync",
		},
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(logserverIdent, volumeMountsStatsExporter)
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, statsExporter)

	// Increase serial each time you need to enforce a deployment change/pod restart between operator versions
	sts.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"fqdn":       r.cr.Spec.FQDN,
		"serial":     "6",
		"httpd-conf": utils.Checksum([]byte(logserverConf)),
		"purgeLogConfig": "retentionDays:" + strconv.Itoa(r.cr.Spec.Logserver.RetentionDays) +
			" loopDelay:" + strconv.Itoa(r.cr.Spec.Logserver.LoopDelay),
		"httpd-image":     base.HTTPDImage(),
		"purgelogs-image": base.PurgelogsImage(),
		"sshd-image":      base.SSHDImage(),
	}

	current, stsUpdated := r.ensureStatefulset(sts)

	pvcReadiness := r.reconcileExpandPVC(logserverIdent+"-"+logserverIdent+"-0", r.cr.Spec.Logserver.Storage)

	// TODO(mhu) We may want to open an ingress to port 9100 for an external prometheus instance.
	// TODO(mhu) we may want to include monitoring objects' status in readiness computation
	r.ensureLogserverPodMonitor()
	r.ensureLogserverPromRule()

	isReady := r.IsStatefulSetReady(current) && !stsUpdated && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, logserverIdent, isReady)

	return isReady
}
