// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package controllers provides controller functions
package controllers

import (
	_ "embed"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/nodepool/generate-launcher-config.sh
var generateConfigScript string

//go:embed static/nodepool/logging.yaml.tmpl
var loggingConfigTemplate string

//go:embed static/nodepool/builder.yaml
var builderConfigFile string

const launcherIdent = "nodepool-launcher"
const launcherPortName = "nlwebapp"
const launcherPort = 8006
const NodepoolProvidersSecretsName = "nodepool-providers-secrets"
const nodepoolLauncherImage = "quay.io/software-factory/" + launcherIdent + ":9.0.0-1"

const builderIdent = "nodepool-builder"
const nodepoolBuilderImage = "quay.io/software-factory/" + builderIdent + ":9.0.0-1"

var configScriptVolumeMount = apiv1.VolumeMount{
	Name:      "nodepool-launcher-tooling-vol",
	SubPath:   "generate-launcher-config.sh",
	MountPath: "/usr/local/bin/generate-launcher-config.sh",
}

func (r *SFController) getNodepoolConfigEnvs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "TRUE"),
			base.MkEnvVar("CONFIG_REPO_BASE_URL", r.cr.Spec.ConfigLocation.BaseURL),
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
		}
	} else {
		return []apiv1.EnvVar{
			base.MkEnvVar("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) DeployNodepoolBuilder() bool {

	builderConfig := make(map[string]string)
	builderConfig["nodepool.yaml"] = builderConfigFile

	r.EnsureConfigMap("nodepool-builder-config", builderConfig)

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkEmptyDirVolume("nodepool-config"),
		base.MkEmptyDirVolume("nodepool-home"),
		base.MkEmptyDirVolume("nodepool-home-dib"),
		base.MkEmptyDirVolume("nodepool-home-build"),
		base.MkVolumeCM("nodepool-builder-config-vol",
			"nodepool-builder-config-config-map"),
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
			Name:      "nodepool-home-dib",
			MountPath: "/var/lib/nodepool/dib",
		},
		{
			Name:      "nodepool-home-build",
			MountPath: "/var/lib/nodepool/build",
		},
		{
			Name:      "nodepool-builder-config-vol",
			SubPath:   "nodepool.yaml",
			MountPath: "/etc/nodepool/nodepool.yaml",
		},
	}

	annotations := map[string]string{
		"serial": "2",
	}

	replicas := int32(1)
	storageSpec := base.StorageConfig{
		StorageClassName: "topolvm-provisioner",
		Size:             utils.Qty1Gi(),
	}
	nb := r.mkStatefulSet(builderIdent, nodepoolBuilderImage, storageSpec, replicas, apiv1.ReadWriteOnce)

	nb.Spec.Template.ObjectMeta.Annotations = annotations
	nb.Spec.Template.Spec.Volumes = volumes
	nb.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMount

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

	return r.IsStatefulSetReady(&current)
}

func (r *SFController) DeployNodepoolLauncher() bool {

	launcheToolingData := make(map[string]string)
	launcheToolingData["generate-launcher-config.sh"] = generateConfigScript
	r.EnsureConfigMap("nodepool-launcher-tooling", launcheToolingData)

	// Unfortunatly I'm unable to leverage default value set at API validation
	logLevel := v1.InfoLogLevel
	if r.cr.Spec.Nodepool.Launcher.LogLevel != "" {
		logLevel = r.cr.Spec.Nodepool.Launcher.LogLevel
	}

	loggingConfig, _ := utils.ParseString(
		loggingConfigTemplate, struct{ LogLevel string }{LogLevel: string(logLevel)})

	launcherExtraConfigData := make(map[string]string)
	launcherExtraConfigData["logging.yaml"] = loggingConfig
	r.EnsureConfigMap("nodepool-launcher-extra-config", launcherExtraConfigData)

	volumes := []apiv1.Volume{
		base.MkVolumeSecret("zookeeper-client-tls"),
		base.MkVolumeSecret(NodepoolProvidersSecretsName),
		base.MkEmptyDirVolume("nodepool-config"),
		base.MkEmptyDirVolume("nodepool-home"),
		{
			Name: "nodepool-launcher-tooling-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "nodepool-launcher-tooling-config-map",
					},
					DefaultMode: &utils.Execmod,
				},
			},
		},
		base.MkVolumeCM("nodepool-launcher-extra-config-vol",
			"nodepool-launcher-extra-config-config-map"),
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
		"serial":                "5",
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
	container.Env = append(r.getNodepoolConfigEnvs(),
		base.MkEnvVar("HOME", "/var/lib/nodepool"))

	initContainer := base.MkContainer("nodepool-launcher-init", BusyboxImage)

	initContainer.Command = []string{"/usr/local/bin/generate-launcher-config.sh"}
	initContainer.Env = append(r.getNodepoolConfigEnvs(),
		base.MkEnvVar("HOME", "/var/lib/nodepool"))
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
	nl.Spec.Template.Spec.Containers = []apiv1.Container{container}
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
