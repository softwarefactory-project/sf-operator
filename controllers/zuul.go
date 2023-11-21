// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	ini "gopkg.in/ini.v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

const (
	zuulWEBPort = 9000

	zuulExecutorPortName = "finger"
	zuulExecutorPort     = 7900

	zuulPrometheusPort     = 9090
	zuulPrometheusPortName = "zuul-metrics"
)

var (
	zuulStatsdExporterPortName = monitoring.GetStatsdExporterPort("zuul")

	//go:embed static/zuul/zuul.conf
	zuulDotconf string

	//go:embed static/zuul/statsd_mapping.yaml
	zuulStatsdMappingConfig string

	//go:embed static/zuul/generate-tenant-config.sh
	zuulGenerateTenantConfig string

	//go:embed static/zuul/logging.yaml.tmpl
	zuulLoggingConfig string

	// Common config sections for all Zuul components
	commonIniConfigSections = []string{"zookeeper", "keystore", "database"}

	zuulFluentBitLabels = []logging.FluentBitLabel{
		{
			Key:   "COMPONENT",
			Value: "zuul",
		},
	}
)

func isStatefulset(service string) bool {
	return service == "zuul-scheduler" || service == "zuul-executor" || service == "zuul-merger"
}

func mkZuulLoggingMount(service string) apiv1.VolumeMount {
	return apiv1.VolumeMount{
		Name:      "zuul-logging-config",
		MountPath: "/var/lib/zuul/" + service + "-logging.yaml",
		SubPath:   service + "-logging.yaml",
	}
}

func mkZuulGitHubSecretsMounts(r *SFController) []apiv1.VolumeMount {
	zuulConnectionMounts := []apiv1.VolumeMount{}
	secretkey := "app_key"
	for _, connection := range r.cr.Spec.Zuul.GitHubConns {
		secretName := connection.Secrets

		_, err := r.GetSecretDataFromKey(secretName, secretkey)
		if err == nil {
			zuulConnectionMounts = append(zuulConnectionMounts, apiv1.VolumeMount{
				Name:      secretName,
				MountPath: "/var/lib/zuul/" + secretName + "/" + secretkey,
				SubPath:   secretkey,
			})
		}
	}
	return zuulConnectionMounts
}

func (r *SFController) mkZuulContainer(service string) []apiv1.Container {
	volumes := []apiv1.VolumeMount{
		{
			Name:      "zuul-config",
			MountPath: "/etc/zuul",
			ReadOnly:  true,
		},
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      service,
			MountPath: "/var/lib/zuul",
		},
		{
			Name:      "zuul-ssh-key",
			MountPath: "/var/lib/zuul-ssh",
			ReadOnly:  true,
		},
		{
			Name:      "ca-cert",
			MountPath: "/etc/pki/ca-trust/source/anchors/ca.crt",
			ReadOnly:  true,
			SubPath:   "ca.crt",
		},
	}
	envs := []apiv1.EnvVar{
		base.MkEnvVar("REQUESTS_CA_BUNDLE", "/etc/ssl/certs/ca-bundle.crt"),
		base.MkEnvVar("HOME", "/var/lib/zuul"),
	}
	if service == "zuul-scheduler" {
		volumes = append(volumes,
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		)
		envs = append(envs, r.getTenantsEnvs()...)
	}

	volumes = append(volumes, mkZuulLoggingMount(service))
	volumes = append(volumes, mkZuulGitHubSecretsMounts(r)...)

	command := []string{
		"sh", "-c",
		fmt.Sprintf("exec %s -f -d", service),
	}
	container := apiv1.Container{
		Name:         service,
		Image:        base.ZuulImage(service),
		Command:      command,
		Env:          envs,
		VolumeMounts: volumes,
	}
	return []apiv1.Container{container}
}

func mkZuulVolumes(service string, r *SFController) []apiv1.Volume {
	var mod int32 = 256 // decimal for 0400 octal

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("ca-cert"),
		base.MkVolumeSecret("zuul-config"),
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeCM("zuul-logging-config", "zuul-logging-config-map"),
		{
			Name: "zuul-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "zuul-ssh-key",
					DefaultMode: &mod,
				},
			},
		},
		base.MkVolumeCM("statsd-config", "zuul-statsd-config-map"),
	}
	if !isStatefulset(service) {
		// statefulset already has a PV for the service-name,
		// for the other, we use an empty dir.
		volumes = append(volumes, base.MkEmptyDirVolume(service))
	}
	if service == "zuul-scheduler" {
		toolingVol := apiv1.Volume{
			Name: "tooling-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "zuul-scheduler-tooling-config-map",
					},
					DefaultMode: &utils.Execmod,
				},
			},
		}
		volumes = append(volumes, toolingVol)
	}

	volumes = append(volumes, mkZuulGitHubSecretsVolumes(r)...)

	return volumes
}

func (r *SFController) getTenantsEnvs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "TRUE"),
			base.MkEnvVar("CONFIG_REPO_BASE_URL", strings.TrimSuffix(r.cr.Spec.ConfigLocation.BaseURL, "/")),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
			base.MkEnvVar("CONFIG_REPO_CONNECTION_NAME", r.cr.Spec.ConfigLocation.ZuulConnectionName),
		}
	} else {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) mkInitSchedulerConfigContainer() apiv1.Container {
	container := base.MkContainer("init-scheduler-config", base.BusyboxImage)
	container.Command = []string{"/usr/local/bin/generate-zuul-tenant-yaml.sh"}
	container.Env = append(r.getTenantsEnvs(),
		base.MkEnvVar("HOME", "/var/lib/zuul"), base.MkEnvVar("INIT_CONTAINER", "1"))
	container.VolumeMounts = []apiv1.VolumeMount{
		{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
		{
			Name:      "tooling-vol",
			SubPath:   "generate-zuul-tenant-yaml.sh",
			MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
	}
	return container
}

func (r *SFController) setZuulLoggingfile() {
	loggingData := make(map[string]string)

	zuulExecutorLogLevel := sfv1.InfoLogLevel
	zuulSchedulerLogLevel := sfv1.InfoLogLevel
	zuulWebLogLevel := sfv1.InfoLogLevel
	zuulMergerLogLevel := sfv1.InfoLogLevel

	if r.cr.Spec.Zuul.Executor.LogLevel != "" {
		zuulExecutorLogLevel = r.cr.Spec.Zuul.Executor.LogLevel
	}
	if r.cr.Spec.Zuul.Scheduler.LogLevel != "" {
		zuulSchedulerLogLevel = r.cr.Spec.Zuul.Scheduler.LogLevel
	}
	if r.cr.Spec.Zuul.Web.LogLevel != "" {
		zuulWebLogLevel = r.cr.Spec.Zuul.Web.LogLevel
	}
	if r.cr.Spec.Zuul.Merger.LogLevel != "" {
		zuulMergerLogLevel = r.cr.Spec.Zuul.Web.LogLevel
	}
	var forwardLogs = false
	var inputBaseURL = ""
	if r.cr.Spec.FluentBitLogForwarding != nil {
		forwardLogs = true
		inputBaseURL = "http://" + r.cr.Spec.FluentBitLogForwarding.HTTPInputHost + ":" + strconv.Itoa(int(r.cr.Spec.FluentBitLogForwarding.HTTPInputPort))
	}

	loggingData["zuul-executor-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		logging.PythonTemplateLoggingParams{
			LogLevel:    string(zuulExecutorLogLevel),
			ForwardLogs: forwardLogs,
			BaseURL:     inputBaseURL,
		})

	loggingData["zuul-scheduler-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		logging.PythonTemplateLoggingParams{
			LogLevel:    string(zuulSchedulerLogLevel),
			ForwardLogs: forwardLogs,
			BaseURL:     inputBaseURL,
		})

	loggingData["zuul-web-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		logging.PythonTemplateLoggingParams{
			LogLevel:    string(zuulWebLogLevel),
			ForwardLogs: forwardLogs,
			BaseURL:     inputBaseURL,
		})

	loggingData["zuul-merger-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		logging.PythonTemplateLoggingParams{
			LogLevel:    string(zuulMergerLogLevel),
			ForwardLogs: forwardLogs,
			BaseURL:     inputBaseURL,
		})

	r.EnsureConfigMap("zuul-logging", loggingData)

}

func (r *SFController) getZuulLoggingString(service string) string {
	var loggingcm apiv1.ConfigMap
	if !r.GetM("zuul-logging-config-map", &loggingcm) {
		return ""
	}
	return loggingcm.Data[service+"-logging.yaml"]
}

func mkZuulGitHubSecretsVolumes(r *SFController) []apiv1.Volume {
	gitConnectionSecretVolumes := []apiv1.Volume{}
	for _, connection := range r.cr.Spec.Zuul.GitHubConns {
		secretName := connection.Secrets

		if _, err := r.GetSecretbyNameRef(secretName); err != nil {
			r.log.V(1).Error(err, "Error while getting secret "+secretName)
			continue
		}

		gitConnectionSecretVolumes = append(gitConnectionSecretVolumes, base.MkVolumeSecret(secretName))
	}
	return gitConnectionSecretVolumes
}

func (r *SFController) EnsureZuulScheduler(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	authSections := utils.IniGetSectionNamesByPrefix(cfg, "auth")
	sections = append(sections, authSections...)
	// TODO add statsd section in followup patch
	sections = append(sections, "scheduler")

	annotations := map[string]string{
		"zuul-common-config":    utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":            base.ZuulImage("zuul-scheduler"),
		"statsd_mapping":        utils.Checksum([]byte(zuulStatsdMappingConfig)),
		"serial":                "3",
		"zuul-logging":          utils.Checksum([]byte(r.getZuulLoggingString("zuul-scheduler"))),
		"zuul-connections":      utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigLocation.ZuulConnectionName + ":" +
			r.cr.Spec.ConfigLocation.BaseURL +
			r.cr.Spec.ConfigLocation.Name
	}

	var relayAddress *string
	if r.cr.Spec.Zuul.Scheduler.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Zuul.Scheduler.StatsdTarget
	}

	zuulContainers := r.mkZuulContainer("zuul-scheduler")

	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-scheduler", r.cr.Spec.FluentBitLogForwarding, zuulFluentBitLabels, annotations)
	zuulContainers[0].Env = append(zuulContainers[0].Env, extraLoggingEnvVars...)

	statsdSidecar := monitoring.MkStatsdExporterSideCarContainer("zuul", "statsd-config", relayAddress)

	zuulContainers = append(zuulContainers, statsdSidecar)

	var setAdditionalContainers = func(sts *appsv1.StatefulSet) {
		sts.Spec.Template.Spec.InitContainers = []apiv1.Container{r.mkInitSchedulerConfigContainer()}
		sts.Spec.Template.Spec.Containers = zuulContainers
	}

	schedulerToolingData := make(map[string]string)
	schedulerToolingData["generate-zuul-tenant-yaml.sh"] = zuulGenerateTenantConfig

	r.EnsureConfigMap("zuul-scheduler-tooling", schedulerToolingData)

	zsVolumes := mkZuulVolumes("zuul-scheduler", r)

	zs := r.mkStatefulSet("zuul-scheduler", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Scheduler.Storage), apiv1.ReadWriteOnce)
	zs.Spec.Template.ObjectMeta.Annotations = annotations
	setAdditionalContainers(&zs)
	zs.Spec.Template.Spec.Volumes = zsVolumes
	zs.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/health/live", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/health/ready", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, zuulPrometheusPortName),
	}

	current := appsv1.StatefulSet{}
	if r.GetM("zuul-scheduler", &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-scheduler configuration changed, rollout zuul-scheduler pods ...")
			current.Spec.Template = *zs.Spec.Template.DeepCopy()
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zs
		r.CreateR(&current)
	}

	isStatefulSet := r.IsStatefulSetReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-scheduler", isStatefulSet)

	return isStatefulSet
}

func (r *SFController) EnsureZuulExecutor(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "executor")
	annotations := map[string]string{
		"zuul-common-config":    utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":            base.ZuulImage("zuul-executor"),
		"serial":                "1",
		"zuul-logging":          utils.Checksum([]byte(r.getZuulLoggingString("zuul-executor"))),
		"zuul-connections":      utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
	}

	ze := r.mkHeadlessSatefulSet("zuul-executor", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Executor.Storage), apiv1.ReadWriteOnce)
	ze.Spec.Template.Spec.Containers = r.mkZuulContainer("zuul-executor")
	ze.Spec.Template.Spec.Volumes = mkZuulVolumes("zuul-executor", r)

	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-executor", r.cr.Spec.FluentBitLogForwarding, zuulFluentBitLabels, annotations)
	ze.Spec.Template.Spec.Containers[0].Env = append(ze.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	ze.Spec.Template.ObjectMeta.Annotations = annotations

	ze.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	ze.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessHTTPProbe("/health/live", zuulPrometheusPort)
	ze.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, zuulPrometheusPortName),
		base.MkContainerPort(zuulExecutorPort, zuulExecutorPortName),
	}
	// NOTE(dpawlik): Zuul Executor needs to privileged pod, due error in the console log:
	// "bwrap: Can't bind mount /oldroot/etc/resolv.conf on /newroot/etc/resolv.conf: Permission denied""
	ze.Spec.Template.Spec.Containers[0].SecurityContext = base.MkSecurityContext(true)
	ze.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = pointer.Int64(1000)

	current := appsv1.StatefulSet{}
	if r.GetM("zuul-executor", &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-executor configuration changed, rollout zuul-executor pods ...")
			current.Spec.Template = *ze.Spec.Template.DeepCopy()
			r.UpdateR(&current)
			return false
		}
	} else {
		current := ze
		r.CreateR(&current)
	}

	isStatefulSet := r.IsStatefulSetReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-executor", isStatefulSet)

	return isStatefulSet
}

func (r *SFController) EnsureZuulMerger(cfg *ini.File) bool {

	service := "zuul-merger"

	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "merger")

	annotations := map[string]string{
		"zuul-common-config":    utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":            base.ZuulImage(service),
		"zuul-connections":      utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
	}

	zm := r.mkHeadlessSatefulSet(service, "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Merger.Storage), apiv1.ReadWriteOnce)
	zm.Spec.Template.Spec.Containers = r.mkZuulContainer(service)
	zm.Spec.Template.Spec.Volumes = mkZuulVolumes(service, r)

	extraLoggingEnvVars := logging.SetupLogForwarding(service, r.cr.Spec.FluentBitLogForwarding, zuulFluentBitLabels, annotations)
	zm.Spec.Template.Spec.Containers[0].Env = append(zm.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	zm.Spec.Template.ObjectMeta.Annotations = annotations

	zm.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessHTTPProbe("/health/live", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, zuulPrometheusPortName),
	}

	current := appsv1.StatefulSet{}
	if r.GetM(service, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-merger configuration changed, rollout zuul-merger pods ...")
			current.Spec.Template = *zm.Spec.Template.DeepCopy()
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zm
		r.CreateR(&current)
	}

	isStatefulSet := r.IsStatefulSetReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, service, isStatefulSet)

	return isStatefulSet
}

func (r *SFController) EnsureZuulWeb(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	authSections := utils.IniGetSectionNamesByPrefix(cfg, "auth")
	sections = append(sections, authSections...)
	sections = append(sections, "web")
	annotations := map[string]string{
		"zuul-common-config":    utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":            base.ZuulImage("zuul-web"),
		"serial":                "1",
		"zuul-logging":          utils.Checksum([]byte(r.getZuulLoggingString("zuul-web"))),
		"zuul-connections":      utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
	}

	zw := base.MkDeployment("zuul-web", r.ns, "")
	zw.Spec.Template.Spec.Containers = r.mkZuulContainer("zuul-web")
	zw.Spec.Template.Spec.Volumes = mkZuulVolumes("zuul-web", r)

	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-web", r.cr.Spec.FluentBitLogForwarding, zuulFluentBitLabels, annotations)
	zw.Spec.Template.Spec.Containers[0].Env = append(zw.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	zw.Spec.Template.ObjectMeta.Annotations = annotations

	zw.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, zuulPrometheusPortName),
	}

	current := appsv1.Deployment{}
	if r.GetM("zuul-web", &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-web configuration changed, rollout zuul-web pods ...")
			current.Spec = zw.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zw
		r.CreateR(&current)
	}

	isDeploymentReady := r.IsDeploymentReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-web", isDeploymentReady)

	return isDeploymentReady
}

func (r *SFController) EnsureZuulComponentsFrontServices() {
	servicePorts := []int32{zuulWEBPort}
	srv := base.MkService("zuul-web", r.ns, "zuul-web", servicePorts, "zuul-web")
	r.GetOrCreate(&srv)

	headlessPorts := []int32{zuulExecutorPort}
	srvZE := base.MkHeadlessService("zuul-executor", r.ns, "zuul-executor", headlessPorts, "zuul-executor")
	r.GetOrCreate(&srvZE)

}

func (r *SFController) EnsureZuulComponents(cfg *ini.File) bool {

	zuulServices := map[string]bool{}
	r.setZuulLoggingfile()
	zuulServices["scheduler"] = r.EnsureZuulScheduler(cfg)
	zuulServices["executor"] = r.EnsureZuulExecutor(cfg)
	zuulServices["web"] = r.EnsureZuulWeb(cfg)
	zuulServices["merger"] = r.EnsureZuulMerger(cfg)

	return zuulServices["scheduler"] && zuulServices["executor"] && zuulServices["web"] && zuulServices["merger"]
}

func (r *SFController) EnsureZuulPodMonitor() bool {
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "run",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"zuul-scheduler", "zuul-merger", "zuul-executor", "zuul-web"},
			},
			{
				Key:      "app",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"sf"},
			},
		},
	}
	desiredZuulPodMonitor := monitoring.MkPodMonitor("zuul-monitor", r.ns, []string{zuulPrometheusPortName, zuulStatsdExporterPortName}, selector)
	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version": "2",
	}
	desiredZuulPodMonitor.ObjectMeta.Annotations = annotations
	currentZPM := monitoringv1.PodMonitor{}
	if !r.GetM(desiredZuulPodMonitor.Name, &currentZPM) {
		r.CreateR(&desiredZuulPodMonitor)
		return false
	} else {
		if !utils.MapEquals(&currentZPM.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Zuul PodMonitor configuration changed, updating...")
			currentZPM.Spec = desiredZuulPodMonitor.Spec
			currentZPM.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentZPM)
			return false
		}
	}
	return true
}

// create default alerts
func (r *SFController) ensureZuulPromRule() bool {
	/* Alert when a config-update job fails on the config repository */
	configUpdateFailureInPostAnnotations := map[string]string{
		"description": "A config-update job failed in the post pipeline. Latest changes might not have been applied. Please check services configurations",
		"summary":     "config-update failure post merge",
	}
	configUpdateFailureInPost := monitoring.MkPrometheusAlertRule(
		"ConfigUpdateFailureInPostPipeline",
		intstr.FromString(
			"increase(zuul_tenant_pipeline_project_job_count"+
				"{jobname=\"config-update\",tenant=\"internal\",pipeline=\"post\",result!~\"SUCCESS|wait_time\"}[1m]) > 0"),
		"0m",
		monitoring.CriticalSeverityLabel,
		configUpdateFailureInPostAnnotations,
	)
	configRepoRuleGroup := monitoring.MkPrometheusRuleGroup(
		"config-repository_default.rules",
		[]monitoringv1.Rule{configUpdateFailureInPost})

	/* Alert when executors are saturated */
	notEnoughExecutorsAnnotations := map[string]string{
		"description": "Some jobs have been waiting for an executor to run on in the last hour",
		"summary":     "Not enough executors",
	}
	notEnoughExecutors := monitoring.MkPrometheusAlertRule(
		"NotEnoughExecutors",
		intstr.FromString(
			"rate(zuul_executors_jobs_queued[1h]) > 0"),
		"1h",
		monitoring.WarningSeverityLabel,
		notEnoughExecutorsAnnotations,
	)

	/* Alert when mergers are saturated */
	notEnoughMergersAnnotations := map[string]string{
		"description": "Some merge jobs have been waiting for a merger to run on in the last hour",
		"summary":     "Not enough mergers",
	}
	notEnoughMergers := monitoring.MkPrometheusAlertRule(
		"NotEnoughMergers",
		intstr.FromString(
			"rate(zuul_mergers_jobs_queued[1h]) > 0"),
		"1h",
		monitoring.WarningSeverityLabel,
		notEnoughMergersAnnotations,
	)

	/* Alert when node requests are saturated */
	notEnoughNodesAnnotations := map[string]string{
		"description": "Nodepool had outstanding node requests in the last hour",
		"summary":     "Not enough testing nodes",
	}
	notEnoughNodes := monitoring.MkPrometheusAlertRule(
		"NotEnoughTestNodes",
		intstr.FromString(
			"rate(zuul_nodepool_current_requests[1h]) > 0"),
		"1h",
		monitoring.WarningSeverityLabel,
		notEnoughNodesAnnotations,
	)

	zuulRuleGroup := monitoring.MkPrometheusRuleGroup(
		"zuul_default.rules",
		[]monitoringv1.Rule{
			notEnoughExecutors,
			notEnoughMergers,
			notEnoughNodes,
		})

	desiredZuulPromRule := monitoring.MkPrometheusRuleCR("zuul-default.rules", r.ns)
	desiredZuulPromRule.Spec.Groups = append(
		desiredZuulPromRule.Spec.Groups,
		configRepoRuleGroup,
		zuulRuleGroup)

	var checksumable string
	for _, group := range desiredZuulPromRule.Spec.Groups {
		for _, rule := range group.Rules {
			checksumable += monitoring.MkAlertRuleChecksumString(rule)
		}
	}

	annotations := map[string]string{
		"version":       "2",
		"rulesChecksum": utils.Checksum([]byte(checksumable)),
	}
	desiredZuulPromRule.ObjectMeta.Annotations = annotations
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredZuulPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredZuulPromRule)
		return false
	} else {
		if !utils.MapEquals(&currentPromRule.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Zuul default Prometheus rules changed, updating...")
			currentPromRule.Spec = desiredZuulPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

func (r *SFController) EnsureZuulConfigSecret(cfg *ini.File) {
	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"zuul.conf": []byte(DumpConfigINI(cfg)),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.ns},
	})
}

func (r *SFController) readSecretContent(name string) string {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		// Loosely return empty string when not found
		return ""
	}
	return string(secret.Data[name])
}

func (r *SFController) AddOIDCAuthenticator(cfg *ini.File, authenticator sfv1.ZuulOIDCAuthenticatorSpec) {
	section := "auth " + authenticator.Name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "OpenIDConnect")
	cfg.Section(section).NewKey("realm", authenticator.Realm)
	cfg.Section(section).NewKey("client_id", authenticator.ClientID)
	cfg.Section(section).NewKey("issuer_id", authenticator.IssuerID)
	if authenticator.UIDClaim != "sub" {
		cfg.Section(section).NewKey("uid_claim", authenticator.UIDClaim)
	}
	if authenticator.MaxValidityTime != 0 {
		cfg.Section(section).NewKey("max_validity_time", strconv.Itoa(int(authenticator.MaxValidityTime)))
	}
	if authenticator.Skew != 0 {
		cfg.Section(section).NewKey("skew", strconv.Itoa(int(authenticator.Skew)))
	}
	if authenticator.Scope != "openid profile" {
		cfg.Section(section).NewKey("scope", authenticator.Scope)
	}
	if authenticator.Authority != "" {
		cfg.Section(section).NewKey("authority", authenticator.Authority)
	}
	if authenticator.Audience != "" {
		cfg.Section(section).NewKey("audience", authenticator.Audience)
	}
	if !authenticator.LoadUserInfo {
		cfg.Section(section).NewKey("load_user_info", strconv.FormatBool(authenticator.LoadUserInfo))
	}
	if authenticator.KeysURL != "" {
		cfg.Section(section).NewKey("keys_url", authenticator.KeysURL)
	}

}

func (r *SFController) AddGerritConnection(cfg *ini.File, conn sfv1.GerritConnection) {
	section := "connection " + conn.Name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "gerrit")
	cfg.Section(section).NewKey("server", conn.Hostname)
	cfg.Section(section).NewKey("sshkey", "/var/lib/zuul-ssh/..data/priv")
	cfg.Section(section).NewKey("gitweb_url_template", "{baseurl}/plugins/gitiles/{project.name}/+/{sha}^!/")
	// Optional fields (set as omitempty in GerritConnection struct definition)
	if conn.Username != "" {
		cfg.Section(section).NewKey("user", conn.Username)
	}
	if conn.Port > 0 {
		cfg.Section(section).NewKey("port", strconv.Itoa(int(conn.Port)))
	}
	if conn.Puburl != "" {
		cfg.Section(section).NewKey("baseurl", conn.Puburl)
	}
	if conn.Password != "" {
		password := r.readSecretContent(conn.Password)
		cfg.Section(section).NewKey("password", password)
	}
	if conn.Canonicalhostname != "" {
		cfg.Section(section).NewKey("canonical_hostname", conn.Canonicalhostname)
	}
	if !conn.VerifySSL {
		// Zuul default is true, so set that setting only when VerifySSL is disabled
		cfg.Section(section).NewKey("verify_ssl", "false")
	}
	if conn.GitOverSSH {
		cfg.Section(section).NewKey("git_over_ssh", "true")
	}
}

// addKeyToSection add a tuple to the Section if the fieldValue is not empty
func addKeyToSection(section *ini.Section, fieldKey string, fieldValue string) {
	if fieldValue != "" {
		section.NewKey(fieldKey, fieldValue)
	}
}

func (r *SFController) AddGitHubConnection(cfg *ini.File, conn sfv1.GitHubConnection) {

	appID := fmt.Sprintf("%d", conn.AppID)
	appKey := "/var/lib/zuul/" + conn.Secrets + "/app_key"

	_, err := r.GetSecretDataFromKey(conn.Secrets, "app_key")
	if err != nil {
		r.log.V(1).Info(err.Error(), "app_key", conn.Secrets)
		appKey = ""
	}

	if appKey == "" || appID == "0" {
		r.log.V(1).Info("app_key or app_id is not defined", "app_key", appKey, "app_id", appID)
		appKey = ""
		appID = ""
	}

	apiToken, err := r.GetSecretDataFromKey(conn.Secrets, "api_token")
	if err != nil {
		r.log.V(1).Info(err.Error(), "api_token", conn.Secrets)
	}

	webhookToken, err := r.GetSecretDataFromKey(conn.Secrets, "webhook_token")
	if err != nil {
		r.log.V(1).Info(err.Error(), "webhook_token", conn.Secrets)
	}

	section := "connection " + conn.Name
	cfg.NewSection(section)

	for key, value := range map[string]string{
		"driver":             "github",
		"app_id":             appID,
		"app_key":            appKey,
		"api_token":          string(apiToken),
		"webhook_token":      string(webhookToken),
		"sshkey":             "/var/lib/zuul-ssh/..data/priv",
		"server":             conn.Server,
		"canonical_hostname": conn.Canonicalhostname,
		"verify_ssl":         fmt.Sprint(conn.VerifySSL),
	} {
		addKeyToSection(cfg.Section(section), key, value)
	}

}

func (r *SFController) AddGitLabConnection(cfg *ini.File, conn sfv1.GitLabConnection) {

	apiToken, _ := r.GetSecretDataFromKey(conn.Secrets, "api_token")
	webHookToken, _ := r.GetSecretDataFromKey(conn.Secrets, "webhook_token")

	section := "connection " + conn.Name
	cfg.NewSection(section)

	for key, value := range map[string]string{
		"driver":                  "gitlab",
		"api_token":               string(apiToken),
		"api_token_name":          conn.APITokenName,
		"webhook_token":           string(webHookToken),
		"server":                  conn.Server,
		"canonical_hostname":      conn.CanonicalHostname,
		"baseurl":                 conn.BaseURL,
		"sshkey":                  "/var/lib/zuul-ssh/..data/priv",
		"cloneurl":                conn.CloneURL,
		"keepalive":               fmt.Sprint(conn.KeepAlive),
		"disable_connection_pool": fmt.Sprint(conn.DisableConnectionPool),
	} {
		addKeyToSection(cfg.Section(section), key, value)
	}

}

func AddGitConnection(cfg *ini.File, name string, baseurl string, poolDelay int32) {
	section := "connection " + name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "git")
	cfg.Section(section).NewKey("baseurl", baseurl)
	// When poolDelay is set to a positive value, then we add the setting or Zuul default will apply
	if poolDelay > 0 {
		cfg.Section(section).NewKey("poll_delay", strconv.Itoa(int(poolDelay)))
	}
}

func AddWebClientSection(cfg *ini.File) {
	section := "webclient"
	cfg.NewSection(section)
	cfg.Section(section).NewKey("url", "http://zuul-web:"+strconv.FormatInt(zuulWEBPort, 10))
}

func (r *SFController) AddDefaultConnections(cfg *ini.File) {
	// Internal git-server for system config
	AddGitConnection(cfg, "git-server", "git://git-server/", 0)

	// Git connection to opendev.org
	AddGitConnection(cfg, "opendev.org", "https://opendev.org/", 0)

	// Add Web Client for zuul-client
	AddWebClientSection(cfg)
}

func LoadConfigINI(zuulConf string) *ini.File {
	cfg, err := ini.Load([]byte(zuulConf))
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

func DumpConfigINI(cfg *ini.File) string {
	writer := bytes.NewBufferString("")
	cfg.WriteTo(writer)
	return writer.String()
}

func (r *SFController) DeployZuulSecrets() {
	r.EnsureSSHKeySecret("zuul-ssh-key")
	r.EnsureSecretUUID("zuul-keystore-password")
	r.EnsureSecretUUID("zuul-auth-secret")
}

func (r *SFController) DeployZuul() bool {
	dbSettings := apiv1.Secret{}
	if !r.GetM(zuulDBConfigSecret, &dbSettings) {
		r.log.Info("Waiting for db connection secret")
		return false
	}

	// create statsd exporter config map
	r.EnsureConfigMap("zuul-statsd", map[string]string{
		monitoring.StatsdExporterConfigFile: zuulStatsdMappingConfig,
	})

	// Update base config to add connections
	cfgINI := LoadConfigINI(zuulDotconf)
	for _, conn := range r.cr.Spec.Zuul.GerritConns {
		r.AddGerritConnection(cfgINI, conn)
	}

	for _, conn := range r.cr.Spec.Zuul.GitHubConns {
		r.AddGitHubConnection(cfgINI, conn)
	}

	for _, conn := range r.cr.Spec.Zuul.GitLabConns {
		r.AddGitLabConnection(cfgINI, conn)
	}

	for _, conn := range r.cr.Spec.Zuul.GitConns {
		AddGitConnection(cfgINI, conn.Name, conn.Baseurl, conn.PollDelay)
	}

	// Add default connections
	r.AddDefaultConnections(cfgINI)

	// Add OIDC authenticators
	for _, authenticator := range r.cr.Spec.Zuul.OIDCAuthenticators {
		r.AddOIDCAuthenticator(cfgINI, authenticator)
	}
	var defaultAuthSection *string
	if len(r.cr.Spec.Zuul.OIDCAuthenticators) == 1 {
		defaultAuthSection = &r.cr.Spec.Zuul.OIDCAuthenticators[0].Name
	} else if r.cr.Spec.Zuul.DefaultAuthenticator != "" {
		defaultAuthSection = &r.cr.Spec.Zuul.DefaultAuthenticator
	}
	if defaultAuthSection != nil {
		cfgINI.Section("auth "+*defaultAuthSection).NewKey("default", "true")
	}

	// Enable prometheus metrics
	for _, srv := range []string{"web", "executor", "scheduler", "merger"} {
		cfgINI.Section(srv).NewKey("prometheus_port", strconv.Itoa(zuulPrometheusPort))
	}
	// Set Zuul web public URL
	cfgINI.Section("web").NewKey("root", "https://zuul."+r.cr.Spec.FQDN)

	// Set Zuul Merger Configurations
	if r.cr.Spec.Zuul.Merger.GitUserName != "" {
		cfgINI.Section("merger").NewKey("git_user_name", r.cr.Spec.Zuul.Merger.GitUserName)
	}
	if r.cr.Spec.Zuul.Merger.GitUserEmail != "" {
		cfgINI.Section("merger").NewKey("git_user_email", r.cr.Spec.Zuul.Merger.GitUserEmail)
	}
	if r.cr.Spec.Zuul.Merger.GitHTTPLowSpeedLimit >= 0 {
		cfgINI.Section("merger").NewKey("git_http_low_speed_limit", fmt.Sprint(r.cr.Spec.Zuul.Merger.GitHTTPLowSpeedLimit))
	}
	if r.cr.Spec.Zuul.Merger.GitHTTPLowSpeedTime >= 0 {
		cfgINI.Section("merger").NewKey("git_http_low_speed_time", fmt.Sprint(r.cr.Spec.Zuul.Merger.GitHTTPLowSpeedTime))
	}
	if r.cr.Spec.Zuul.Merger.GitTimeout > 0 {
		cfgINI.Section("merger").NewKey("git_timeout", fmt.Sprint(r.cr.Spec.Zuul.Merger.GitTimeout))
	}

	// Set Database DB URI
	cfgINI.Section("database").NewKey("dburi", fmt.Sprintf(
		"mysql+pymysql://%s:%s@%s/%s", dbSettings.Data["username"], dbSettings.Data["password"], dbSettings.Data["host"], dbSettings.Data["database"]))

	// Set Zookeeper hosts
	cfgINI.Section("zookeeper").NewKey("hosts", "zookeeper."+r.ns+":2281")

	// Set Keystore secret
	keystorePass, err := r.getSecretData("zuul-keystore-password")
	if err != nil {
		r.log.Info("Waiting for zuul-keystore-password secret")
		return false
	}
	cfgINI.Section("keystore").NewKey("password", string(keystorePass))

	// Set CLI auth
	cliAuthSecret, err := r.getSecretData("zuul-auth-secret")
	if err != nil {
		r.log.Info("Waiting for zuul-auth-secret secret")
		return false
	}
	cfgINI.Section("auth zuul_client").NewKey("secret", string(cliAuthSecret))
	cfgINI.Section("auth zuul_client").NewKey("realm", "zuul."+r.cr.Spec.FQDN)
	// Configure statsd common config
	cfgINI.Section("statsd").NewKey("port", strconv.Itoa(int(monitoring.StatsdExporterPortListen)))

	r.EnsureZuulConfigSecret(cfgINI)
	r.EnsureZuulComponentsFrontServices()
	// We could condition readiness to the state of the PodMonitor, but we don't.
	r.EnsureZuulPodMonitor()
	r.ensureZuulPromRule()

	return r.EnsureZuulComponents(cfgINI) && r.setupZuulIngress()
}

func (r *SFController) runZuulInternalTenantReconfigure() bool {
	err := r.PodExec(
		"zuul-scheduler-0",
		"zuul-scheduler",
		[]string{"zuul-scheduler", "tenant-reconfigure", "internal"})
	return err == nil
}

func (r *SFController) setupZuulIngress() bool {
	route1Ready := r.ensureHTTPSRoute(r.cr.Name+"-zuul", "zuul", "zuul-web", "/", zuulWEBPort,
		map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	// Zuul ingress is special because the zuul-web container expect the
	// the files to be served at `/zuul/`, but it is listening on `/`.
	// Thus this ingress remove the `/zuul/` so that the javascript loads as
	// expected
	route2Ready := r.ensureHTTPSRoute(r.cr.Name+"-zuul-red", "zuul", "zuul-web", "/zuul", zuulWEBPort, map[string]string{
		"haproxy.router.openshift.io/rewrite-target": "/",
	}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)
	return route1Ready && route2Ready
}
