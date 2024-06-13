// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	ini "gopkg.in/ini.v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

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

	zuulPrometheusPort       = 9090
	ZuulPrometheusPortName   = "zuul-metrics"
	ZuulSchedulerIdent       = "zuul-scheduler"
	ZuulKeystorePasswordName = "zuul-keystore-password"
)

var (
	ZuulStatsdExporterPortName = monitoring.GetStatsdExporterPort("zuul")

	//go:embed static/zuul/zuul.conf
	zuulDotconf string

	//go:embed static/zuul/statsd_mapping.yaml
	zuulStatsdMappingConfig string

	//go:embed static/zuul/scheduler-init-container.sh
	zuulSchedulerInitContainerScript string

	//go:embed static/zuul/generate-tenant-config.sh
	zuulGenerateTenantConfig string

	//go:embed static/zuul/logging.yaml.tmpl
	zuulLoggingConfig string

	// Common config sections for all Zuul components
	commonIniConfigSections = []string{"zookeeper", "keystore", "database"}

	//go:embed static/zuul/ssh_config
	sshConfig string

	zuulFluentBitLabels = []logging.FluentBitLabel{
		{
			Key:   "COMPONENT",
			Value: "zuul",
		},
	}
)

func enableZuulLocalSource(template *apiv1.PodTemplateSpec, zuulSourceHostPath string, withInitContainer bool) {
	template.Spec.Volumes = append(template.Spec.Volumes, apiv1.Volume{
		Name: "host-mount",
		VolumeSource: apiv1.VolumeSource{
			HostPath: &apiv1.HostPathVolumeSource{
				Path: zuulSourceHostPath,
			},
		},
	})
	template.Spec.Containers[0].SecurityContext = base.MkSecurityContext(true)
	template.Spec.SecurityContext.RunAsNonRoot = ptr.To(false)
	template.Spec.Containers[0].VolumeMounts = append(template.Spec.Containers[0].VolumeMounts,
		apiv1.VolumeMount{
			Name:      "host-mount",
			MountPath: "/usr/local/lib/python3.11/site-packages/zuul",
		})
	if withInitContainer {
		template.Spec.InitContainers[0].SecurityContext = base.MkSecurityContext(true)
	}
	template.ObjectMeta.Annotations["zuul-local-source"] = "true"
}

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

func mkZuulConnectionsSecretsMount(r *SFController) []apiv1.VolumeMount {
	zuulConnectionMounts := []apiv1.VolumeMount{}
	for _, connection := range r.cr.Spec.Zuul.GitHubConns {
		secretName := connection.Secrets
		if connection.AppID > 0 {
			zuulConnectionMounts = append(zuulConnectionMounts, apiv1.VolumeMount{
				Name:      secretName,
				MountPath: "/var/lib/zuul/" + secretName + "/app_key",
				SubPath:   "app_key",
			})
		}
	}

	for _, conn := range r.cr.Spec.Zuul.GerritConns {
		if conn.Sshkey != "" {
			keyMount := apiv1.VolumeMount{
				Name:      "zuul-ssh-key-" + conn.Sshkey,
				MountPath: "/var/lib/zuul-" + conn.Sshkey + "/",
			}
			zuulConnectionMounts = append(zuulConnectionMounts, keyMount)
		}
	}

	return zuulConnectionMounts
}

func getZuulImage(service string) string {
	switch srv := service; srv {
	case "zuul-scheduler":
		return base.ZuulSchedulerImage()
	case "zuul-executor":
		return base.ZuulExecutorImage()
	case "zuul-merger":
		return base.ZuulMergerImage()
	case "zuul-web":
		return base.ZuulWebImage()
	default:
		panic("unsupported zuul service")
	}
}

func (r *SFController) mkZuulContainer(service string, corporateCMExists bool) apiv1.Container {
	volumeMounts := []apiv1.VolumeMount{
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
			Name:      "extra-config",
			SubPath:   "ssh_config",
			MountPath: "/etc/ssh/ssh_config.d/99-sf-operator.conf",
			ReadOnly:  true,
		},
		{
			Name:      "ca-cert",
			MountPath: "/etc/pki/ca-trust/source/anchors/ca.crt",
			ReadOnly:  true,
			SubPath:   "ca.crt",
		},
		{
			Name:      "zuul-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
		},
	}
	envs := []apiv1.EnvVar{
		base.MkEnvVar("REQUESTS_CA_BUNDLE", "/etc/ssl/certs/ca-bundle.crt"),
		base.MkEnvVar("HOME", "/var/lib/zuul"),
	}
	if service == "zuul-scheduler" {
		volumeMounts = append(volumeMounts,
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh",
				ReadOnly:  true,
			},
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "fetch-config-repo.sh",
				MountPath: "/usr/local/bin/fetch-config-repo.sh",
				ReadOnly:  true,
			},
		)
		envs = append(envs, r.getTenantsEnvs()...)
	}

	volumeMounts = append(volumeMounts, mkZuulLoggingMount(service))
	volumeMounts = append(volumeMounts, mkZuulConnectionsSecretsMount(r)...)

	if corporateCMExists {
		volumeMounts = AppendCorporateCACertsVolumeMount(volumeMounts, service+"-corporate-ca-certs")
	}

	container := apiv1.Container{
		Name:  service,
		Image: getZuulImage(service),
		Command: []string{"/usr/local/bin/dumb-init", "--", "bash", "-c",
			// Trigger the update of the CA Trust chain
			UpdateCATrustCommand + " && " +
				// https://git-scm.com/docs/git-config#Documentation/git-config.txt-safedirectory
				// This is needed when we mount the local zuul source from the host
				// to bypass the git ownership verification
				"git config --global --add safe.directory '*'" + " && " +
				// Start the service
				"/usr/local/bin/" + service + " -f -d"},
		Env:          envs,
		VolumeMounts: volumeMounts,
	}

	base.SetContainerLimitsHighProfile(&container)

	if service == "zuul-scheduler" {
		// For the scheduler we do not run the update-ca-trust because the initContainer
		// already handles that task.
		container.Command = []string{"/usr/local/bin/dumb-init", "--", "bash", "-c",
			"/usr/local/bin/zuul-scheduler -f -d"}
	}

	return container
}

func mkZuulVolumes(service string, r *SFController, corporateCMExists bool) []apiv1.Volume {
	var mod int32 = 256 // decimal for 0400 octal

	// create extra config config map
	r.EnsureConfigMap("zuul-extra", map[string]string{
		"ssh_config": sshConfig,
	})

	// create statsd exporter config map
	r.EnsureConfigMap("zuul-statsd", map[string]string{
		monitoring.StatsdExporterConfigFile: zuulStatsdMappingConfig,
	})

	// Install the logging settings config map resource
	r.ensureZuulLoggingConfigMap()

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
		base.MkVolumeCM("extra-config", "zuul-extra-config-map"),
		base.MkEmptyDirVolume("zuul-ca"),
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

	for _, conn := range r.cr.Spec.Zuul.GerritConns {
		if conn.Sshkey != "" {
			keyVol := apiv1.Volume{
				Name: "zuul-ssh-key-" + conn.Sshkey,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName:  conn.Sshkey,
						DefaultMode: &mod,
					},
				},
			}
			volumes = append(volumes, keyVol)
		}
	}

	volumes = append(volumes, mkZuulGitHubSecretsVolumes(r)...)

	if corporateCMExists {
		volumes = append(volumes, base.MkVolumeCM(service+"-corporate-ca-certs", "corporate-ca-certs"))
	}

	return volumes
}

func (r *SFController) getTenantsEnvs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "TRUE"),
			base.MkEnvVar("CONFIG_REPO_BASE_URL", strings.TrimSuffix(r.cr.Spec.ConfigRepositoryLocation.BaseURL, "/")),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
			base.MkEnvVar("CONFIG_REPO_CONNECTION_NAME", r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName),
		}
	} else {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) ensureZuulLoggingConfigMap() {
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
		zuulMergerLogLevel = r.cr.Spec.Zuul.Merger.LogLevel
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

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	annotations := map[string]string{
		"zuul-common-config":         utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config":      utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":                 getZuulImage("zuul-scheduler"),
		"statsd_mapping":             utils.Checksum([]byte(zuulStatsdMappingConfig)),
		"serial":                     "9",
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-scheduler"))),
		"zuul-extra":                 utils.Checksum([]byte(sshConfig)),
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName + ":" +
			r.cr.Spec.ConfigRepositoryLocation.BaseURL +
			r.cr.Spec.ConfigRepositoryLocation.Name
	}

	var relayAddress *string
	if r.cr.Spec.Zuul.Scheduler.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Zuul.Scheduler.StatsdTarget
	}

	zuulContainer := r.mkZuulContainer(ZuulSchedulerIdent, corporateCMExists)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Scheduler.Limits, &zuulContainer)

	zsFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-scheduler"})
	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-scheduler", r.cr.Spec.FluentBitLogForwarding, zsFluentBitLabels, annotations)
	zuulContainer.Env = append(zuulContainer.Env, extraLoggingEnvVars...)

	statsdSidecar := monitoring.MkStatsdExporterSideCarContainer("zuul", "statsd-config", relayAddress)
	nodeExporterSidecar := monitoring.MkNodeExporterSideCarContainer(
		"zuul-scheduler",
		[]apiv1.VolumeMount{
			{
				Name:      "zuul-scheduler",
				MountPath: "/var/lib/zuul",
			},
		})

	zuulContainers := append([]apiv1.Container{}, zuulContainer, statsdSidecar, nodeExporterSidecar)

	initContainer := base.MkContainer("init-scheduler-config", getZuulImage("zuul-scheduler"))
	initContainer.Command = []string{"/usr/local/bin/init-container.sh"}
	initContainer.Env = append(r.getTenantsEnvs(),
		base.MkEnvVar("HOME", "/var/lib/zuul"), base.MkEnvVar("INIT_CONTAINER", "1"))
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
		{
			Name:      "tooling-vol",
			SubPath:   "init-container.sh",
			MountPath: "/usr/local/bin/init-container.sh",
			ReadOnly:  true,
		},
		{
			Name:      "tooling-vol",
			SubPath:   "fetch-config-repo.sh",
			MountPath: "/usr/local/bin/fetch-config-repo.sh",
			ReadOnly:  true,
		},
		{
			Name:      "tooling-vol",
			SubPath:   "generate-zuul-tenant-yaml.sh",
			MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh",
			ReadOnly:  true,
		},
		{
			Name:      "zuul-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
		},
	}
	if corporateCMExists {
		initContainer.VolumeMounts = AppendCorporateCACertsVolumeMount(initContainer.VolumeMounts, "zuul-scheduler-corporate-ca-certs")
	}
	base.SetContainerLimitsLowProfile(&initContainer)

	schedulerToolingData := make(map[string]string)
	schedulerToolingData["init-container.sh"] = zuulSchedulerInitContainerScript
	schedulerToolingData["generate-zuul-tenant-yaml.sh"] = zuulGenerateTenantConfig
	schedulerToolingData["fetch-config-repo.sh"] = fetchConfigRepoScript

	r.EnsureConfigMap("zuul-scheduler-tooling", schedulerToolingData)

	zsVolumes := mkZuulVolumes("zuul-scheduler", r, corporateCMExists)

	zs := r.mkStatefulSet("zuul-scheduler", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Scheduler.Storage), apiv1.ReadWriteOnce)
	zs.Spec.Template.ObjectMeta.Annotations = annotations
	zs.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	zs.Spec.Template.Spec.Containers = zuulContainers
	zs.Spec.Template.Spec.Volumes = zsVolumes
	zs.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/health/live", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/health/ready", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zs.Spec.Template, path, true)
	}

	current, changed := r.ensureStatefulset(zs)
	if changed {
		return false
	}

	ready := r.IsStatefulSetReady(current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-scheduler", ready)

	return ready
}

func (r *SFController) EnsureZuulExecutor(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "executor")

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	annotations := map[string]string{
		"zuul-common-config":         utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config":      utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":                 getZuulImage("zuul-executor"),
		"serial":                     "7",
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-executor"))),
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}
	// TODO Add the zk-port-forward-kube-config secret resource version in the annotation if enabled

	ze := r.mkHeadlessSatefulSet("zuul-executor", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Executor.Storage), apiv1.ReadWriteOnce)
	zuulContainer := r.mkZuulContainer("zuul-executor", corporateCMExists)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Executor.Limits, &zuulContainer)
	ze.Spec.Template.Spec.Containers = []apiv1.Container{zuulContainer}
	ze.Spec.Template.Spec.Volumes = mkZuulVolumes("zuul-executor", r, corporateCMExists)

	zeFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-executor"})
	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-executor", r.cr.Spec.FluentBitLogForwarding, zeFluentBitLabels, annotations)
	ze.Spec.Template.Spec.Containers[0].Env = append(ze.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	ze.Spec.Template.ObjectMeta.Annotations = annotations

	ze.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	ze.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessHTTPProbe("/health/live", zuulPrometheusPort)
	ze.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
		base.MkContainerPort(zuulExecutorPort, zuulExecutorPortName),
	}
	// NOTE(dpawlik): Zuul Executor needs to privileged pod, due error in the console log:
	// "bwrap: Can't bind mount /oldroot/etc/resolv.conf on /newroot/etc/resolv.conf: Permission denied""
	ze.Spec.Template.Spec.Containers[0].SecurityContext = base.MkSecurityContext(true)
	ze.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = ptr.To[int64](1000)

	nodeExporterSidecar := monitoring.MkNodeExporterSideCarContainer(
		"zuul-executor",
		[]apiv1.VolumeMount{
			{
				Name:      "zuul-executor",
				MountPath: "/var/lib/zuul",
			},
		})
	ze.Spec.Template.Spec.Containers = append(ze.Spec.Template.Spec.Containers, nodeExporterSidecar)
	// FIXME: OpenShift doesn't seem very happy when containers in the same pod don't share
	// the same security context; or maybe it is because a volume is shared between the two?
	// Anyhow, the simplest fix is to elevate privileges on the node exporter sidecar.
	ze.Spec.Template.Spec.Containers[1].SecurityContext = base.MkSecurityContext(true)
	ze.Spec.Template.Spec.Containers[1].SecurityContext.RunAsUser = ptr.To[int64](1000)

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&ze.Spec.Template, path, false)
	}

	current, changed := r.ensureStatefulset(ze)
	if changed {
		return false
	}

	ready := r.IsStatefulSetReady(current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-executor", ready)

	return ready
}

func (r *SFController) EnsureZuulMerger(cfg *ini.File) bool {

	service := "zuul-merger"

	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "merger")

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	annotations := map[string]string{
		"zuul-common-config":         utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config":      utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":                 getZuulImage(service),
		"serial":                     "5",
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-merger"))),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	zm := r.mkHeadlessSatefulSet(service, "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Merger.Storage), apiv1.ReadWriteOnce)
	zuulContainer := r.mkZuulContainer(service, corporateCMExists)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Merger.Limits, &zuulContainer)
	zm.Spec.Template.Spec.Containers = []apiv1.Container{zuulContainer}
	zm.Spec.Template.Spec.Volumes = mkZuulVolumes(service, r, corporateCMExists)

	zmFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-merger"})
	extraLoggingEnvVars := logging.SetupLogForwarding(service, r.cr.Spec.FluentBitLogForwarding, zmFluentBitLabels, annotations)
	zm.Spec.Template.Spec.Containers[0].Env = append(zm.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	nodeExporterSidecar := monitoring.MkNodeExporterSideCarContainer(
		service,
		[]apiv1.VolumeMount{
			{
				Name:      service,
				MountPath: "/var/lib/zuul",
			},
		})
	zm.Spec.Template.Spec.Containers = append(zm.Spec.Template.Spec.Containers, nodeExporterSidecar)

	zm.Spec.Template.ObjectMeta.Annotations = annotations

	zm.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessHTTPProbe("/health/live", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zm.Spec.Template, path, false)
	}

	current, changed := r.ensureStatefulset(zm)
	if changed {
		return false
	}

	ready := r.IsStatefulSetReady(current)
	conds.UpdateConditions(&r.cr.Status.Conditions, service, ready)

	return ready
}

func (r *SFController) EnsureZuulWeb(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	authSections := utils.IniGetSectionNamesByPrefix(cfg, "auth")
	sections = append(sections, authSections...)
	sections = append(sections, "web")
	annotations := map[string]string{
		"zuul-common-config":    utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": utils.IniSectionsChecksum(cfg, sections),
		"zuul-image":            getZuulImage("zuul-web"),
		"serial":                "6",
		"zuul-logging":          utils.Checksum([]byte(r.getZuulLoggingString("zuul-web"))),
		"zuul-connections":      utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
	}

	zw := base.MkDeployment("zuul-web", r.ns, "")
	zuulContainer := r.mkZuulContainer("zuul-web", false)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Web.Limits, &zuulContainer)
	zw.Spec.Template.Spec.Containers = []apiv1.Container{zuulContainer}
	zw.Spec.Template.Spec.Volumes = mkZuulVolumes("zuul-web", r, false)

	zwFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-web"})
	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-web", r.cr.Spec.FluentBitLogForwarding, zwFluentBitLabels, annotations)
	zw.Spec.Template.Spec.Containers[0].Env = append(zw.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	zw.Spec.Template.ObjectMeta.Annotations = annotations

	zw.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zw.Spec.Template, path, false)
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

func (r *SFController) IsExecutorEnabled() bool {
	if r.cr.Spec.Zuul.Executor.Enabled != nil && !*r.cr.Spec.Zuul.Executor.Enabled {
		return false
	} else {
		return true
	}
}

func (r *SFController) EnsureZuulExecutorService() {
	headlessPorts := []int32{zuulExecutorPort}
	srvZE := base.MkHeadlessService("zuul-executor", r.ns, "zuul-executor", headlessPorts, "zuul-executor")
	r.GetOrCreate(&srvZE)
}

func (r *SFController) EnsureZuulComponentsFrontServices() {
	servicePorts := []int32{zuulWEBPort}
	srv := base.MkService("zuul-web", r.ns, "zuul-web", servicePorts, "zuul-web")
	r.GetOrCreate(&srv)

	if r.IsExecutorEnabled() {
		r.EnsureZuulExecutorService()
	}
}

func (r *SFController) EnsureZuulComponents() bool {

	// Ensure executor removed if disabled
	if r.cr.Spec.Zuul.Executor.Enabled != nil && !*r.cr.Spec.Zuul.Executor.Enabled {
		zuulExecutor := appsv1.StatefulSet{}
		if r.GetM("zuul-executor", &zuulExecutor) {
			r.log.Info("zuul-executor is disabled but running. Deleting the executor ...")
			r.DeleteR(&zuulExecutor)
		}
	}

	// Setup zuul.conf Secret
	cfg := r.EnsureZuulConfigSecret(false, false)
	if cfg == nil {
		return false
	}

	r.ensureZuulPromRule()

	// Install Services resources
	r.EnsureZuulComponentsFrontServices()

	// Init the zuul services status index
	zuulServices := map[string]bool{}

	zuulServices["scheduler"] = r.EnsureZuulScheduler(cfg)
	if r.IsExecutorEnabled() {
		zuulServices["executor"] = r.EnsureZuulExecutor(cfg)
	}
	zuulServices["web"] = r.EnsureZuulWeb(cfg)
	zuulServices["merger"] = r.EnsureZuulMerger(cfg)

	for _, ready := range maps.Values(zuulServices) {
		if !ready {
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
			"increase(zuul_executors_jobs_queued[1h]) > 0"),
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
			"increase(zuul_mergers_jobs_queued[1h]) > 0"),
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
			"increase(zuul_nodepool_current_requests[1h]) > 0"),
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

func (r *SFController) IsExternalExecutorEnabled() bool {
	return r.cr.Spec.Zuul.Executor.Standalone != nil
}

// EnsureZuulConfigSecret build and install the zuul.conf Secret resource
// If the resource cannot be built then the returned value is nil
func (r *SFController) EnsureZuulConfigSecret(skipDBSettings bool, skipAuthSettings bool) *ini.File {

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

	for _, conn := range r.cr.Spec.Zuul.PagureConns {
		r.AddPagureConnection(cfgINI, conn)
	}

	for _, conn := range r.cr.Spec.Zuul.GitConns {
		AddGitConnection(cfgINI, conn.Name, conn.Baseurl, conn.PollDelay)
	}

	for _, conn := range r.cr.Spec.Zuul.ElasticSearchConns {
		r.AddElasticSearchConnection(cfgINI, conn)
	}

	gitServerURL := "git://git-server/"
	if r.IsExternalExecutorEnabled() {
		gitServerURL = "git://" + r.cr.Spec.Zuul.Executor.Standalone.ControlPlanePublicGSHostname + "/"
	}
	// Add default connections
	r.AddDefaultConnections(cfgINI, gitServerURL)

	// Add Web Client for zuul-client
	AddWebClientSection(cfgINI)

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
	cfgINI.Section("web").NewKey("root", "https://"+r.cr.Spec.FQDN+"/zuul/")

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

	if !skipDBSettings {
		// Set Database DB URI
		dbSettings := apiv1.Secret{}
		if !r.GetM(zuulDBConfigSecret, &dbSettings) {
			r.log.Info("Waiting for db connection secret")
			return nil
		}
		cfgINI.Section("database").NewKey("dburi", fmt.Sprintf(
			"mysql+pymysql://%s:%s@%s/%s", dbSettings.Data["username"], dbSettings.Data["password"], dbSettings.Data["host"], dbSettings.Data["database"]))
	}

	// Set Zookeeper hosts
	zkHost := "zookeeper." + r.ns + ":2281"
	if r.IsExternalExecutorEnabled() {
		zkHost = r.cr.Spec.Zuul.Executor.Standalone.ControlPlanePublicZKHostname + ":2281"
	}
	cfgINI.Section("zookeeper").NewKey("hosts", zkHost)

	// Set executor public hostname (live job console support)
	// Zuul web needs to access that host on default finger port to stream live logs to user-agents
	if r.IsExternalExecutorEnabled() {
		if r.cr.Spec.Zuul.Executor.Standalone.PublicHostName != "" {
			cfgINI.Section("executor").NewKey("hostname", r.cr.Spec.Zuul.Executor.Standalone.PublicHostName)
		}
	}

	// Set Keystore secret
	keystorePass, err := r.getSecretData(ZuulKeystorePasswordName)
	if err != nil {
		r.log.Info("Waiting for " + ZuulKeystorePasswordName + " secret")
		return nil
	}
	cfgINI.Section("keystore").NewKey("password", string(keystorePass))

	if !skipAuthSettings {
		// Set CLI auth
		cliAuthSecret, err := r.getSecretData("zuul-auth-secret")
		if err != nil {
			r.log.Info("Waiting for zuul-auth-secret secret")
			return nil
		}
		cfgINI.Section("auth zuul_client").NewKey("secret", string(cliAuthSecret))
		cfgINI.Section("auth zuul_client").NewKey("realm", "zuul."+r.cr.Spec.FQDN)
	}

	// Configure statsd common config
	cfgINI.Section("statsd").NewKey("port", strconv.Itoa(int(monitoring.StatsdExporterPortListen)))

	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"zuul.conf": []byte(DumpConfigINI(cfgINI)),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.ns},
	})

	return cfgINI
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
	if conn.Sshkey != "" {
		cfg.Section(section).NewKey("sshkey", "/var/lib/zuul-"+conn.Sshkey+"/..data/priv")
	} else {
		cfg.Section(section).NewKey("sshkey", "/var/lib/zuul-ssh/..data/priv")
	}
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
	if conn.VerifySSL != nil && !*conn.VerifySSL {
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

	apiToken, apiTokenErr := r.GetSecretDataFromKey(conn.Secrets, "api_token")
	webHookToken, webHookTokenErr := r.GetSecretDataFromKey(conn.Secrets, "webhook_token")

	if apiTokenErr != nil {
		r.log.Error(apiTokenErr, "Use empty value for api_token on Gitlab connection due to err", "connection name", conn.Name)
	}
	if webHookTokenErr != nil {
		r.log.Error(webHookTokenErr, "Use empty value for webhook_token on Gitlab connection due to err", "connection name", conn.Name)
	}

	section := "connection " + conn.Name
	cfg.NewSection(section)

	for key, value := range map[string]string{
		"driver":                  "gitlab",
		"server":                  conn.Server,
		"canonical_hostname":      conn.CanonicalHostname,
		"baseurl":                 conn.BaseURL,
		"sshkey":                  "/var/lib/zuul-ssh/..data/priv",
		"cloneurl":                conn.CloneURL,
		"keepalive":               fmt.Sprint(conn.KeepAlive),
		"disable_connection_pool": fmt.Sprint(conn.DisableConnectionPool),
		"api_token_name":          conn.APITokenName,
	} {
		addKeyToSection(cfg.Section(section), key, value)
	}

	// addKeyToSection drops null (like empty string) keys
	// As those keys are mandatory for Zuul we simply adds them even with empty string
	cfg.Section(section).NewKey("api_token", string(apiToken))
	cfg.Section(section).NewKey("webhook_token", string(webHookToken))

}

func (r *SFController) AddPagureConnection(cfg *ini.File, conn sfv1.PagureConnection) {

	apiToken, _ := r.GetSecretDataFromKey(conn.Secrets, "api_token")

	section := "connection " + conn.Name
	cfg.NewSection(section)

	for key, value := range map[string]string{
		"driver":             "pagure",
		"server":             conn.Server,
		"canonical_hostname": conn.CanonicalHostname,
		"baseurl":            conn.BaseURL,
		"cloneurl":           conn.CloneURL,
		"api_token":          string(apiToken),
		"app_name":           conn.AppName,
		"source_whitelist":   conn.SourceWhitelist,
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

func (r *SFController) AddElasticSearchConnection(cfg *ini.File, conn sfv1.ElasticSearchConnection) {
	section := "connection " + conn.Name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "elasticsearch")
	cfg.Section(section).NewKey("uri", conn.URI)
	// Optional fields (set as omitempty in ElasticSearchConnection struct definition)
	if conn.UseSSL != nil && !*conn.UseSSL {
		cfg.Section(section).NewKey("use_ssl", "false")
	}
	if conn.VerifyCerts != nil && !*conn.VerifyCerts {
		cfg.Section(section).NewKey("verify_certs", "false")
	}
}

func AddWebClientSection(cfg *ini.File) {
	section := "webclient"
	cfg.NewSection(section)
	cfg.Section(section).NewKey("url", "http://zuul-web:"+strconv.FormatInt(zuulWEBPort, 10))
}

func (r *SFController) AddDefaultConnections(cfg *ini.File, gitServerURL string) {
	// Internal git-server for system config
	AddGitConnection(cfg, "git-server", gitServerURL, 0)

	// Git connection to opendev.org
	AddGitConnection(cfg, "opendev.org", "https://opendev.org/", 0)
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
	r.EnsureSecretUUID(ZuulKeystorePasswordName)
	r.EnsureSecretUUID("zuul-auth-secret")
}

func (r *SFController) DeployZuul() bool {
	return r.EnsureZuulComponents()
}

func (r *SFController) runZuulInternalTenantReconfigure() bool {
	err := r.PodExec(
		"zuul-scheduler-0",
		"zuul-scheduler",
		[]string{"zuul-scheduler", "tenant-reconfigure", "internal"})
	return err == nil
}
