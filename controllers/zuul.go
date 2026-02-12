// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	ini "gopkg.in/ini.v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"

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

	//go:embed static/zuul/reconnect-zk.py
	zuulReconnectZK string

	//go:embed static/zuul/rotate-keystore.py
	zuulRotateKeystore string

	//go:embed static/zuul/logging.yaml.tmpl
	zuulLoggingConfig string

	//go:embed static/zuul/zuul-change-dump.py.tmpl
	zuulChangeDump string

	//go:embed static/hound-search/hound-search-init.sh
	houndSearchInit string

	//go:embed static/hound-search/hound-search-config.sh
	houndSearchConfig string

	//go:embed static/hound-search/hound-search-render.py
	houndSearchRender string

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

func enableZuulLocalSource(template *apiv1.PodTemplateSpec, zuulSourceHostPath string, withInitContainer bool, openshiftUser bool) {
	template.Spec.Volumes = append(template.Spec.Volumes, apiv1.Volume{
		Name: "host-mount",
		VolumeSource: apiv1.VolumeSource{
			HostPath: &apiv1.HostPathVolumeSource{
				Path: zuulSourceHostPath,
			},
		},
	})
	template.Spec.Containers[0].SecurityContext = base.MkSecurityContext(true, openshiftUser)
	template.Spec.SecurityContext.RunAsNonRoot = ptr.To(false)
	template.Spec.Containers[0].VolumeMounts = append(template.Spec.Containers[0].VolumeMounts,
		apiv1.VolumeMount{
			Name:      "host-mount",
			MountPath: "/usr/local/lib/python3.11/site-packages/zuul",
		})
	if withInitContainer {
		template.Spec.InitContainers[0].SecurityContext = base.MkSecurityContext(true, openshiftUser)
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

func (r *SFKubeContext) mkKazooPod() *apiv1.Pod {
	return &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-kazoo", Namespace: r.Ns},
		Spec: apiv1.PodSpec{
			Volumes: []apiv1.Volume{
				base.MkVolumeSecret("zuul-config"),
				base.MkVolumeSecret("zookeeper-client-tls"),
				mkToolingVolume(),
				base.MkEmptyDirVolume("zuul-tmp"),
			},
			Containers: []apiv1.Container{
				{
					Name:    "zuul-kazoo",
					Image:   getZuulImage("zuul-scheduler"),
					Command: []string{"sleep", "inf"},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "zuul-config",
							MountPath: "/etc/zuul",
							ReadOnly:  true,
						}, {
							Name:      "zookeeper-client-tls",
							MountPath: "/tls/client",
							ReadOnly:  true,
						}, {
							Name:      "tooling-vol",
							SubPath:   "rotate-keystore.py",
							MountPath: "/usr/local/bin/rotate-keystore.py",
							ReadOnly:  true,
						}, {
							Name:      "zuul-tmp",
							MountPath: "/var/lib/zuul",
						},
					},
					SecurityContext: base.MkSecurityContext(false, r.IsOpenShift),
				},
			},
		},
	}
}

// EnsureKazooPod setups a kazoo client pod that can be used to access ZooKeeper directly
func (r *SFKubeContext) EnsureKazooPod() bool {
	var current apiv1.Pod
	if r.GetM("zuul-kazoo", &current) {
		if current.Status.Phase == "Running" {
			return true
		}
	} else {
		r.CreateR(r.mkKazooPod())
	}
	return false
}

func (r *SFKubeContext) DeleteKazooPod() {
	var current apiv1.Pod
	if r.GetM("zuul-kazoo", &current) {
		r.DeleteR(&current)
	}
}

func (r *SFController) mkZuulContainer(service string, corporateCMExists bool) apiv1.Container {

	mkZuulConnectionsSecretsMount := func(r *SFController) []apiv1.VolumeMount {
		added := []string{}
		zuulConnectionMounts := []apiv1.VolumeMount{}
		for _, conn := range r.cr.Spec.Zuul.GitHubConns {
			if conn.AppID > 0 && !slices.Contains(added, conn.Secrets) {
				zuulConnectionMounts = append(zuulConnectionMounts, apiv1.VolumeMount{
					Name:      conn.Secrets,
					MountPath: "/var/lib/zuul/" + conn.Secrets + "/app_key",
					SubPath:   "app_key",
				})
				added = append(added, conn.Secrets)
			}
		}

		for _, conn := range r.cr.Spec.Zuul.GerritConns {
			if conn.Sshkey != "" && !slices.Contains(added, conn.Sshkey) {
				keyMount := apiv1.VolumeMount{
					Name:      "zuul-ssh-key-" + conn.Sshkey,
					MountPath: "/var/lib/zuul-" + conn.Sshkey + "/",
				}
				zuulConnectionMounts = append(zuulConnectionMounts, keyMount)
				added = append(added, conn.Sshkey)
			}
		}

		return zuulConnectionMounts
	}

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
			Name:      "zuul-ca",
			MountPath: TrustedCAExtractedMountPath,
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
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "zuul-change-dump.py",
				MountPath: "/usr/local/bin/zuul-change-dump.py",
				ReadOnly:  true,
			},
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "reconnect-zk.py",
				MountPath: "/usr/local/bin/reconnect-zk.py",
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
		Command: []string{"/usr/local/bin/dumb-init", "-c", "--", "bash", "-c",
			// Trigger the update of the CA Trust chain
			UpdateCATrustCommand + " && " +
				// https://git-scm.com/docs/git-config#Documentation/git-config.txt-safedirectory
				// This is needed when we mount the local zuul source from the host
				// to bypass the git ownership verification
				"git config --global --add safe.directory '*'" + " && " +
				// Start the service
				"exec /usr/local/bin/" + service + " -f -d"},
		Env:             envs,
		VolumeMounts:    volumeMounts,
		SecurityContext: base.MkSecurityContext(false, r.IsOpenShift),
	}

	base.SetContainerLimitsHighProfile(&container)

	if service == "zuul-scheduler" {
		// For the scheduler we do not run the update-ca-trust because the initContainer
		// already handles that task.
		container.Command = []string{"/usr/local/bin/dumb-init", "-c", "--",
			"/usr/local/bin/zuul-scheduler", "-f", "-d"}
	}

	return container
}

func mkZuulVolumes(service string, r *SFController, corporateCMExists bool) []apiv1.Volume {
	mkZuulConnectionSecretsVolumes := func(r *SFController) []apiv1.Volume {
		added := []string{}
		volumes := []apiv1.Volume{}
		for _, conn := range r.cr.Spec.Zuul.GitHubConns {
			if conn.Secrets != "" && !slices.Contains(added, conn.Secrets) {
				volumes = append(volumes, base.MkVolumeSecret(conn.Secrets))
				added = append(added, conn.Secrets)
			}
		}
		for _, conn := range r.cr.Spec.Zuul.GerritConns {
			if conn.Sshkey != "" && !slices.Contains(added, conn.Sshkey) {
				keyVol := apiv1.Volume{
					Name: "zuul-ssh-key-" + conn.Sshkey,
					VolumeSource: apiv1.VolumeSource{
						Secret: &apiv1.SecretVolumeSource{
							SecretName:  conn.Sshkey,
							DefaultMode: &utils.Readmod,
						},
					},
				}
				volumes = append(volumes, keyVol)
				added = append(added, conn.Sshkey)
			}
		}
		return volumes
	}

	// create extra config config map
	r.EnsureConfigMap("zuul-extra", map[string]string{
		"ssh_config": sshConfig,
	})

	// create statsd exporter config map
	r.EnsureConfigMap("zuul-statsd", map[string]string{
		monitoring.StatsdExporterConfigFile: zuulStatsdMappingConfig,
	})

	// Install the logging settings config map resource
	r.EnsureConfigMap("zuul-logging", r.computeLoggingConfig())

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zuul-config"),
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeCM("zuul-logging-config", "zuul-logging-config-map"),
		{
			Name: "zuul-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "zuul-ssh-key",
					DefaultMode: &utils.Readmod,
				},
			},
		},
		base.MkVolumeCM("statsd-config", "zuul-statsd-config-map"),
		base.MkVolumeCM("extra-config", "zuul-extra-config-map"),
		base.MkVolumeCM("extra-python-module", "extra-python-module-config-map"),
		base.MkEmptyDirVolume("zuul-ca"),
	}
	if !isStatefulset(service) {
		// statefulset already has a PV for the service-name,
		// for the other, we use an empty dir.
		volumes = append(volumes, base.MkEmptyDirVolume(service))
	}
	if service == "zuul-scheduler" {
		volumes = AppendToolingVolume(volumes)
	}

	volumes = append(volumes, mkZuulConnectionSecretsVolumes(r)...)

	if corporateCMExists {
		volumes = append(volumes, base.MkVolumeCM(service+"-corporate-ca-certs", CorporateCACerts))
	}

	return volumes
}

func (r *SFController) getTenantsEnvs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		branch := r.cr.Spec.ConfigRepositoryLocation.Branch
		if branch == "" {
			// Default to "main" when not set
			branch = "main"
		}
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "TRUE"),
			base.MkEnvVar("CONFIG_REPO_BASE_URL", strings.TrimSuffix(r.configBaseURL, "/")),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
			base.MkEnvVar("CONFIG_REPO_CONNECTION_NAME", r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName),
			base.MkEnvVar("CONFIG_REPO_BRANCH", branch),
		}
	} else {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) computeLoggingConfig() map[string]string {
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

	var zeloggingParams = logging.CreateForwarderConfigTemplateParams("zuul.executor", r.cr.Spec.FluentBitLogForwarding)
	var zeloggingExtraKeys = logging.CreateBaseLoggingExtraKeys("zuul-executor", "zuul", "zuul-executor", r.Ns)
	// Change logLevel to what we actually want
	zeloggingParams.LogLevel = string(zuulExecutorLogLevel)
	loggingData["zuul-executor-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{zeloggingExtraKeys, zeloggingParams})

	var zsloggingParams = logging.CreateForwarderConfigTemplateParams("zuul.scheduler", r.cr.Spec.FluentBitLogForwarding)
	var zsloggingExtraKeys = logging.CreateBaseLoggingExtraKeys("zuul-scheduler", "zuul", "zuul-scheduler", r.Ns)
	// Change logLevel to what we actually want
	zsloggingParams.LogLevel = string(zuulSchedulerLogLevel)
	loggingData["zuul-scheduler-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{zsloggingExtraKeys, zsloggingParams})

	var zwloggingParams = logging.CreateForwarderConfigTemplateParams("zuul.web", r.cr.Spec.FluentBitLogForwarding)
	var zwloggingExtraKeys = logging.CreateBaseLoggingExtraKeys("zuul-web", "zuul", "zuul-web", r.Ns)
	// Change logLevel to what we actually want
	zwloggingParams.LogLevel = string(zuulWebLogLevel)
	loggingData["zuul-web-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{zwloggingExtraKeys, zwloggingParams})

	var zmloggingParams = logging.CreateForwarderConfigTemplateParams("zuul.merger", r.cr.Spec.FluentBitLogForwarding)
	var zmloggingExtraKeys = logging.CreateBaseLoggingExtraKeys("zuul-merger", "zuul", "zuul-merger", r.Ns)
	// Change logLevel to what we actually want
	zmloggingParams.LogLevel = string(zuulMergerLogLevel)
	loggingData["zuul-merger-logging.yaml"], _ = utils.ParseString(
		zuulLoggingConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{zmloggingExtraKeys, zmloggingParams})

	return loggingData
}

func (r *SFController) getZuulLoggingString(service string) string {
	loggingData := r.computeLoggingConfig()
	return loggingData[service+"-logging.yaml"]
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
		"statsd_mapping":             utils.Checksum([]byte(zuulStatsdMappingConfig)),
		"serial":                     "13",
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-scheduler"))),
		"zuul-extra":                 utils.Checksum([]byte(sshConfig)),
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName + ":" +
			r.configBaseURL +
			r.cr.Spec.ConfigRepositoryLocation.Name +
			r.cr.Spec.ConfigRepositoryLocation.Branch
	}

	var relayAddress *string
	if r.cr.Spec.Zuul.Scheduler.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Zuul.Scheduler.StatsdTarget
	}

	zuulContainer := r.mkZuulContainer("zuul-scheduler", corporateCMExists)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Scheduler.Limits, &zuulContainer)

	zsFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-scheduler"})
	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-scheduler", r.cr.Spec.FluentBitLogForwarding, zsFluentBitLabels, annotations)
	zuulContainer.Env = append(zuulContainer.Env, extraLoggingEnvVars...)

	statsdSidecar := monitoring.MkStatsdExporterSideCarContainer("zuul", "statsd-config", relayAddress, r.IsOpenShift)
	nodeExporterSidecar := monitoring.MkNodeExporterSideCarContainer(
		"zuul-scheduler",
		[]apiv1.VolumeMount{
			{
				Name:      "zuul-scheduler",
				MountPath: "/var/lib/zuul",
			},
		}, r.IsOpenShift)

	zuulContainers := append([]apiv1.Container{}, zuulContainer, statsdSidecar, nodeExporterSidecar)

	initContainer := base.MkContainer("init-scheduler-config", getZuulImage("zuul-scheduler"), r.IsOpenShift)
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
			MountPath: TrustedCAExtractedMountPath,
		},
	}
	if corporateCMExists {
		initContainer.VolumeMounts = AppendCorporateCACertsVolumeMount(initContainer.VolumeMounts, "zuul-scheduler-corporate-ca-certs")
	}
	base.SetContainerLimitsLowProfile(&initContainer)

	zsVolumes := mkZuulVolumes("zuul-scheduler", r, corporateCMExists)

	zsStorage := r.getStorageConfOrDefault(r.cr.Spec.Zuul.Scheduler.Storage)
	zs := r.mkStatefulSet("zuul-scheduler", "", zsStorage, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.IsOpenShift)
	zs.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	zs.Spec.Template.Spec.Containers = zuulContainers
	zs.Spec.Template.Spec.Volumes = zsVolumes
	if !r.IsOpenShift {
		zs.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = ptr.To[int64](1001)
	}
	zs.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/health/live", zuulPrometheusPort)
	zs.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/health/ready", zuulPrometheusPort)
	base.SlowStartingProbe(zs.Spec.Template.Spec.Containers[0].StartupProbe)
	zs.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}
	zs.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	zs.Spec.Template.ObjectMeta.Annotations = annotations

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zs.Spec.Template, path, true, r.IsOpenShift)
	}

	current, changed := r.ensureStatefulset(zsStorage.StorageClassName, zs, nil)
	if changed {
		return false
	}

	ready := r.IsStatefulSetReady(current)
	conds.UpdateConditions(&r.cr.Status.Conditions, "zuul-scheduler", ready)

	if ready {
		// TODO: make this configurable
		return r.EnsureZuulWeeder(annotations["zuul-connections"])
	}

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
		"serial":                     "11",
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-executor"))),
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}
	// TODO Add the zk-port-forward-kube-config secret resource version in the annotation if enabled

	zeStorage := r.getStorageConfOrDefault(r.cr.Spec.Zuul.Executor.Storage)
	ze := r.mkHeadlessStatefulSet("zuul-executor", "", zeStorage, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.IsOpenShift)
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
	ze.Spec.Template.Spec.Containers[0].SecurityContext = base.MkSecurityContext(!r.hasProcMount, r.IsOpenShift)
	zeuid := int64(1000)
	if !r.IsOpenShift {
		zeuid = int64(65534)
	}
	ze.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = ptr.To[int64](zeuid)
	if r.hasProcMount {
		ze.Spec.Template.ObjectMeta.Annotations["io.kubernetes.cri-o.userns-mode"] = "auto"
		ze.Spec.Template.Spec.HostUsers = ptr.To[bool](false)
		ze.Spec.Template.Spec.Containers[0].SecurityContext.ProcMount = ptr.To[apiv1.ProcMountType](apiv1.UnmaskedProcMount)

		// The default seccomp profile doesn't allow nested namespace creation:
		//   bwrap: No permissions to creating new namespace, likely because the kernel does not allow non-privileged user namespaces.
		// Zuul fails with:
		//   Exception: bwrap execution validation failed. You can use `zuul-bwrap /tmp id` to investigate manually.
		// To try, run:
		//   kubectl --namespace sf exec zuul-executor-0 -- bwrap --ro-bind /lib /lib --ro-bind /usr /usr --symlink /usr/lib64 /lib64 --proc /proc --dev /dev --tmpfs /tmp --unshare-all --new-session ps afx
		// FIXME: find the right SeccompProfile that can be used for bwrap
		ze.Spec.Template.Spec.Containers[0].SecurityContext.SeccompProfile = nil
		ze.Spec.Template.Spec.SecurityContext.SeccompProfile = nil
	}

	nodeExporterSidecar := monitoring.MkNodeExporterSideCarContainer(
		"zuul-executor",
		[]apiv1.VolumeMount{
			{
				Name:      "zuul-executor",
				MountPath: "/var/lib/zuul",
			},
		}, r.IsOpenShift)
	ze.Spec.Template.Spec.Containers = append(ze.Spec.Template.Spec.Containers, nodeExporterSidecar)
	// FIXME: OpenShift doesn't seem very happy when containers in the same pod don't share
	// the same security context; or maybe it is because a volume is shared between the two?
	// Anyhow, the simplest fix is to elevate privileges on the node exporter sidecar.
	ze.Spec.Template.Spec.Containers[1].SecurityContext = base.MkSecurityContext(true, r.IsOpenShift)
	ze.Spec.Template.Spec.Containers[1].SecurityContext.RunAsUser = ptr.To[int64](1000)
	ze.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	// Setup max graceful period
	period := r.cr.Spec.Zuul.Executor.TerminationGracePeriodSeconds
	if period == 0 {
		period = 7200
	}
	ze.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To[int64](period)

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&ze.Spec.Template, path, false, r.IsOpenShift)
	}

	current, changed := r.ensureStatefulset(zeStorage.StorageClassName, ze, nil)
	if changed {
		return false
	}

	pvcReadiness := r.reconcileExpandPVCs("zuul-executor", r.cr.Spec.Zuul.Executor.Storage)

	ready := r.IsStatefulSetReady(current) && pvcReadiness
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
		"serial":                     "8",
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-merger"))),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	zmStorage := r.getStorageConfOrDefault(r.cr.Spec.Zuul.Merger.Storage)
	zm := r.mkHeadlessStatefulSet(service, "", zmStorage, apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.IsOpenShift)
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
		}, r.IsOpenShift)
	zm.Spec.Template.Spec.Containers = append(zm.Spec.Template.Spec.Containers, nodeExporterSidecar)

	zm.Spec.Template.ObjectMeta.Annotations = annotations

	if !r.IsOpenShift {
		zm.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = ptr.To[int64](1001)
	}
	zm.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/health/ready", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessHTTPProbe("/health/live", zuulPrometheusPort)
	zm.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}
	zm.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zm.Spec.Template, path, false, r.IsOpenShift)
	}

	current, changed := r.ensureStatefulset(zmStorage.StorageClassName, zm, nil)
	if changed {
		return false
	}

	pvcReadiness := r.reconcileExpandPVCs("zuul-merger", r.cr.Spec.Zuul.Merger.Storage)

	ready := r.IsStatefulSetReady(current) && pvcReadiness
	conds.UpdateConditions(&r.cr.Status.Conditions, service, ready)

	return ready
}

func (r *SFController) EnsureZuulWeb(cfg *ini.File) bool {
	sections := utils.IniGetSectionNamesByPrefix(cfg, "connection")
	authSections := utils.IniGetSectionNamesByPrefix(cfg, "auth")
	sections = append(sections, authSections...)
	sections = append(sections, "web")

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	annotations := map[string]string{
		"zuul-common-config":         utils.IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config":      utils.IniSectionsChecksum(cfg, sections),
		"serial":                     "9",
		"zuul-logging":               utils.Checksum([]byte(r.getZuulLoggingString("zuul-web"))),
		"zuul-connections":           utils.IniSectionsChecksum(cfg, utils.IniGetSectionNamesByPrefix(cfg, "connection")),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	zw := base.MkDeployment("zuul-web", r.Ns, "", r.cr.Spec.ExtraLabels, r.IsOpenShift)
	zuulContainer := r.mkZuulContainer("zuul-web", corporateCMExists)
	annotations["limits"] = base.UpdateContainerLimit(r.cr.Spec.Zuul.Web.Limits, &zuulContainer)
	zw.Spec.Template.Spec.Containers = []apiv1.Container{zuulContainer}
	zw.Spec.Template.Spec.Volumes = mkZuulVolumes("zuul-web", r, corporateCMExists)

	zwFluentBitLabels := append(zuulFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: "zuul-web"})
	extraLoggingEnvVars := logging.SetupLogForwarding("zuul-web", r.cr.Spec.FluentBitLogForwarding, zwFluentBitLabels, annotations)
	zw.Spec.Template.Spec.Containers[0].Env = append(zw.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)

	zw.Spec.Template.ObjectMeta.Annotations = annotations

	if !r.IsOpenShift {
		zw.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = ptr.To[int64](1001)
	}
	zw.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/api/info", zuulWEBPort)
	zw.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/api/info", zuulWEBPort)
	base.SlowStartingProbe(zw.Spec.Template.Spec.Containers[0].StartupProbe)
	zw.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(zuulPrometheusPort, ZuulPrometheusPortName),
	}
	zw.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	// Mount a local directory in place of the Zuul source from the container image
	if path, _ := utils.GetEnvVarValue("ZUUL_LOCAL_SOURCE"); path != "" {
		enableZuulLocalSource(&zw.Spec.Template, path, false, r.IsOpenShift)
	}

	current, changed := r.ensureDeployment(zw, nil)
	if changed {
		return false
	}

	isDeploymentReady := r.IsDeploymentReady(current)
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
	srvZE := base.MkHeadlessService("zuul-executor", r.Ns, "zuul-executor", headlessPorts, "zuul-executor", r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srvZE)
}

func (r *SFController) EnsureZuulComponentsFrontServices() {
	servicePorts := []int32{zuulWEBPort}
	srv := base.MkService("zuul-web", r.Ns, "zuul-web", servicePorts, "zuul-web", r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srv)

	if r.IsExecutorEnabled() {
		r.EnsureZuulExecutorService()
	}
}

func (r *SFController) EnsureZuulComponents() map[string]bool {

	componentStatus := make(map[string]bool)
	componentStatus["Zuul"] = false
	componentStatus["|_ Config Secrets"] = false

	// Ensure executor removed if disabled
	if r.cr.Spec.Zuul.Executor.Enabled != nil && !*r.cr.Spec.Zuul.Executor.Enabled {
		zuulExecutor := appsv1.StatefulSet{}
		if r.GetM("zuul-executor", &zuulExecutor) {
			logging.LogI("zuul-executor is disabled but running. Deleting the executor ...")
			r.DeleteR(&zuulExecutor)
		}
	}

	// Init the zuul services status index
	zuulServices := map[string]bool{}

	// Setup zuul.conf Secret
	cfg := r.EnsureZuulConfigSecret(false)
	if cfg == nil {
		return componentStatus
	} else {
		componentStatus["|_ Config Secrets"] = true
	}

	componentStatus["|_ Scheduler"] = false
	componentStatus["|_ Web"] = false
	componentStatus["|_ Merger"] = false

	// Install Services resources
	r.EnsureZuulComponentsFrontServices()

	zuulServices["Scheduler"] = r.EnsureZuulScheduler(cfg)
	if r.IsExecutorEnabled() {
		zuulServices["Executor"] = r.EnsureZuulExecutor(cfg)
	}
	zuulServices["Web"] = r.EnsureZuulWeb(cfg)
	zuulServices["Merger"] = r.EnsureZuulMerger(cfg)

	componentStatus["Zuul"] = true
	for ready := range zuulServices {
		componentStatus["|_ "+ready] = zuulServices[ready]
		componentStatus["Zuul"] = componentStatus["Zuul"] && zuulServices[ready]
	}

	return componentStatus
}

func (r *SFController) IsExternalExecutorEnabled() bool {
	return r.cr.Spec.Zuul.Executor.Standalone != nil
}

// EnsureZuulConfigSecret build and install the zuul.conf Secret resource
// If the resource cannot be built then the returned value is nil
func (r *SFController) EnsureZuulConfigSecret(remoteExecutor bool) *ini.File {

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

	for _, conn := range r.cr.Spec.Zuul.SMTPConns {
		r.AddSMTPConnection(cfgINI, conn)
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

	// Set scheduler settings
	if r.cr.Spec.Zuul.Scheduler.DefaultHoldExpiration != nil {
		cfgINI.Section("scheduler").NewKey("default_hold_expiration", strconv.Itoa(int(*r.cr.Spec.Zuul.Scheduler.DefaultHoldExpiration)))
	}
	if r.cr.Spec.Zuul.Scheduler.MaxHoldExpiration != nil {
		cfgINI.Section("scheduler").NewKey("max_hold_expiration", strconv.Itoa(int(*r.cr.Spec.Zuul.Scheduler.MaxHoldExpiration)))
	}

	// Set executor settings
	if r.cr.Spec.Zuul.Executor.DiskLimitPerJob != 0 {
		size := int(r.cr.Spec.Zuul.Executor.DiskLimitPerJob)
		if size < -1 {
			size = -1
		}
		cfgINI.Section("executor").NewKey("disk_limit_per_job", strconv.Itoa(size))
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

	if !remoteExecutor {
		// Set Database DB URI
		dbSettings := apiv1.Secret{}
		if !r.GetM(zuulDBConfigSecret, &dbSettings) {
			logging.LogI("Waiting for db connection secret")
			return nil
		}
		cfgINI.Section("database").NewKey("dburi", fmt.Sprintf(
			"mariadb+pymysql://%s:%s@%s/%s", dbSettings.Data["username"], dbSettings.Data["password"], dbSettings.Data["host"], dbSettings.Data["database"]))
	}

	// Set Zookeeper hosts
	var zkHost string
	if r.IsExternalExecutorEnabled() {
		if r.cr.Spec.Zuul.Executor.Standalone.ControlPlanePublicZKHostnames != nil {
			publicZKHostnames := *r.cr.Spec.Zuul.Executor.Standalone.ControlPlanePublicZKHostnames
			for _, h := range publicZKHostnames {
				zkHost = zkHost + h + ":2281,"
			}
		} else {
			logging.LogI("controlPlanePublicZKHostname (string) is deprecated, use controlePlanePublicZKHostnames ([]string) instead")
			zkHost = r.cr.Spec.Zuul.Executor.Standalone.ControlPlanePublicZKHostname + ":2281"
		}
	} else {
		for i := range ZookeeperReplicas {
			zkHost = zkHost + fmt.Sprintf("%s-%d.%s-headless.%s:2281,", ZookeeperIdent, i, ZookeeperIdent, r.Ns)
		}
	}
	zkHost = strings.TrimSuffix(zkHost, ",")
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
		logging.LogI("Waiting for " + ZuulKeystorePasswordName + " secret")
		return nil
	}
	cfgINI.Section("keystore").NewKey("password", string(keystorePass))

	if !remoteExecutor {
		// Set CLI auth
		cliAuthSecret, err := r.getSecretData("zuul-auth-secret")
		if err != nil {
			logging.LogI("Waiting for zuul-auth-secret secret")
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
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.Ns},
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
	if authenticator.UIDClaim == "" {
		cfg.Section(section).NewKey("uid_claim", "sub")
	} else {
		cfg.Section(section).NewKey("uid_claim", authenticator.UIDClaim)
	}
	if authenticator.MaxValidityTime != 0 {
		cfg.Section(section).NewKey("max_validity_time", strconv.Itoa(int(authenticator.MaxValidityTime)))
	}
	if authenticator.Skew != 0 {
		cfg.Section(section).NewKey("skew", strconv.Itoa(int(authenticator.Skew)))
	}
	if authenticator.Scope == "" {
		cfg.Section(section).NewKey("scope", "openid profile")
	} else {
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
	if conn.StreamEvents != nil && !*conn.StreamEvents {
		cfg.Section(section).NewKey("stream_events", "false")
	}
}

// addKeyToSection add a tuple to the Section if the fieldValue is not empty
func addKeyToSection(section *ini.Section, fieldKey string, fieldValue string) {
	// server key is mandatory.
	if fieldKey == "server" || fieldValue != "" {
		if fieldValue == "" {
			// if the server is not set, use localhost
			// todo: report an error to the user
			fieldValue = "localhost"
		}
		section.NewKey(fieldKey, fieldValue)
	}
}

func (r *SFController) AddGitHubConnection(cfg *ini.File, conn sfv1.GitHubConnection) {

	appID := fmt.Sprintf("%d", conn.AppID)
	appKey := "/var/lib/zuul/" + conn.Secrets + "/app_key"

	if appKey == "" || appID == "0" {
		logging.LogI(fmt.Sprintf("app_key or app_id is not defined, app_key: %s, app_id: %s", appKey, appID))
		appKey = ""
		appID = ""
	}

	apiToken, err := r.GetSecretDataFromKey(conn.Secrets, "api_token")
	if err != nil {
		logging.LogE(err, "Unable to find 'api_token' in Secret: "+conn.Secrets)
	}

	webhookToken, err := r.GetSecretDataFromKey(conn.Secrets, "webhook_token")
	if err != nil {
		logging.LogE(err, "Unable to find 'webhook_token' in Secret: "+conn.Secrets)
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
		logging.LogE(apiTokenErr, "Use empty value for api_token on Gitlab connection due to err, connection name: "+conn.Name)
	}
	if webHookTokenErr != nil {
		logging.LogE(webHookTokenErr, "Use empty value for webhook_token on Gitlab connection due to err, connection name: "+conn.Name)
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
		"api_token_name":          "zuul",
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
	scheme := ""
	uri := conn.URI
	// crude clear-text basic auth check
	if match, _ := regexp.MatchString("http[s]?://[^:]+:.+@.+", uri); match {
		logging.LogI(fmt.Sprintf("It looks like elasticsearch connection %s has basic auth secrets stored in clear text. Use the 'basicAuthSecret' property instead", conn.Name))
	}
	if strings.HasPrefix(uri, "https://") {
		scheme = "https://"
		// TODO might not work with unicode URLs
		uri = uri[len("https://"):]
	}
	if strings.HasPrefix(uri, "http://") {
		scheme = "http://"
		// TODO might not work with unicode URLs
		uri = uri[len("http://"):]
	}
	if conn.BasicAuthSecret != nil {
		password, passwordErr := r.GetSecretDataFromKey(*conn.BasicAuthSecret, "password")
		// TODO we may also want to handle missing values in the secret
		if errors.IsNotFound(passwordErr) {
			logging.LogE(passwordErr, fmt.Sprintf("elasticsearch connection %s refers to a non-existing secret: %s ", conn.Name, *conn.BasicAuthSecret))
		}
		username, _ := r.GetSecretDataFromKey(*conn.BasicAuthSecret, "username")
		uri = string(username) + ":" + string(password) + "@" + uri
	}
	uri = scheme + uri

	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "elasticsearch")
	cfg.Section(section).NewKey("ca_certs", "/etc/ssl/certs/ca-bundle.crt")
	cfg.Section(section).NewKey("uri", uri)
	// Optional fields (set as omitempty in ElasticSearchConnection struct definition)
	if conn.UseSSL != nil && !*conn.UseSSL {
		cfg.Section(section).NewKey("use_ssl", "false")
	}
	if conn.VerifyCerts != nil && !*conn.VerifyCerts {
		cfg.Section(section).NewKey("verify_certs", "false")
	}
}

func (r *SFController) AddSMTPConnection(cfg *ini.File, conn sfv1.SMTPConnection) {
	section := "connection " + conn.Name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "smtp")
	cfg.Section(section).NewKey("server", conn.Server)
	// Optional fields (set as omitempty in SMTPConnection struct definition)
	if conn.Port > 0 {
		cfg.Section(section).NewKey("port", strconv.Itoa(int(conn.Port)))
	}
	if conn.DefaultFrom != "" {
		cfg.Section(section).NewKey("default_from", conn.DefaultFrom)
	}
	if conn.DefaultTo != "" {
		cfg.Section(section).NewKey("default_to", conn.DefaultTo)
	}
	if conn.User != "" {
		cfg.Section(section).NewKey("user", conn.User)
	}
	if conn.Secrets != nil {
		password, passwordErr := r.GetSecretDataFromKey(*conn.Secrets, "password")
		if errors.IsNotFound(passwordErr) {
			logging.LogE(passwordErr, fmt.Sprintf("SMTP connection %s refers to a non-existing secret: %s ", conn.Name, *conn.Secrets))
		}
		cfg.Section(section).NewKey("password", string(password))
	} else {
		if conn.Password != "" {
			logging.LogDeprecation("SMTPConnection's Password field will disappear in a future version. Use Secrets instead")
			cfg.Section(section).NewKey("password", conn.Password)
		}
	}
	if conn.TLS != nil && !*conn.TLS {
		cfg.Section(section).NewKey("use_starttls", "false")
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

	if r.needOpendev {
		// Git connection to opendev.org
		AddGitConnection(cfg, "opendev.org", "https://opendev.org/", 0)
	}
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
	sections := cfg.Sections()

	for _, section := range sections {

		keys := section.Keys()
		sortedKeys := []*ini.Key{}

		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Name() < keys[j].Name()
		})

		for _, key := range keys {
			sortedKeys = append(sortedKeys, key)
			section.DeleteKey(key.Name())
		}

		for _, key := range sortedKeys {
			section.NewKey(key.Name(), key.Value())
		}

	}
	cfg.WriteTo(writer)
	return writer.String()
}

func (r *SFController) DeployZuulSecrets() {
	r.EnsureSSHKeySecret("zuul-ssh-key")
	r.EnsureSecretUUID(ZuulKeystorePasswordName)
	r.EnsureSecretUUID("zuul-auth-secret")
}

func (r *SFController) DeployZuul() map[string]bool {
	return r.EnsureZuulComponents()
}

func (r *SFController) runZuulInternalTenantReconfigure() bool {
	err := r.PodExec(
		"zuul-scheduler-0",
		"zuul-scheduler",
		[]string{"zuul-scheduler", "tenant-reconfigure", "internal"})
	return err == nil
}
