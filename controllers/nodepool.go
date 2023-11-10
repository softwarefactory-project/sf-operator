// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package controllers provides controller functions
package controllers

import (
	_ "embed"
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
)

//go:embed static/nodepool/generate-config.sh
var generateConfigScript string

//go:embed static/nodepool/logging.yaml.tmpl
var loggingConfigTemplate string

//go:embed static/nodepool/dib-ansible.py
var dibAnsibleWrapper string

//go:embed static/nodepool/ssh_config
var builderSSHConfig string

//go:embed static/nodepool/statsd_mapping.yaml.tmpl
var nodepoolStatsdMappingConfigTemplate string

//go:embed static/nodepool/httpd-build-logs-dir.conf
var httpdBuildLogsDirConfig string

const (
	nodepoolIdent                = "nodepool"
	launcherIdent                = nodepoolIdent + "-launcher"
	shortIdent                   = "np"
	launcherPortName             = "nlwebapp"
	launcherPort                 = 8006
	buildLogsHttpdPort           = 8080
	buildLogsHttpdPortName       = "buildlogs-http"
	NodepoolProvidersSecretsName = "nodepool-providers-secrets"
	builderIdent                 = nodepoolIdent + "-builder"
)

var nodepoolStatsdExporterPortName = monitoring.GetStatsdExporterPort(shortIdent)

var configScriptVolumeMount = apiv1.VolumeMount{
	Name:      "nodepool-tooling-vol",
	SubPath:   "generate-config.sh",
	MountPath: "/usr/local/bin/generate-config.sh",
	ReadOnly:  true,
}

func (r *SFController) setNodepoolTooling() {
	toolingData := make(map[string]string)
	toolingData["generate-config.sh"] = generateConfigScript
	toolingData["dib-ansible.py"] = dibAnsibleWrapper
	toolingData["ssh_config"] = builderSSHConfig
	r.EnsureConfigMap("nodepool-tooling", toolingData)
}

func (r *SFController) commonToolingVolume() apiv1.Volume {
	return apiv1.Volume{
		Name: "nodepool-tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "nodepool-tooling-config-map",
				},
				DefaultMode: &utils.Execmod,
			},
		},
	}
}

func (r *SFController) getNodepoolConfigEnvs() []apiv1.EnvVar {
	nodepoolEnvVars := []apiv1.EnvVar{}
	if r.isConfigRepoSet() {
		nodepoolEnvVars = append(nodepoolEnvVars,
			base.MkEnvVar("CONFIG_REPO_SET", "TRUE"),
			base.MkEnvVar("CONFIG_REPO_BASE_URL", r.cr.Spec.ConfigLocation.BaseURL),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
		)
	} else {
		nodepoolEnvVars = append(nodepoolEnvVars,
			base.MkEnvVar("CONFIG_REPO_SET", "FALSE"),
		)
	}
	nodepoolEnvVars = append(nodepoolEnvVars,
		base.MkEnvVar("HOME", "/var/lib/nodepool"),
		base.MkEnvVar("STATSD_HOST", "localhost"),
		base.MkEnvVar("STATSD_PORT", strconv.Itoa(int(monitoring.StatsdExporterPortListen))),
	)
	return nodepoolEnvVars
}

func mkLoggingTemplate(logLevel v1.LogLevel) (string, error) {
	// Unfortunatly I'm unable to leverage default value set at API validation
	selectedLogLevel := v1.InfoLogLevel
	if logLevel != "" {
		selectedLogLevel = logLevel
	}

	loggingConfig, err := utils.ParseString(
		loggingConfigTemplate, struct{ LogLevel string }{LogLevel: string(selectedLogLevel)})

	return loggingConfig, err
}

func mkStatsdMappingConfig(cloudsYaml map[string]interface{}) (string, error) {
	var extraMappings []monitoring.StatsdMetricMapping

	extraMappings = monitoring.MkStatsdMappingsFromCloudsYaml(extraMappings, cloudsYaml)

	statsdMappingConfig, err := utils.ParseString(
		nodepoolStatsdMappingConfigTemplate, extraMappings)
	return statsdMappingConfig, err
}

func (r *SFController) EnsureNodepoolPodMonitor() bool {
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "run",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{launcherIdent, builderIdent},
			},
			{
				Key:      "app",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"sf"},
			},
		},
	}
	desiredNodepoolMonitor := monitoring.MkPodMonitor("nodepool-monitor", r.ns, []string{nodepoolStatsdExporterPortName}, selector)
	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version": "1",
	}
	desiredNodepoolMonitor.ObjectMeta.Annotations = annotations
	currentNPM := monitoringv1.PodMonitor{}
	if !r.GetM(desiredNodepoolMonitor.Name, &currentNPM) {
		r.CreateR(&desiredNodepoolMonitor)
		return false
	} else {
		if !utils.MapEquals(&currentNPM.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool PodMonitor configuration changed, updating...")
			currentNPM.Spec = desiredNodepoolMonitor.Spec
			currentNPM.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentNPM)
			return false
		}
	}
	return true
}

// create default alerts
func (r *SFController) ensureNodepoolPromRule(cloudsYaml map[string]interface{}) bool {
	var extraOpenStackMappings []monitoring.StatsdMetricMapping

	extraOpenStackMappings = monitoring.MkStatsdMappingsFromCloudsYaml(extraOpenStackMappings, cloudsYaml)

	/* Alert when more than 5% of node launches resulted in failure in the last hour with any provider */
	highLaunchErrorRateAnnotations := map[string]string{
		"description": "More than 5% ({{ $value }}%) of node launch events for provider {{ $labels.provider }} were failures in the last hour",
		"summary":     "Too many nodes failing to launch on provider {{ $labels.provider }}",
	}

	highLaunchErrorRate := monitoring.MkPrometheusAlertRule(
		"HighNodeLaunchErrorRate",
		intstr.FromString(
			"sum(rate(nodepool_launch_provider_error{error=~'.*'}[1h]))"+
				" / (sum(rate(nodepool_launch_provider_ready[1h])) + "+
				"sum(rate(nodepool_launch_provider_error{error=~'.*'}[1h]))) * 100 > 5"),
		"1h",
		monitoring.CriticalSeverityLabel,
		highLaunchErrorRateAnnotations,
	)

	/* Alert when a DIB build failed */
	dibImageBuildFailureAnnotations := map[string]string{
		"summary":     "DIB failure: {{ $labels.diskimage }}",
		"description": "DIB could not build image {{ $labels.diskimage }}, check build logs",
	}
	dibImageBuildFailure := monitoring.MkPrometheusAlertRule(
		"DIBImageBuildFailure",
		intstr.FromString(
			"nodepool_dib_image_build_status_rc != 0"),
		"0m",
		monitoring.WarningSeverityLabel,
		dibImageBuildFailureAnnotations,
	)

	/* Alert when more than 5% of nodes are in "failed" state for more than 1h with any provider */
	highFailedStateRateAnnotations := map[string]string{
		"description": "More than 5% ({{ $value }}%) of nodes were in failed state in the last hour on provider {{ $labels.provider }}",
		"summary":     "Too many failed nodes on provider {{ $labels.provider }}",
	}

	highFailedStateRate := monitoring.MkPrometheusAlertRule(
		"HighFailedStateRate",
		intstr.FromString(
			"sum(rate(nodepool_provider_nodes{state='failed'}[1h]))"+
				" / sum(rate(nodepool_launch_provider_error{state=~'.*'}[1h])) * 100 > 5"),
		"1h",
		monitoring.CriticalSeverityLabel,
		highFailedStateRateAnnotations,
	)

	/* Alert when more than 5% of OpenStack API calls return with status 5xx */
	var openstackAPIRules = []monitoringv1.Rule{}
	for _, mapping := range extraOpenStackMappings {
		var alertName = "HighOpenStackAPIError5xxRate"
		error5xxRateAnnotations := make(map[string]string)
		error5xxRateAnnotations["description"] = "More than 5% ({{ $value }}%) of API calls to service {{ $labels.service }} / {{ $labels.method }} / {{ $labels.operation }} resulted in HTTP error code 5xx"
		if mapping.ProviderName == "ALL" {
			error5xxRateAnnotations["summary"] = "Too many OpenStack API errors"
		} else {
			alertName += "_" + mapping.ProviderName
			error5xxRateAnnotations["summary"] = "Too many OpenStack API errors on cloud " + mapping.ProviderName
		}
		error5xxRateAlert := monitoring.MkPrometheusAlertRule(
			alertName,
			intstr.FromString(
				"sum(rate("+mapping.Name+"{status=~'5..'}[15m]))"+
					" / sum(rate("+mapping.Name+"{status=~'.*'}[15m])) * 100 > 5"),
			"15m",
			monitoring.CriticalSeverityLabel,
			error5xxRateAnnotations,
		)
		openstackAPIRules = append(openstackAPIRules, error5xxRateAlert)
	}

	launcherRuleGroup := monitoring.MkPrometheusRuleGroup(
		"launcher_default.rules",
		[]monitoringv1.Rule{
			highLaunchErrorRate,
			highFailedStateRate,
		})
	builderRuleGroup := monitoring.MkPrometheusRuleGroup(
		"builder_default.rules",
		[]monitoringv1.Rule{
			dibImageBuildFailure,
		})
	providersAPIRuleGroup := monitoring.MkPrometheusRuleGroup(
		"providersAPI_default.rules",
		openstackAPIRules)
	desiredNodepoolPromRule := monitoring.MkPrometheusRuleCR(nodepoolIdent+"-default.rules", r.ns)
	desiredNodepoolPromRule.Spec.Groups = append(
		desiredNodepoolPromRule.Spec.Groups,
		launcherRuleGroup,
		builderRuleGroup,
		providersAPIRuleGroup)

	var checksumable string
	for _, group := range desiredNodepoolPromRule.Spec.Groups {
		for _, rule := range group.Rules {
			checksumable += monitoring.MkAlertRuleChecksumString(rule)
		}
	}

	annotations := map[string]string{
		"version":       "1",
		"rulesChecksum": utils.Checksum([]byte(checksumable)),
	}
	desiredNodepoolPromRule.ObjectMeta.Annotations = annotations
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredNodepoolPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredNodepoolPromRule)
		return false
	} else {
		if !utils.MapEquals(&currentPromRule.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool default Prometheus rules changed, updating...")
			currentPromRule.Spec = desiredNodepoolPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

func (r *SFController) setProviderSecretsVolumeMounts(volumeMount []apiv1.VolumeMount) (apiv1.Secret, []apiv1.VolumeMount, bool) {
	var nodepoolProvidersSecrets apiv1.Secret
	if r.GetM(NodepoolProvidersSecretsName, &nodepoolProvidersSecrets) {
		if data, ok := nodepoolProvidersSecrets.Data["clouds.yaml"]; ok && len(data) > 0 {
			volumeMount = append(volumeMount, apiv1.VolumeMount{
				Name:      "nodepool-providers-secrets",
				SubPath:   "clouds.yaml",
				MountPath: "/var/lib/nodepool/.config/openstack/clouds.yaml",
				ReadOnly:  true,
			})
		}

		if data, ok := nodepoolProvidersSecrets.Data["kube.config"]; ok && len(data) > 0 {
			volumeMount = append(volumeMount, apiv1.VolumeMount{
				Name:      "nodepool-providers-secrets",
				SubPath:   "kube.config",
				MountPath: "/var/lib/nodepool/.kube/config",
				ReadOnly:  true,
			})
		}
		return nodepoolProvidersSecrets, volumeMount, true
	} else {
		return nodepoolProvidersSecrets, volumeMount, false
	}
}

func (r *SFController) DeployNodepoolBuilder(statsdExporterVolume apiv1.Volume, nodepoolStatsdMappingConfig string) bool {

	r.EnsureSSHKeySecret("nodepool-builder-ssh-key")

	r.setNodepoolTooling()

	loggingConfig, _ := mkLoggingTemplate(r.cr.Spec.Nodepool.Builder.LogLevel)

	builderExtraConfigData := make(map[string]string)
	builderExtraConfigData["logging.yaml"] = loggingConfig
	builderExtraConfigData["httpd-build-logs-dir.conf"] = httpdBuildLogsDirConfig
	r.EnsureConfigMap("nodepool-builder-extra-config", builderExtraConfigData)

	var mod int32 = 256 // decimal for 0400 octal
	// get statsd relay if defined
	var relayAddress *string
	if r.cr.Spec.Nodepool.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Nodepool.StatsdTarget
	}

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret(NodepoolProvidersSecretsName),
		base.MkEmptyDirVolume("nodepool-config"),
		r.commonToolingVolume(),
		{
			Name: "nodepool-builder-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "nodepool-builder-ssh-key",
					DefaultMode: &mod,
				},
			},
		},
		{
			Name: "zuul-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: "zuul-ssh-key",
					Items: []apiv1.KeyToPath{{
						Key:  "pub",
						Path: "pub",
					}},
					DefaultMode: &mod,
				},
			},
		},
		base.MkVolumeCM("nodepool-builder-extra-config-vol",
			"nodepool-builder-extra-config-config-map"),
		statsdExporterVolume,
	}

	volumeMount := []apiv1.VolumeMount{
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool",
		},
		{
			Name:      builderIdent,
			MountPath: "/var/lib/nodepool",
		},
		configScriptVolumeMount,
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "dib-ansible.py",
			MountPath: "/usr/local/bin/dib-ansible",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-builder-ssh-key",
			MountPath: "/var/lib/nodepool-ssh-key",
			ReadOnly:  true,
		},
		{
			Name:      "zuul-ssh-key",
			MountPath: "/var/lib/zuul-ssh-key",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "ssh_config",
			MountPath: "/var/lib/nodepool/.ssh/config",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-builder-extra-config-vol",
			SubPath:   "logging.yaml",
			MountPath: "/etc/nodepool-logging/logging.yaml",
			ReadOnly:  true,
		},
	}

	nodepoolProvidersSecrets, volumeMount, ready := r.setProviderSecretsVolumeMounts(volumeMount)
	if !ready {
		return false
	}

	annotations := map[string]string{
		"nodepool.yaml":          utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml":  utils.Checksum([]byte(loggingConfig)),
		"dib-ansible.py":         utils.Checksum([]byte(dibAnsibleWrapper)),
		"ssh_config":             utils.Checksum([]byte(builderSSHConfig)),
		"buildlogs_httpd_config": utils.Checksum([]byte(httpdBuildLogsDirConfig)),
		"statsd_mapping":         utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		// When the Secret ResourceVersion field change (when edited) we force a nodepool-builder restart
		"nodepool-providers-secrets": string(nodepoolProvidersSecrets.ResourceVersion),
		"serial":                     "10",
	}

	initContainer := base.MkContainer("nodepool-builder-init", base.BusyboxImage)

	initContainer.Command = []string{"bash", "-c", "mkdir -p ~/dib ~/builds; /usr/local/bin/generate-config.sh"}
	initContainer.Env = append(r.getNodepoolConfigEnvs(),
		base.MkEnvVar("NODEPOOL_CONFIG_FILE", "nodepool-builder.yaml"),
	)
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/",
		},
		{
			Name:      builderIdent,
			MountPath: "/var/lib/nodepool",
		},
		configScriptVolumeMount,
	}

	replicas := int32(1)
	nb := r.mkStatefulSet(
		builderIdent, base.NodepoolBuilderImage, r.getStorageConfOrDefault(r.cr.Spec.Nodepool.Builder.Storage),
		replicas, apiv1.ReadWriteOnce)

	nb.Spec.Template.ObjectMeta.Annotations = annotations
	nb.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	nb.Spec.Template.Spec.Volumes = volumes
	nb.Spec.Template.Spec.Containers[0].Command = []string{"/usr/local/bin/dumb-init", "--",
		"/usr/local/bin/nodepool-builder", "-f", "-l", "/etc/nodepool-logging/logging.yaml"}
	nb.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMount
	nb.Spec.Template.Spec.Containers[0].Env = r.getNodepoolConfigEnvs()

	// Append statsd exporter sidecar
	nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers,
		monitoring.MkStatsdExporterSideCarContainer(shortIdent, "statsd-config", relayAddress),
	)

	// Append image build logs HTTPD sidecar
	buildLogsContainer := base.MkContainer("build-logs-httpd", HTTPDImage)
	buildLogsContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      builderIdent,
			SubPath:   "builds",
			MountPath: "/var/www/html/builds",
		},
		{
			Name:      "nodepool-builder-extra-config-vol",
			SubPath:   "httpd-build-logs-dir.conf",
			MountPath: "/etc/httpd/conf.d/build-logs-dir.conf",
		},
	}
	buildLogsContainer.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(buildLogsHttpdPort, buildLogsHttpdPortName),
	}
	buildLogsContainer.ReadinessProbe = base.MkReadinessHTTPProbe("/builds", buildLogsHttpdPort)
	buildLogsContainer.StartupProbe = base.MkStartupHTTPProbe("/builds", buildLogsHttpdPort)
	buildLogsContainer.LivenessProbe = base.MkLiveHTTPProbe("/builds", buildLogsHttpdPort)
	nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers,
		buildLogsContainer,
	)

	httpdService := base.MkService(
		buildLogsHttpdPortName, r.ns, builderIdent, []int32{buildLogsHttpdPort}, buildLogsHttpdPortName)
	r.GetOrCreate(&httpdService)

	current := appsv1.StatefulSet{}
	if r.GetM(builderIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool-builder configuration changed, rollout pods ...")
			current.Spec = nb.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := nb
		r.CreateR(&current)
	}

	pvcReadiness := r.reconcileExpandPVC(builderIdent+"-"+builderIdent+"-0", r.cr.Spec.Nodepool.Builder.Storage)

	routeReady := r.ensureHTTPSRoute(r.cr.Name+"-nodepool-builder", "nodepool", buildLogsHttpdPortName, "/builds",
		buildLogsHttpdPort, map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	var isReady = r.IsStatefulSetReady(&current) && routeReady && pvcReadiness

	conds.UpdateConditions(&r.cr.Status.Conditions, builderIdent, isReady)

	return isReady
}

func (r *SFController) DeployNodepoolLauncher(statsdExporterVolume apiv1.Volume, nodepoolProvidersSecrets apiv1.Secret, nodepoolStatsdMappingConfig string) bool {

	r.setNodepoolTooling()

	loggingConfig, _ := mkLoggingTemplate(r.cr.Spec.Nodepool.Launcher.LogLevel)

	// get statsd relay if defined
	var relayAddress *string
	if r.cr.Spec.Nodepool.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Nodepool.StatsdTarget
	}

	launcherExtraConfigData := make(map[string]string)
	launcherExtraConfigData["logging.yaml"] = loggingConfig
	r.EnsureConfigMap("nodepool-launcher-extra-config", launcherExtraConfigData)

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret(NodepoolProvidersSecretsName),
		base.MkEmptyDirVolume("nodepool-config"),
		base.MkEmptyDirVolume("nodepool-home"),
		r.commonToolingVolume(),
		base.MkVolumeCM("nodepool-launcher-extra-config-vol",
			"nodepool-launcher-extra-config-config-map"),
		statsdExporterVolume,
	}

	volumeMount := []apiv1.VolumeMount{
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/",
		},
		{
			Name:      "nodepool-home",
			MountPath: "/var/lib/nodepool",
		},
		{
			Name:      "nodepool-launcher-extra-config-vol",
			SubPath:   "logging.yaml",
			MountPath: "/etc/nodepool-logging/logging.yaml",
		},
		configScriptVolumeMount,
	}

	nodepoolProvidersSecrets, volumeMount, ready := r.setProviderSecretsVolumeMounts(volumeMount)
	if !ready {
		return false
	}

	annotations := map[string]string{
		"nodepool.yaml":         utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml": utils.Checksum([]byte(loggingConfig)),
		"statsd_mapping":        utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		"serial":                "6",
		// When the Secret ResourceVersion field change (when edited) we force a nodepool-launcher restart
		"nodepool-providers-secrets": string(nodepoolProvidersSecrets.ResourceVersion),
		"nodepool-launcher-image":    base.NodepoolLauncherImage,
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigLocation.BaseURL + r.cr.Spec.ConfigLocation.Name
	}

	nl := base.MkDeployment("nodepool-launcher", r.ns, "")

	container := base.MkContainer("launcher", base.NodepoolLauncherImage)
	container.VolumeMounts = volumeMount
	container.Command = []string{"/usr/local/bin/dumb-init", "--",
		"/usr/local/bin/nodepool-launcher", "-f", "-l", "/etc/nodepool-logging/logging.yaml"}
	container.Env = r.getNodepoolConfigEnvs()

	initContainer := base.MkContainer("nodepool-launcher-init", base.BusyboxImage)

	initContainer.Command = []string{"/usr/local/bin/generate-config.sh"}
	initContainer.Env = r.getNodepoolConfigEnvs()
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/",
		},
		{
			Name:      "nodepool-home",
			MountPath: "/var/lib/nodepool",
		},
		configScriptVolumeMount,
	}

	nl.Spec.Template.Spec.Volumes = volumes
	nl.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	nl.Spec.Template.Spec.Containers = []apiv1.Container{
		container,
		monitoring.MkStatsdExporterSideCarContainer(shortIdent, "statsd-config", relayAddress)}
	nl.Spec.Template.ObjectMeta.Annotations = annotations
	nl.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(launcherPort, launcherPortName),
	}

	current := appsv1.Deployment{}
	if r.GetM(launcherIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool-launcher configuration changed, rollout pods ...")
			current.Spec = nl.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := nl
		r.CreateR(&current)
	}

	srv := base.MkService(launcherIdent, r.ns, launcherIdent, []int32{launcherPort}, launcherIdent)
	r.GetOrCreate(&srv)

	routeReady := r.ensureHTTPSRoute(r.cr.Name+"-nodepool-launcher", "nodepool", launcherIdent, "/",
		launcherPort, map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	isDeploymentReady := r.IsDeploymentReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, launcherIdent, isDeploymentReady)

	return isDeploymentReady && routeReady
}

func (r *SFController) DeployNodepool() map[string]bool {

	deployments := make(map[string]bool)

	// We need to initialize the providers secrets early
	var v []apiv1.VolumeMount
	var nodepoolProvidersSecrets, _, ready = r.setProviderSecretsVolumeMounts(v)
	if !ready {
		deployments[launcherIdent] = false
		deployments[builderIdent] = false
		return deployments
	}

	cloudsData, ok := nodepoolProvidersSecrets.Data["clouds.yaml"]
	var cloudsYaml = make(map[string]interface{})
	if ok && len(cloudsData) > 0 {
		yaml.Unmarshal(cloudsData, &cloudsYaml)
	}
	nodepoolStatsdMappingConfig, _ := mkStatsdMappingConfig(cloudsYaml)

	// create statsd exporter config map
	r.EnsureConfigMap("np-statsd", map[string]string{
		monitoring.StatsdExporterConfigFile: nodepoolStatsdMappingConfig,
	})
	statsdVolume := base.MkVolumeCM("statsd-config", "np-statsd-config-map")

	// Ensure monitoring - TODO add to condition
	r.EnsureNodepoolPodMonitor()
	r.ensureNodepoolPromRule(cloudsYaml)

	deployments[launcherIdent] = r.DeployNodepoolLauncher(statsdVolume, nodepoolProvidersSecrets, nodepoolStatsdMappingConfig)
	deployments[builderIdent] = r.DeployNodepoolBuilder(statsdVolume, nodepoolStatsdMappingConfig)
	return deployments
}
