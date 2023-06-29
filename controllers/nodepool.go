// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
package controllers

import (
	_ "embed"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

//go:embed static/nodepool/generate-launcher-config.sh
var generateConfigScript string

const NL_IDENT = "nodepool-launcher"
const NL_WEBAPP_PORT_NAME = "nlwebapp"
const NL_WEBAPP_PORT = 8006

var configScriptVolumeMount = apiv1.VolumeMount{
	Name:      "nodepool-launcher-tooling-vol",
	SubPath:   "generate-launcher-config.sh",
	MountPath: "/usr/local/bin/generate-launcher-config.sh",
}

func (r *SFController) get_generate_nodepool_config_envs() []apiv1.EnvVar {
	configRepoSet := "FALSE"
	if r.cr.Spec.ConfigLocation.BaseURL != "" &&
		r.cr.Spec.ConfigLocation.Name != "" &&
		r.cr.Spec.ConfigLocation.ZuulConnectionName != "" {
		configRepoSet = "TRUE"
	}
	return []apiv1.EnvVar{
		Create_env("CONFIG_REPO_SET", configRepoSet),
		Create_env("CONFIG_REPO_BASE_URL", r.cr.Spec.ConfigLocation.BaseURL),
		Create_env("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
	}
}

func (r *SFController) init_container(volumeMounts []apiv1.VolumeMount) apiv1.Container {
	container := MkContainer("nodepool-launcher-init", BUSYBOX_IMAGE)
	container.Command = []string{"/usr/local/bin/generate-launcher-config.sh"}
	container.Env = r.get_generate_nodepool_config_envs()
	container.VolumeMounts = append(volumeMounts, configScriptVolumeMount)
	container.SecurityContext = create_security_context(false)
	return container
}

func (r *SFController) sidecar_container(volumeMounts []apiv1.VolumeMount) apiv1.Container {
	container := MkContainer("nodepool-launcher-sidecar", BUSYBOX_IMAGE)
	container.Command = []string{"sh", "-c", "touch /tmp/healthy && sleep inf"}
	container.Env = r.get_generate_nodepool_config_envs()
	container.VolumeMounts = append(volumeMounts, configScriptVolumeMount)
	container.ReadinessProbe = Create_readiness_cmd_probe([]string{"cat", "/tmp/healthy"})
	container.SecurityContext = create_security_context(false)
	return container
}

func (r *SFController) DeployNodepool() bool {
	cert_client := r.create_client_certificate("zookeeper-client", "ca-issuer", "zookeeper-client-tls", "zookeeper", r.cr.Spec.FQDN)
	r.GetOrCreate(&cert_client)

	launcher_tooling_data := make(map[string]string)
	launcher_tooling_data["generate-launcher-config.sh"] = generateConfigScript
	r.EnsureConfigMap("nodepool-launcher-tooling", launcher_tooling_data)

	volumes := []apiv1.Volume{
		create_volume_secret("zookeeper-client-tls"),
		create_empty_dir("nodepool-config"),
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
	}

	_, err := r.getSecretbyNameRef("nodepool-providers-secrets")

	if err == nil {
		volumes = append(volumes, create_volume_secret("nodepool-providers-secrets"))
		volume_mount = append(volume_mount, apiv1.VolumeMount{
			Name:      "nodepool-providers-secrets",
			SubPath:   "kube.config",
			MountPath: "/.kube/config",
			ReadOnly:  true,
		})
		volume_mount = append(volume_mount, apiv1.VolumeMount{
			Name:      "nodepool-providers-secrets",
			SubPath:   "clouds.yaml",
			MountPath: "/.config/openstack/clouds.yaml",
			ReadOnly:  true,
		})
	}

	annotations := map[string]string{
		"nodepool.yaml":         checksum([]byte(generateConfigScript)),
		"config-repo-info-hash": r.cr.Spec.ConfigLocation.BaseURL + r.cr.Spec.ConfigLocation.Name,
		"serial":                "1",
	}

	nl := r.create_deployment("nodepool-launcher", "")
	container := apiv1.Container{
		Name:            "launcher",
		Image:           "quay.io/software-factory/" + NL_IDENT + ":8.2.0-2",
		SecurityContext: create_security_context(false),
		VolumeMounts:    volume_mount,
	}
	nl.Spec.Template.Spec.Volumes = volumes
	nl.Spec.Template.Spec.InitContainers = []apiv1.Container{r.init_container(volume_mount)}
	nl.Spec.Template.Spec.Containers = []apiv1.Container{container, r.sidecar_container(volume_mount)}
	nl.Spec.Template.ObjectMeta.Annotations = annotations
	// FIXME: Add readiness and liveness probe when they are available.
	//nl.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9090)
	//nl.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", 9090)
	nl.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(NL_WEBAPP_PORT, NL_WEBAPP_PORT_NAME),
	}

	current := appsv1.Deployment{}
	if r.GetM(NL_IDENT, &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Nodepool-launcher configuration changed, restarting ...")
			current.Spec = nl.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := nl
		r.CreateR(&current)
	}

	return r.IsDeploymentReady(&current)
}
