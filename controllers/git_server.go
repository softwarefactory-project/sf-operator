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
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const gsIdent = "git-server"
const gsGitPort = 9418
const gsGitPortName = "git-server-port"
const gsImage = "quay.io/software-factory/git-deamon:2.39.1-3"
const gsGitMountPath = "/git"
const gsPiMountPath = "/entry"

//go:embed static/git-server/update-system-config.sh
var preInitScriptTemplate string

// This function creates dummy connections to be used during the config-check
func makeZuulConnectionConfig(spec *sfv1.ZuulSpec) string {
	var sb strings.Builder
	sb.WriteString("\n")
	for _, name := range sfv1.GetGerritConnectionsName(spec) {
		sb.WriteString(fmt.Sprintf("[connection %s]\n", name))
		sb.WriteString("driver=git\n")
		sb.WriteString("baseurl=localhost\n\n")
	}
	for _, name := range sfv1.GetGitHubConnectionsName(spec) {
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
	cmData := make(map[string]string)
	cmData["pre-init.sh"] = preInitScript
	r.EnsureConfigMap(gsIdent+"-pi", cmData)

	annotations := map[string]string{
		"system-config": utils.Checksum([]byte(preInitScript)),
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-name"] = r.cr.Spec.ConfigLocation.Name
		annotations["config-zuul-connection-name"] = r.cr.Spec.ConfigLocation.ZuulConnectionName
	}

	// Create the deployment
	replicas := int32(1)
	dep := r.mkStatefulSet(gsIdent, gsImage, r.getStorageConfOrDefault(r.cr.Spec.GitServer.Storage), replicas, apiv1.ReadWriteOnce)
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      gsIdent,
			MountPath: gsGitMountPath,
		},
	}

	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM(gsIdent+"-pi", gsIdent+"-pi-config-map"),
	}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(gsGitPort, gsGitPortName),
	}

	// Define initContainer
	initContainer := base.MkContainer("init-config", gsImage)
	initContainer.Command = []string{"/bin/bash", "/entry/pre-init.sh"}
	initContainer.Env = []apiv1.EnvVar{
		base.MkEnvVar("FQDN", r.cr.Spec.FQDN),
		base.MkEnvVar("LOGSERVER_SSHD_SERVICE_PORT", strconv.Itoa(sshdPort)),
	}
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      gsIdent,
			MountPath: gsGitMountPath,
		},
		{
			Name:      gsIdent + "-pi",
			MountPath: gsPiMountPath,
		},
	}

	if r.isConfigRepoSet() {
		initContainer.Env = append(initContainer.Env,
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
			base.MkEnvVar("CONFIG_ZUUL_CONNECTION_NAME", r.cr.Spec.ConfigLocation.ZuulConnectionName))
	}

	dep.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}

	// Create readiness probes
	// Note: The probe is causing error message to be logged by the service
	// dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GS_GIT_PORT)

	current := appsv1.StatefulSet{}
	if r.GetM(gsIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
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
	gitService := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsIdent,
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     gsGitPortName,
					Protocol: apiv1.ProtocolTCP,
					Port:     gsGitPort,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": gsIdent,
			},
		}}
	r.GetOrCreate(&gitService)

	isStatefulset := r.IsStatefulSetReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, gsIdent, isStatefulset)

	return isStatefulset
}
