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
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

const GitServerIdent = "git-server"
const gsGitPort = 9418
const gsGitPortName = "git-server-port"
const gsGitMountPath = "/git"
const gsPiMountPath = "/entry"

//go:embed static/git-server/update-system-config.sh
var preInitScriptTemplate string

// This function creates dummy connections to be used during the config-check
func makeZuulConnectionConfig(spec *sfv1.ZuulSpec) string {
	var sb strings.Builder
	connectionNames := sfv1.GetGerritConnectionsName(spec)
	connectionNames = append(connectionNames, sfv1.GetGitHubConnectionsName(spec)...)
	connectionNames = append(connectionNames, sfv1.GetGitLabConnectionsName(spec)...)
	connectionNames = append(connectionNames, sfv1.GetGitConnectionsName(spec)...)
	sb.WriteString("\n")
	for _, name := range connectionNames {
		sb.WriteString(fmt.Sprintf("[connection %s]\n", name))
		sb.WriteString("driver=git\n")
		sb.WriteString("baseurl=localhost\n\n")
	}
	return sb.String()
}

func (r *SFController) makePreInitScript() string {

	parentJobName := "base"

	semaphore := zuulcf.Semaphores{
		{
			Semaphore: zuulcf.SemaphoreBody{
				Name: "semaphore-config-update",
				Max:  1,
			},
		},
	}

	jobs := zuulcf.JobConfig{
		{
			Job: zuulcf.JobBody{
				Name:        "base",
				Parent:      nil,
				Description: "The base job.",
				PreRun: []string{
					"playbooks/base/pre.yaml",
				},
				PostRun: []string{
					"playbooks/base/post.yaml",
				},
				Roles: []zuulcf.JobRoles{
					{
						"zuul": "zuul/zuul-jobs",
					},
				},
				Timeout:  1800,
				Attempts: 3,
				Secrets: []interface{}{
					"site_sflogs",
				},
			},
		},
		{
			Job: zuulcf.JobBody{
				Name:        "config-check",
				Parent:      &parentJobName,
				Final:       true,
				Description: "Validate the config repo.",
				Run: []interface{}{
					"playbooks/config/check.yaml",
				},
			},
		},
		{
			Job: zuulcf.JobBody{
				Name:        "config-update",
				Parent:      &parentJobName,
				Final:       true,
				Description: "Deploy config repo update.",
				Run: []interface{}{
					"playbooks/config/update.yaml",
				},
				Secrets: []interface{}{
					"k8s_config",
				},
				Semaphores: []zuulcf.JobRunNameAndSemaphore{
					{
						Name: "semaphore-config-update",
					},
				},
			},
		},
	}

	semaphoreOutput, _ := yaml.Marshal(semaphore)
	jobbaseOutput, _ := yaml.Marshal(jobs)

	preInitScriptTemplate = strings.Replace(preInitScriptTemplate, "# Semaphore", string(semaphoreOutput), 1)
	preInitScriptTemplate = strings.Replace(preInitScriptTemplate, "# JobBase", string(jobbaseOutput), 1)

	return strings.Replace(
		preInitScriptTemplate,
		"# ZUUL_CONNECTIONS",
		makeZuulConnectionConfig(&r.cr.Spec.Zuul), 1)
}

func (r *SFController) DeployGitServer() bool {
	preInitScript := r.makePreInitScript()
	cmData := make(map[string]string)
	cmData["pre-init.sh"] = preInitScript
	r.EnsureConfigMap(GitServerIdent+"-pi", cmData)

	annotations := map[string]string{
		"system-config": utils.Checksum([]byte(preInitScript)),
		"image":         base.GitServerImage,
		"serial":        "1",
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-name"] = r.cr.Spec.ConfigLocation.Name
		annotations["config-zuul-connection-name"] = r.cr.Spec.ConfigLocation.ZuulConnectionName
	}

	// Create the statefulset
	sts := r.mkStatefulSet(GitServerIdent, base.GitServerImage, r.getStorageConfOrDefault(r.cr.Spec.GitServer.Storage), apiv1.ReadWriteOnce)
	sts.Spec.Template.ObjectMeta.Annotations = annotations
	GSVolumeMounts := []apiv1.VolumeMount{
		{
			Name:      GitServerIdent,
			MountPath: gsGitMountPath,
		},
	}
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = GSVolumeMounts

	sts.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM(GitServerIdent+"-pi", GitServerIdent+"-pi-config-map"),
	}

	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(gsGitPort, gsGitPortName),
	}

	// Define initContainer
	initContainer := base.MkContainer("init-config", base.GitServerImage)
	initContainer.Command = []string{"/bin/bash", "/entry/pre-init.sh"}
	initContainer.Env = []apiv1.EnvVar{
		base.MkEnvVar("FQDN", r.cr.Spec.FQDN),
		base.MkEnvVar("LOGSERVER_SSHD_SERVICE_PORT", strconv.Itoa(sshdPort)),
	}
	initContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      GitServerIdent,
			MountPath: gsGitMountPath,
		},
		{
			Name:      GitServerIdent + "-pi",
			MountPath: gsPiMountPath,
		},
	}

	if r.isConfigRepoSet() {
		initContainer.Env = append(initContainer.Env,
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
			base.MkEnvVar("CONFIG_ZUUL_CONNECTION_NAME", r.cr.Spec.ConfigLocation.ZuulConnectionName))
	}

	sts.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}

	// Create readiness probes
	// Note: The probe is causing error message to be logged by the service
	// dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GS_GIT_PORT)

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(GitServerIdent, GSVolumeMounts)
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, statsExporter)

	current := appsv1.StatefulSet{}
	if r.GetM(GitServerIdent, &current) {
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("System configuration needs to be updated, restarting git-server...")
			current.Spec.Template = *sts.Spec.Template.DeepCopy()
			r.UpdateR(&current)
			return false
		}
	} else {
		current := sts
		r.CreateR(&current)
	}

	// Create services exposed
	svc := base.MkServicePod(GitServerIdent, r.ns, GitServerIdent+"-0", []int32{gsGitPort}, gsGitPortName)
	r.EnsureService(&svc)

	isStatefulset := r.IsStatefulSetReady(&current)
	conds.UpdateConditions(&r.cr.Status.Conditions, GitServerIdent, isStatefulset)

	return isStatefulset
}
