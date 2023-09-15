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
)

//go:embed static/nodepool/generate-config.sh
var generateConfigScript string

//go:embed static/nodepool/logging.yaml.tmpl
var loggingConfigTemplate string

//go:embed static/nodepool/dib-ansible.py
var dibAnsibleWrapper string

//go:embed static/nodepool/ssh_config
var builderSSHConfig string

//go:embed static/nodepool/statsd_mapping.yaml
var nodepoolStatsdMappingConfig string

const nodepoolIdent = "nodepool"
const LauncherIdent = nodepoolIdent + "-launcher"
const shortIdent = "np"
const launcherPortName = "nlwebapp"
const launcherPort = 8006
const NodepoolProvidersSecretsName = "nodepool-providers-secrets"
const nodepoolLauncherImage = "quay.io/software-factory/" + LauncherIdent + ":9.0.0-3"

const BuilderIdent = nodepoolIdent + "-builder"
const nodepoolBuilderImage = "quay.io/software-factory/" + BuilderIdent + ":9.0.0-3"

var nodepoolStatsdExporterPortName = monitoring.GetStatsdExporterPort(shortIdent)

var configScriptVolumeMount = apiv1.VolumeMount{
	Name:      "nodepool-tooling-vol",
	SubPath:   "generate-config.sh",
	MountPath: "/usr/local/bin/generate-config.sh",
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

func (r *SFController) EnsureNodepoolPodMonitor() bool {
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "run",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{LauncherIdent, BuilderIdent},
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

func (r *SFController) DeployNodepoolBuilder(statsdExporterVolume apiv1.Volume) bool {

	r.EnsureSSHKeySecret("nodepool-builder-ssh-key")

	r.setNodepoolTooling()

	loggingConfig, _ := mkLoggingTemplate(r.cr.Spec.Nodepool.Builder.LogLevel)

	builderExtraConfigData := make(map[string]string)
	builderExtraConfigData["logging.yaml"] = loggingConfig
	r.EnsureConfigMap("nodepool-builder-extra-config", builderExtraConfigData)

	var mod int32 = 256 // decimal for 0400 octal
	// get statsd relay if defined
	var relayAddress *string
	if r.cr.Spec.Nodepool.StatsdTarget != "" {
		relayAddress = &r.cr.Spec.Nodepool.StatsdTarget
	}

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkEmptyDirVolume("nodepool-config"),
		base.MkEmptyDirVolume("nodepool-home-ssh"),
		base.MkEmptyDirVolume("nodepool-log"),
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
			Name:      BuilderIdent,
			MountPath: "/var/lib/nodepool",
		},
		{
			Name:      "nodepool-log",
			MountPath: "/var/log/nodepool",
		},
		configScriptVolumeMount,
		{
			Name:      "nodepool-tooling-vol",
			SubPath:   "dib-ansible.py",
			MountPath: "/usr/local/bin/dib-ansible",
		},
		{
			Name:      "nodepool-builder-ssh-key",
			MountPath: "/var/lib/nodepool-ssh-key",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-home-ssh",
			MountPath: "/var/lib/nodepool/.ssh",
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
		},
	}

	annotations := map[string]string{
		"nodepool.yaml":         utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml": utils.Checksum([]byte(loggingConfig)),
		"dib-ansible.py":        utils.Checksum([]byte(dibAnsibleWrapper)),
		"ssh_config":            utils.Checksum([]byte(builderSSHConfig)),
		"statsd_mapping":        utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		"serial":                "7",
	}

	initContainer := base.MkContainer("nodepool-builder-init", BusyboxImage)

	initContainer.Command = []string{"bash", "-c", "mkdir -p ~/dib; /usr/local/bin/generate-config.sh"}
	initContainer.Env = append(r.getNodepoolConfigEnvs(),
		base.MkEnvVar("NODEPOOL_CONFIG_FILE", "nodepool-builder.yaml"),
	)
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "nodepool-config",
			MountPath: "/etc/nodepool/",
		},
		{
			Name:      BuilderIdent,
			MountPath: "/var/lib/nodepool",
		},
		configScriptVolumeMount,
	}

	replicas := int32(1)
	nb := r.mkStatefulSet(
		BuilderIdent, nodepoolBuilderImage, r.getStorageConfOrDefault(r.cr.Spec.Nodepool.Builder.Storage),
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

	current := appsv1.StatefulSet{}
	if r.GetM(BuilderIdent, &current) {
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

	var isReady = r.IsStatefulSetReady(&current)

	conds.UpdateConditions(&r.cr.Status.Conditions, BuilderIdent, isReady)

	return isReady
}

func (r *SFController) DeployNodepoolLauncher(statsdExporterVolume apiv1.Volume) bool {

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

	// We set a place holder secret to ensure that the Secret is owned by the SoftwareFactory instance (ControllerReference)
	var nodepoolProvidersSecrets apiv1.Secret
	if !r.GetM(NodepoolProvidersSecretsName, &nodepoolProvidersSecrets) {
		r.CreateR(&apiv1.Secret{
			Data:       map[string][]byte{},
			ObjectMeta: metav1.ObjectMeta{Name: NodepoolProvidersSecretsName, Namespace: r.ns}})
	} else {
		if len(nodepoolProvidersSecrets.GetOwnerReferences()) == 0 {
			r.log.V(1).Info("Adopting the providers secret to set the owner reference", "secret", NodepoolProvidersSecretsName)
			if !r.UpdateR(&nodepoolProvidersSecrets) {
				return false
			}
		}
	}

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

	annotations := map[string]string{
		"nodepool.yaml":         utils.Checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml": utils.Checksum([]byte(loggingConfig)),
		"statsd_mapping":        utils.Checksum([]byte(nodepoolStatsdMappingConfig)),
		"serial":                "6",
		// When the Secret ResourceVersion field change (when edited) we force a nodepool-launcher restart
		"nodepool-providers-secrets": string(nodepoolProvidersSecrets.ResourceVersion),
		"nodepool-launcher-image":    nodepoolLauncherImage,
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigLocation.BaseURL + r.cr.Spec.ConfigLocation.Name
	}

	nl := base.MkDeployment("nodepool-launcher", r.ns, "")

	container := base.MkContainer("launcher", nodepoolLauncherImage)
	container.VolumeMounts = volumeMount
	container.Command = []string{"/usr/local/bin/dumb-init", "--",
		"/usr/local/bin/nodepool-launcher", "-f", "-l", "/etc/nodepool-logging/logging.yaml"}
	container.Env = r.getNodepoolConfigEnvs()

	initContainer := base.MkContainer("nodepool-launcher-init", BusyboxImage)

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
	if r.GetM(LauncherIdent, &current) {
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

	srv := base.MkService(LauncherIdent, r.ns, LauncherIdent, []int32{launcherPort}, LauncherIdent)
	r.GetOrCreate(&srv)

	routeReady := r.ensureHTTPSRoute(r.cr.Name+"-nodepool-launcher", "nodepool", LauncherIdent, "/",
		launcherPort, map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	isDeploymentReady := r.IsDeploymentReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, LauncherIdent, isDeploymentReady)

	return isDeploymentReady && routeReady
}

func (r *SFController) DeployNodepool() map[string]bool {

	// create statsd exporter config map
	r.EnsureConfigMap("np-statsd", map[string]string{
		monitoring.StatsdExporterConfigFile: nodepoolStatsdMappingConfig,
	})
	statsdVolume := base.MkVolumeCM("statsd-config", "np-statsd-config-map")

	// Ensure monitoring
	r.EnsureNodepoolPodMonitor()

	deployments := make(map[string]bool)
	deployments[LauncherIdent] = r.DeployNodepoolLauncher(statsdVolume)
	deployments[BuilderIdent] = r.DeployNodepoolBuilder(statsdVolume)
	return deployments
}
