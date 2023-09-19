// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the git-server configuration.

package controllers

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const GS_IDENT = "git-server"
const GS_GIT_PORT = 9418
const GS_GIT_PORT_NAME = "git-server-port"
const GS_IMAGE = "quay.io/software-factory/git-deamon:2.39.1-3"
const GS_GIT_MOUNT_PATH = "/git"
const GS_PI_MOUNT_PATH = "/entry"

//go:embed static/git-server/update-system-config.sh
var preInitScriptTemplate string

// This function creates dummy connections to be used during the config-check
func makeZuulConnectionConfig(spec *sfv1.ZuulSpec) string {
	var sb strings.Builder
	sb.WriteString("\n")
	for _, name := range sfv1.GetConnectionsName(spec) {
		sb.WriteString(fmt.Sprintf("[connection %s]\n", name))
		sb.WriteString("driver=git\n")
		sb.WriteString("baseurl=localhost\n\n")
	}
	return sb.String()
}

func (r *SFController) makePreInitScript() string {
	return strings.Replace(
		preInitScriptTemplate,
		"# ZUUL_CONNECTIONS",
		makeZuulConnectionConfig(&r.cr.Spec.Zuul), 1)
}

func (r *SFController) DeployGitServer() bool {
	preInitScript := r.makePreInitScript()
	cm_data := make(map[string]string)
	cm_data["pre-init.sh"] = preInitScript
	r.EnsureConfigMap(GS_IDENT+"-pi", cm_data)

	annotations := map[string]string{
		"system-config": checksum([]byte(preInitScript)),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-name"] = r.cr.Spec.ConfigLocation.Name
		annotations["config-zuul-connection-name"] = r.cr.Spec.ConfigLocation.ZuulConnectionName
	}

	// Create the deployment
	replicas := int32(1)
	dep := r.create_statefulset(GS_IDENT, GS_IMAGE, r.getStorageConfOrDefault(r.cr.Spec.GitServer.Storage), replicas, apiv1.ReadWriteOnce)
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      GS_IDENT,
			MountPath: GS_GIT_MOUNT_PATH,
		},
	}

	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		Create_volume_cm(GS_IDENT+"-pi", GS_IDENT+"-pi-config-map"),
	}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(GS_GIT_PORT, GS_GIT_PORT_NAME),
	}

	// Define initContainer
	initContainer := MkContainer("init-config", GS_IMAGE)
	initContainer.Command = []string{"/bin/bash", "/entry/pre-init.sh"}
	initContainer.Env = []apiv1.EnvVar{
		Create_env("FQDN", r.cr.Spec.FQDN),
		Create_env("LOGSERVER_SSHD_SERVICE_PORT", strconv.Itoa(LOGSERVER_SSHD_PORT)),
	}
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      GS_IDENT,
			MountPath: GS_GIT_MOUNT_PATH,
		},
		{
			Name:      GS_IDENT + "-pi",
			MountPath: GS_PI_MOUNT_PATH,
		},
	}

	if r.isConfigRepoSet() {
		initContainer.Env = append(initContainer.Env,
			Create_env("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
			Create_env("CONFIG_ZUUL_CONNECTION_NAME", r.cr.Spec.ConfigLocation.ZuulConnectionName))
	}

	dep.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}

	// Create readiness probes
	// Note: The probe is causing error message to be logged by the service
	// dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GS_GIT_PORT)

	current := appsv1.StatefulSet{}
	if r.GetM(GS_IDENT, &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("System configuration needs to be updated, restarting git-server...")
			current.Spec = dep.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := dep
		r.CreateR(&current)
	}

	// Create services exposed
	git_service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GS_IDENT,
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     GS_GIT_PORT_NAME,
					Protocol: apiv1.ProtocolTCP,
					Port:     GS_GIT_PORT,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": GS_IDENT,
			},
		}}
	r.GetOrCreate(&git_service)

	is_statefulset := r.IsStatefulSetReady(&current)
	updateConditions(&r.cr.Status.Conditions, GS_IDENT, is_statefulset)

	return is_statefulset
}
