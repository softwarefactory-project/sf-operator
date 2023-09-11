// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
package controllers

import (
	_ "embed"

	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/nodepool/generate-launcher-config.sh
var generateConfigScript string

//go:embed static/nodepool/logging.yaml.tmpl
var loggingConfigTemplate string

const NL_IDENT = "nodepool-launcher"
const NL_WEBAPP_PORT_NAME = "nlwebapp"
const NL_WEBAPP_PORT = 8006
const NodepoolProvidersSecretsName = "nodepool-providers-secrets"

const nodepoolLauncherImage = "quay.io/software-factory/" + NL_IDENT + ":9.0.0-1"

var configScriptVolumeMount = apiv1.VolumeMount{
	Name:      "nodepool-launcher-tooling-vol",
	SubPath:   "generate-launcher-config.sh",
	MountPath: "/usr/local/bin/generate-launcher-config.sh",
}

func (r *SFController) get_generate_nodepool_config_envs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		return []apiv1.EnvVar{
			Create_env("CONFIG_REPO_SET", "TRUE"),
			Create_env("CONFIG_REPO_BASE_URL", r.cr.Spec.ConfigLocation.BaseURL),
			Create_env("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
		}
	} else {
		return []apiv1.EnvVar{
			Create_env("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) DeployNodepool() bool {
	cert_client := r.create_client_certificate("zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper", r.cr.Spec.FQDN)
	r.GetOrCreate(&cert_client)

	launcher_tooling_data := make(map[string]string)
	launcher_tooling_data["generate-launcher-config.sh"] = generateConfigScript
	r.EnsureConfigMap("nodepool-launcher-tooling", launcher_tooling_data)

	// Unfortunatly I'm unable to leverage default value set at API validation
	logLevel := v1.InfoLogLevel
	if r.cr.Spec.Nodepool.Launcher.LogLevel != "" {
		logLevel = r.cr.Spec.Nodepool.Launcher.LogLevel
	}

	loggingConfig, _ := Parse_string(
		loggingConfigTemplate, struct{ LogLevel string }{LogLevel: string(logLevel)})

	launcher_extra_config_data := make(map[string]string)
	launcher_extra_config_data["logging.yaml"] = loggingConfig
	r.EnsureConfigMap("nodepool-launcher-extra-config", launcher_extra_config_data)

	volumes := []apiv1.Volume{
		create_volume_secret("zookeeper-client-tls"),
		create_volume_secret(NodepoolProvidersSecretsName),
		create_empty_dir("nodepool-config"),
		create_empty_dir("nodepool-home"),
		{
			Name: "nodepool-launcher-tooling-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "nodepool-launcher-tooling-config-map",
					},
					DefaultMode: &Execmod,
				},
			},
		},
		Create_volume_cm("nodepool-launcher-extra-config-vol",
			"nodepool-launcher-extra-config-config-map"),
	}

	volume_mount := []apiv1.VolumeMount{
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
		{
			Name:      "nodepool-providers-secrets",
			SubPath:   "kube.config",
			MountPath: "/var/lib/nodepool/.kube/config",
			ReadOnly:  true,
		},
		{
			Name:      "nodepool-providers-secrets",
			SubPath:   "clouds.yaml",
			MountPath: "/var/lib/nodepool/.config/openstack/clouds.yaml",
			ReadOnly:  true,
		},
		configScriptVolumeMount,
	}

	// We set a place holder secret to ensure that the Secret is owned by the SoftwareFactory instance (ControllerReference)
	var nodepool_providers_secrets apiv1.Secret
	if !r.GetM(NodepoolProvidersSecretsName, &nodepool_providers_secrets) {
		r.CreateR(&apiv1.Secret{
			Data:       map[string][]byte{},
			ObjectMeta: metav1.ObjectMeta{Name: NodepoolProvidersSecretsName, Namespace: r.ns}})
	} else {
		if len(nodepool_providers_secrets.GetOwnerReferences()) == 0 {
			r.log.V(1).Info("Adopting the providers secret to set the owner reference", "secret", NodepoolProvidersSecretsName)
			if !r.UpdateR(&nodepool_providers_secrets) {
				return false
			}
		}
	}

	annotations := map[string]string{
		"nodepool.yaml":         checksum([]byte(generateConfigScript)),
		"nodepool-logging.yaml": checksum([]byte(loggingConfig)),
		"serial":                "4",
		// When the Secret ResourceVersion field change (when edited) we force a nodepool-launcher restart
		"nodepool-providers-secrets": string(nodepool_providers_secrets.ResourceVersion),
		"nodepool-launcher-image":    nodepoolLauncherImage,
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigLocation.BaseURL + r.cr.Spec.ConfigLocation.Name
	}

	nl := r.create_deployment("nodepool-launcher", "")

	container := apiv1.Container{
		Name:            "launcher",
		Image:           nodepoolLauncherImage,
		SecurityContext: create_security_context(false),
		VolumeMounts:    volume_mount,
		Env: append(r.get_generate_nodepool_config_envs(),
			Create_env("HOME", "/var/lib/nodepool")),
		Command: []string{"/usr/local/bin/dumb-init", "--",
			"/usr/local/bin/nodepool-launcher", "-f", "-l", "/etc/nodepool-logging/logging.yaml"},
	}

	init_container := MkContainer("nodepool-launcher-init", BUSYBOX_IMAGE)
	init_container.Command = []string{"/usr/local/bin/generate-launcher-config.sh"}
	init_container.Env = append(r.get_generate_nodepool_config_envs(),
		Create_env("HOME", "/var/lib/nodepool"))
	init_container.VolumeMounts = []apiv1.VolumeMount{
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
	init_container.SecurityContext = create_security_context(false)

	nl.Spec.Template.Spec.Volumes = volumes
	nl.Spec.Template.Spec.InitContainers = []apiv1.Container{init_container}
	nl.Spec.Template.Spec.Containers = []apiv1.Container{container}
	nl.Spec.Template.ObjectMeta.Annotations = annotations
	nl.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/ready", NL_WEBAPP_PORT)
	nl.Spec.Template.Spec.Containers[0].LivenessProbe = create_liveness_http_probe("/ready", NL_WEBAPP_PORT)
	nl.Spec.Template.Spec.Containers[0].StartupProbe = create_startup_http_probe("/ready", NL_WEBAPP_PORT)
	nl.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(NL_WEBAPP_PORT, NL_WEBAPP_PORT_NAME),
	}

	current := appsv1.Deployment{}
	if r.GetM(NL_IDENT, &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool-launcher configuration changed, rollout pods ...")
			current.Spec = nl.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := nl
		r.CreateR(&current)
	}

	srv := r.create_service(NL_IDENT, NL_IDENT, []int32{NL_WEBAPP_PORT}, NL_IDENT)
	r.GetOrCreate(&srv)

	route_ready := r.ensureHTTPSRoute(r.cr.Name+"-nodepool-launcher", "nodepool", NL_IDENT, "/",
		NL_WEBAPP_PORT, map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	isDeploymentReady := r.IsDeploymentReady(&current)
	if isDeploymentReady {
		refresh_condition(&r.cr.Status.Conditions, NL_IDENT, metav1.ConditionTrue, "Complete", "Initialization of "+NL_IDENT+" service completed.")
	} else {
		refresh_condition(&r.cr.Status.Conditions, NL_IDENT, metav1.ConditionUnknown, "Awaiting", "Initializing "+NL_IDENT+" service...")
	}

	return isDeploymentReady && route_ready
}
