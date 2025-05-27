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
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
)

//go:embed static/nodepool/init-container.sh
var initContainerScript string

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

//go:embed static/nodepool/fluentbit/parsers.conf
var fluentBitForwarderParsersConfig string

//go:embed static/nodepool/fluentbit/fluent-bit.conf.tmpl
var fluentBitForwarderConfig string

// ansible.cfg and the timestamp callback could be hard-coded in the nodepool builder container.
//
//go:embed static/nodepool/ansible/ansible.cfg
var ansibleConfiguration string

//go:embed static/nodepool/ansible/timestamp.py
var timestampOutputCallback string

const (
	nodepoolIdent                = "nodepool"
	LauncherIdent                = nodepoolIdent + "-launcher"
	shortIdent                   = "np"
	launcherPortName             = "nlwebapp"
	launcherPort                 = 8006
	buildLogsHttpdPort           = 8080
	BuildLogsHttpdPortName       = "buildlogs-http"
	NodepoolProvidersSecretsName = "nodepool-providers-secrets"
	BuilderIdent                 = nodepoolIdent + "-builder"
)

var NodepoolStatsdExporterPortName = monitoring.GetStatsdExporterPort(shortIdent)

var configScriptsVolumeMounts = []apiv1.VolumeMount{
	{
		Name:      "nodepool-tooling-vol",
		SubPath:   "generate-config.sh",
		MountPath: "/usr/local/bin/generate-config.sh",
		ReadOnly:  true,
	},
	{
		Name:      "nodepool-tooling-vol",
		SubPath:   "fetch-config-repo.sh",
		MountPath: "/usr/local/bin/fetch-config-repo.sh",
		ReadOnly:  true,
	},
}

var nodepoolFluentBitLabels = []logging.FluentBitLabel{
	{
		Key:   "COMPONENT",
		Value: "nodepool",
	},
}

func createImageBuildLogForwarderSidecar(r *SFController, annotations map[string]string) ([]apiv1.Volume, apiv1.Container) {
	fbForwarderConfig := make(map[string]string)
	var loggingParams = logging.CreateForwarderConfigTemplateParams("diskimage-builder", r.cr.Spec.FluentBitLogForwarding)

	fbForwarderConfig["fluent-bit.conf"], _ = utils.ParseString(
		fluentBitForwarderConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{[]logging.FluentBitLabel{}, loggingParams})
	fbForwarderConfig["parsers.conf"] = fluentBitForwarderParsersConfig
	r.EnsureConfigMap("fluentbit-dib-cfg", fbForwarderConfig)

	volume := base.MkVolumeCM("dib-log-forwarder-config",
		"fluentbit-dib-cfg-config-map")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      BuilderIdent,
			SubPath:   "builds",
			MountPath: "/watch/",
		},
		{
			Name:      "dib-log-forwarder-config",
			MountPath: "/fluent-bit/etc/",
		},
	}
	builderFluentBitLabels := append(nodepoolFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: BuilderIdent})
	sidecar, storageEmptyDir := logging.CreateFluentBitSideCarContainer("diskimage-builder", builderFluentBitLabels, volumeMounts, r.isOpenShift)
	annotations["dib-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	annotations["dib-fluent-bit-parser"] = utils.Checksum([]byte(fbForwarderConfig["parsers.conf"]))
	annotations["dib-fluent-bit-image"] = sidecar.Image
	return []apiv1.Volume{volume, storageEmptyDir}, sidecar

}

func (r *SFController) setNodepoolTooling() {
	toolingData := make(map[string]string)
	toolingData["init-container.sh"] = initContainerScript
	toolingData["generate-config.sh"] = generateConfigScript
	toolingData["fetch-config-repo.sh"] = fetchConfigRepoScript
	toolingData["dib-ansible.py"] = dibAnsibleWrapper
	toolingData["ssh_config"] = builderSSHConfig
	toolingData["timestamp.py"] = timestampOutputCallback
	toolingData["ansible.cfg"] = ansibleConfiguration
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
			base.MkEnvVar("CONFIG_REPO_BASE_URL", r.configBaseURL),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
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

func (r *SFController) mkLoggingTemplate(serviceName string) (string, error) {
	// Unfortunatly I'm unable to leverage default value set at API validation
	selectedLogLevel := v1.InfoLogLevel
	var logLevel v1.LogLevel
	if serviceName == "builder" {
		logLevel = r.cr.Spec.Nodepool.Builder.LogLevel
	} else {
		logLevel = r.cr.Spec.Nodepool.Launcher.LogLevel
	}
	if logLevel != "" {
		selectedLogLevel = logLevel
	}

	var loggingParams = logging.CreateForwarderConfigTemplateParams("nodepool."+serviceName, r.cr.Spec.FluentBitLogForwarding)
	var loggingExtraKeys = logging.CreateBaseLoggingExtraKeys("nodepool-"+serviceName, "nodepool", serviceName, r.ns)
	// Change logLevel to what we actually want
	loggingParams.LogLevel = string(selectedLogLevel)

	loggingConfig, err := utils.ParseString(
		loggingConfigTemplate,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{loggingExtraKeys, loggingParams})

	return loggingConfig, err
}

func mkStatsdMappingConfig(cloudsYaml map[string]interface{}) (string, error) {
	var extraMappings []monitoring.StatsdMetricMapping

	extraMappings = monitoring.MkStatsdMappingsFromCloudsYaml(extraMappings, cloudsYaml)

	statsdMappingConfig, err := utils.ParseString(
		nodepoolStatsdMappingConfigTemplate, extraMappings)
	return statsdMappingConfig, err
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
			logging.LogI("Nodepool default Prometheus rules changed, updating...")
			currentPromRule.Spec = desiredNodepoolPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

func (r *SFController) setProviderSecretsVolumeMounts() ([]apiv1.VolumeMount, apiv1.Secret, bool) {
	var (
		nodepoolProvidersSecrets apiv1.Secret
		volumeMount              []apiv1.VolumeMount
	)
	exists := r.GetM(NodepoolProvidersSecretsName, &nodepoolProvidersSecrets)
	if exists {
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
	}
	return volumeMount, nodepoolProvidersSecrets, exists
}

func getSecretsVersion(secret apiv1.Secret, secretExists bool) string {
	secretVersion := "0"
	if secretExists {
		secretVersion = string(secret.ResourceVersion)
	}
	return secretVersion
}

func getCMVersion(cm apiv1.ConfigMap, cmExists bool) string {
	cmVersion := "0"
	if cmExists {
		cmVersion = string(cm.ResourceVersion)
	}
	return cmVersion
}

func (r *SFController) DeployNodepoolBuilder(statsdExporterVolume apiv1.Volume, nodepoolStatsdMappingConfig string,
	initialVolumeMounts []apiv1.VolumeMount, providersSecrets apiv1.Secret, providerSecretsExists bool) bool {

	r.EnsureSSHKeySecret("nodepool-builder-ssh-key")

	r.setNodepoolTooling()

	loggingConfig, _ := r.mkLoggingTemplate("builder")

	builderExtraConfigData := make(map[string]string)
	builderExtraConfigData["logging.yaml"] = loggingConfig
	builderExtraConfigData["httpd-build-logs-dir.conf"] = httpdBuildLogsDirConfig
	r.EnsureConfigMap("nodepool-builder-extra-config", builderExtraConfigData)

	// get statsd relay if defined
	var relayAddress *string
	if r.cr.Spec.Nodepool.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Nodepool.StatsdTarget
	}

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret(NodepoolProvidersSecretsName),
		base.MkEmptyDirVolume("nodepool-config"),
		base.MkEmptyDirVolume("nodepool-ca"),
		r.commonToolingVolume(),
		{
			Name: "nodepool-builder-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "nodepool-builder-ssh-key",
					DefaultMode: &utils.Readmod,
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
					DefaultMode: &utils.Readmod,
				},
			},
		},
		base.MkVolumeCM("nodepool-builder-extra-config-vol",
			"nodepool-builder-extra-config-config-map"),
		statsdExporterVolume,
	}

	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	// Create the corporate CM based Volume when the Corporate CM exists
	if corporateCMExists {
		volumes = append(volumes, base.MkVolumeCM("nodepool-builder-corporate-ca-certs", CorporateCACerts))
	}

	nodeExporterVolumeMount := []apiv1.VolumeMount{
		{
			Name:      BuilderIdent,
			MountPath: "/var/lib/nodepool",
		},
	}

	volumeMounts := append(initialVolumeMounts, []apiv1.VolumeMount{
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
			Name:      "nodepool-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
		},
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
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "ansible.cfg",
			MountPath: "/etc/ansible/ansible.cfg",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "timestamp.py",
			MountPath: "/usr/share/ansible/plugins/callback/timestamp.py",
			ReadOnly:  true,
		},
	}...)

	volumeMounts = append(volumeMounts, configScriptsVolumeMounts...)
	volumeMounts = append(volumeMounts, nodeExporterVolumeMount...)

	annotations := map[string]string{
		"nodepool.yaml":              utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml":      utils.Checksum([]byte(loggingConfig)),
		"dib-ansible.py":             utils.Checksum([]byte(dibAnsibleWrapper)),
		"ssh_config":                 utils.Checksum([]byte(builderSSHConfig)),
		"buildlogs_httpd_config":     utils.Checksum([]byte(httpdBuildLogsDirConfig)),
		"statsd_mapping":             utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		"image":                      base.NodepoolBuilderImage(),
		"nodepool-providers-secrets": getSecretsVersion(providersSecrets, providerSecretsExists),
		"serial":                     "18",
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.configBaseURL + r.cr.Spec.ConfigRepositoryLocation.Name
	}

	initContainer := base.MkContainer("nodepool-builder-init", base.NodepoolBuilderImage(), r.isOpenShift)
	base.SetContainerLimitsLowProfile(&initContainer)

	initContainer.Command = []string{"/usr/local/bin/init-container.sh"}
	initContainer.Env = append(r.getNodepoolConfigEnvs(),
		base.MkEnvVar("NODEPOOL_CONFIG_FILE", "nodepool-builder.yaml"),
	)
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "init-container.sh",
			MountPath: "/usr/local/bin/init-container.sh",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/",
		},
		{
			Name:      "nodepool-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
		},
	}

	initContainer.VolumeMounts = append(initContainer.VolumeMounts, configScriptsVolumeMounts...)
	initContainer.VolumeMounts = append(initContainer.VolumeMounts, nodeExporterVolumeMount...)

	if corporateCMExists {
		initContainer.VolumeMounts = AppendCorporateCACertsVolumeMount(initContainer.VolumeMounts, "nodepool-builder-corporate-ca-certs")
	}

	nb := r.mkStatefulSet(
		BuilderIdent, base.NodepoolBuilderImage(), r.getStorageConfOrDefault(r.cr.Spec.Nodepool.Builder.Storage),
		apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels, r.isOpenShift)

	nb.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	nb.Spec.Template.Spec.Volumes = volumes
	nb.Spec.Template.Spec.Containers[0].Command = []string{
		"/usr/local/bin/dumb-init", "--", "bash", "-c",
		"/usr/local/bin/nodepool-builder -f -l /etc/nodepool-logging/logging.yaml",
	}
	nb.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	nb.Spec.Template.Spec.Containers[0].Env = r.getNodepoolConfigEnvs()

	base.SetContainerLimitsHighProfile(&nb.Spec.Template.Spec.Containers[0])
	limitstr := base.UpdateContainerLimit(r.cr.Spec.Nodepool.Builder.Limits, &nb.Spec.Template.Spec.Containers[0])
	annotations["limits"] = limitstr

	extraLoggingEnvVars := logging.SetupLogForwarding("nodepool-builder", r.cr.Spec.FluentBitLogForwarding, nodepoolFluentBitLabels, annotations)
	nb.Spec.Template.Spec.Containers[0].Env = append(nb.Spec.Template.Spec.Containers[0].Env, extraLoggingEnvVars...)
	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolumes, fbSidecar := createImageBuildLogForwarderSidecar(r, annotations)
		nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers, fbSidecar)
		nb.Spec.Template.Spec.Volumes = append(nb.Spec.Template.Spec.Volumes, fbVolumes...)
	}

	nb.Spec.Template.ObjectMeta.Annotations = annotations

	// Append statsd exporter sidecar
	nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers,
		monitoring.MkStatsdExporterSideCarContainer(shortIdent, "statsd-config", relayAddress, r.isOpenShift),
	)

	diskUsageExporter := monitoring.MkNodeExporterSideCarContainer(BuilderIdent, nodeExporterVolumeMount, r.isOpenShift)
	nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers, diskUsageExporter)

	// Append image build logs HTTPD sidecar
	buildLogsContainer := base.MkContainer("build-logs-httpd", base.HTTPDImage(), r.isOpenShift)
	buildLogsContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      BuilderIdent,
			SubPath:   "builds",
			MountPath: "/var/www/html/nodepool/builds",
		},
		{
			Name:      "nodepool-builder-extra-config-vol",
			SubPath:   "httpd-build-logs-dir.conf",
			MountPath: "/etc/httpd/conf.d/build-logs-dir.conf",
		},
	}
	buildLogsContainer.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(buildLogsHttpdPort, BuildLogsHttpdPortName),
	}
	buildLogsContainer.ReadinessProbe = base.MkReadinessHTTPProbe("/nodepool/builds", buildLogsHttpdPort)
	buildLogsContainer.StartupProbe = base.MkStartupHTTPProbe("/nodepool/builds", buildLogsHttpdPort)
	buildLogsContainer.LivenessProbe = base.MkLiveHTTPProbe("/nodepool/builds", buildLogsHttpdPort)
	nb.Spec.Template.Spec.Containers = append(nb.Spec.Template.Spec.Containers,
		buildLogsContainer,
	)
	nb.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	svc := base.MkServicePod(
		BuilderIdent, r.ns, BuilderIdent+"-0", []int32{buildLogsHttpdPort}, BuilderIdent, r.cr.Spec.ExtraLabels)
	r.EnsureService(&svc)

	current, changed := r.ensureStatefulset(nb)
	if changed {
		return false
	}

	pvcReadiness := r.reconcileExpandPVC(BuilderIdent+"-"+BuilderIdent+"-0", r.cr.Spec.Nodepool.Builder.Storage)

	var isReady = r.IsStatefulSetReady(current) && pvcReadiness

	conds.UpdateConditions(&r.cr.Status.Conditions, BuilderIdent, isReady)

	return isReady
}

func hasProviderSecret(volumeMounts []apiv1.VolumeMount) bool {
	for _, volume := range volumeMounts {
		if volume.Name == "nodepool-providers-secrets" {
			return true
		}
	}
	return false
}

func (r *SFController) DeployNodepoolLauncher(statsdExporterVolume apiv1.Volume, nodepoolStatsdMappingConfig string,
	initialVolumeMounts []apiv1.VolumeMount, providersSecrets apiv1.Secret, providerSecretsExists bool) bool {

	r.setNodepoolTooling()

	loggingConfig, _ := r.mkLoggingTemplate("launcher")

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
		base.MkEmptyDirVolume("nodepool-ca"),
		r.commonToolingVolume(),
		base.MkVolumeCM("nodepool-launcher-extra-config-vol",
			"nodepool-launcher-extra-config-config-map"),
		statsdExporterVolume,
	}
	// Check if Corporate Certificate exists
	corporateCM, corporateCMExists := r.CorporateCAConfigMapExists()

	if corporateCMExists {
		volumes = append(volumes, base.MkVolumeCM("nodepool-launcher-corporate-ca-certs", CorporateCACerts))
	}

	volumeMounts := append(initialVolumeMounts, []apiv1.VolumeMount{
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
			Name:      "nodepool-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
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
	}...)

	volumeMounts = append(volumeMounts, configScriptsVolumeMounts...)

	annotations := map[string]string{
		"nodepool.yaml":         utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml": utils.Checksum([]byte(loggingConfig)),
		"statsd_mapping":        utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		"serial":                "12",
		// When the Secret ResourceVersion field change (when edited) we force a nodepool-launcher restart
		"image":                      base.NodepoolLauncherImage(),
		"nodepool-providers-secrets": getSecretsVersion(providersSecrets, providerSecretsExists),
		"corporate-ca-certs-version": getCMVersion(corporateCM, corporateCMExists),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.configBaseURL + r.cr.Spec.ConfigRepositoryLocation.Name
	}

	initContainer := base.MkContainer("nodepool-launcher-init", base.NodepoolLauncherImage(), r.isOpenShift)
	base.SetContainerLimitsLowProfile(&initContainer)

	initContainer.Command = []string{"/usr/local/bin/init-container.sh"}
	initContainer.Env = r.getNodepoolConfigEnvs()
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "init-container.sh",
			MountPath: "/usr/local/bin/init-container.sh",
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
			Name:      "nodepool-ca",
			MountPath: "/etc/pki/ca-trust/extracted",
		},
	}

	initContainer.VolumeMounts = append(initContainer.VolumeMounts, configScriptsVolumeMounts...)

	if corporateCMExists {
		initContainer.VolumeMounts = AppendCorporateCACertsVolumeMount(initContainer.VolumeMounts, "nodepool-launcher-corporate-ca-certs")
	}

	nl := base.MkDeployment("nodepool-launcher", r.ns, "", r.cr.Spec.ExtraLabels, r.isOpenShift)

	container := base.MkContainer("launcher", base.NodepoolLauncherImage(), r.isOpenShift)
	container.VolumeMounts = volumeMounts
	container.Command = []string{
		"/usr/local/bin/dumb-init", "--", "bash", "-c",
		"/usr/local/bin/nodepool-launcher -f -l /etc/nodepool-logging/logging.yaml",
	}
	container.Env = r.getNodepoolConfigEnvs()
	base.SetContainerLimitsHighProfile(&container)
	limitstr := base.UpdateContainerLimit(r.cr.Spec.Nodepool.Launcher.Limits, &container)
	annotations["limits"] = limitstr

	launcherFluentBitLabels := append(nodepoolFluentBitLabels, logging.FluentBitLabel{Key: "CONTAINER", Value: LauncherIdent})
	extraLoggingEnvVars := logging.SetupLogForwarding("nodepool-launcher", r.cr.Spec.FluentBitLogForwarding, launcherFluentBitLabels, annotations)
	container.Env = append(container.Env, extraLoggingEnvVars...)

	nl.Spec.Template.Spec.Volumes = volumes
	nl.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	nl.Spec.Template.Spec.Containers = []apiv1.Container{
		container,
		monitoring.MkStatsdExporterSideCarContainer(shortIdent, "statsd-config", relayAddress, r.isOpenShift)}
	nl.Spec.Template.ObjectMeta.Annotations = annotations
	nl.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/ready", launcherPort)
	nl.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(launcherPort, launcherPortName),
	}

	if hasProviderSecret(initialVolumeMounts) {
		// Append zuul-capacity sidecar
		nl.Spec.Template.Spec.Containers = append(nl.Spec.Template.Spec.Containers,
			MkZuulCapacityContainer(r.isOpenShift),
		)

		// Setup zuul-capacity service
		zcSrv := base.MkService("zuul-capacity", r.ns, "nodepool-launcher", []int32{9100}, "zuul-capacity", r.cr.Spec.ExtraLabels)
		r.GetOrCreate(&zcSrv)
	}
	nl.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	current := appsv1.Deployment{}
	if r.GetM(LauncherIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			logging.LogI("Nodepool-launcher configuration changed, rollout pods ...")
			current.Spec = nl.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := nl
		r.CreateR(&current)
	}

	srv := base.MkService(LauncherIdent, r.ns, LauncherIdent, []int32{launcherPort}, LauncherIdent, r.cr.Spec.ExtraLabels)
	r.GetOrCreate(&srv)

	isDeploymentReady := r.IsDeploymentReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, LauncherIdent, isDeploymentReady)

	return isDeploymentReady
}

func (r *SFController) DeployNodepool() map[string]bool {

	deployments := make(map[string]bool)

	// We need to initialize the providers secrets early
	var volumeMounts, nodepoolProvidersSecrets, providerSecretsResourceExists = r.setProviderSecretsVolumeMounts()

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
	if !r.cr.Spec.PrometheusMonitorsDisabled {
		r.ensureNodepoolPromRule(cloudsYaml)
	}

	deployments[LauncherIdent] = r.DeployNodepoolLauncher(
		statsdVolume, nodepoolStatsdMappingConfig, volumeMounts, nodepoolProvidersSecrets, providerSecretsResourceExists)
	deployments[BuilderIdent] = r.DeployNodepoolBuilder(statsdVolume, nodepoolStatsdMappingConfig,
		volumeMounts, nodepoolProvidersSecrets, providerSecretsResourceExists)
	return deployments
}
