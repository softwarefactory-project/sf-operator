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
	"github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
	"gopkg.in/yaml.v3"
	apiv1 "k8s.io/api/core/v1"
)

const GitServerIdent = "git-server"
const GitServerIdentRW = "git-server-rw"
const gsGitPort = 9418
const gsGitPortRW = 9419
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
	connectionNames = append(connectionNames, sfv1.GetPagureConnectionsName(spec)...)
	sb.WriteString("\n")
	for _, name := range connectionNames {
		sb.WriteString(fmt.Sprintf("[connection %s]\n", name))
		sb.WriteString("driver=git\n")
		sb.WriteString("baseurl=localhost\n\n")
	}
	return sb.String()
}

func (r *SFController) MkPreInitScript() string {
	configRepoConnectionName := r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName
	configRepoName := r.cr.Spec.ConfigRepositoryLocation.Name

	getConnectionDriver := func(r *SFController, connName string) string {

		for _, con := range r.cr.Spec.Zuul.GerritConns {
			if connName == con.Name {
				return "gerrit"
			}
		}

		for _, con := range r.cr.Spec.Zuul.GitLabConns {
			if connName == con.Name {
				return "gitlab"
			}
		}
		// If not found, defaults to gerrit
		return "gerrit"

	}

	driver := getConnectionDriver(r, configRepoConnectionName)

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

	// Check Pipeline
	requireCheck, err := zuulcf.GetRequireCheckByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	triggerCheck, err := zuulcf.GetTriggerCheckByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	reportersCheck, err := zuulcf.GetReportersCheckByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	// Gate Pipeline
	requireGate, err := zuulcf.GetRequireGateByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	triggerGate, err := zuulcf.GetTriggerGateByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	reportersGate, err := zuulcf.GetReportersGateByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}
	// Post Pipeline
	triggerPost, err := zuulcf.GetTriggerPostByDriver(driver, configRepoConnectionName)
	if err != nil {
		fmt.Println(err)
	}

	pipelines := zuulcf.PipelineConfig{
		{
			Pipeline: zuulcf.PipelineBody{
				Name:        "check",
				Description: "Newly uploaded patchsets enter this pipeline to receive an initial +/-1 Verified vote.",
				Manager:     zuulcf.Independent,
				Require:     requireCheck,
				Trigger:     triggerCheck,
				Start:       reportersCheck[0],
				Success:     reportersCheck[1],
				Failure:     reportersCheck[2],
			},
		},
		{
			Pipeline: zuulcf.PipelineBody{
				Name:           "gate",
				Description:    "Changes that have been approved by core developers are enqueued in order in this pipeline, and if they pass tests, will be merged.",
				SuccessMessage: "Build succeeded (gate pipeline).",
				FailureMessage: "Build failed (gate pipeline).",
				Manager:        zuulcf.Dependent,
				Precedence:     zuulcf.High,
				Supercedes: []string{
					"check",
				},
				PostReview:           true,
				Require:              requireGate,
				Trigger:              triggerGate,
				Start:                reportersGate[0],
				Success:              reportersGate[1],
				Failure:              reportersGate[2],
				WindowFloor:          20,
				WindowIncreaseFactor: 2,
			},
		},
		{
			Pipeline: zuulcf.PipelineBody{
				Name:        "post",
				PostReview:  true,
				Description: "This pipeline runs jobs that operate after each change is merged.",
				Manager:     zuulcf.Supercedent,
				Precedence:  zuulcf.Low,
				Trigger:     triggerPost,
			},
		},
	}

	projects := zuulcf.ProjectConfig{
		{
			Project: zuulcf.ZuulProjectBody{
				Name: configRepoName,
				Pipeline: zuulcf.ZuulProjectPipelineMap{
					"check": zuulcf.ZuulProjectPipeline{
						Jobs: []string{
							"config-check",
						},
					},
					"gate": zuulcf.ZuulProjectPipeline{
						Jobs: []string{
							"config-check",
						},
					},
					"post": zuulcf.ZuulProjectPipeline{
						Jobs: []string{
							"config-update",
						},
					},
				},
			},
		},
	}

	semaphoreOutput, _ := yaml.Marshal(semaphore)
	jobbaseOutput, _ := yaml.Marshal(jobs)
	pipelineOutput, _ := yaml.Marshal(pipelines)
	projectOutput, _ := yaml.Marshal(projects)

	// We need to copy the global value `preInitScriptTemplate` to avoid updating the global
	// and thus loosing the markers.
	template := preInitScriptTemplate
	template = strings.Replace(template, "# Semaphores", string(semaphoreOutput), 1)
	template = strings.Replace(template, "# JobsBase", string(jobbaseOutput), 1)
	template = strings.Replace(template, "# Pipelines", string(pipelineOutput), 1)
	template = strings.Replace(template, "# Projects", string(projectOutput), 1)

	return strings.Replace(
		template,
		"# ZUUL_CONNECTIONS",
		makeZuulConnectionConfig(&r.cr.Spec.Zuul), 1)
}

func (r *SFController) DeployGitServer() bool {
	preInitScript := r.MkPreInitScript()
	cmData := make(map[string]string)
	cmData["pre-init.sh"] = preInitScript
	r.EnsureConfigMap(GitServerIdent+"-pi", cmData)

	annotations := map[string]string{
		"system-config": utils.Checksum([]byte(preInitScript)),
		"image":         base.GitServerImage(),
		"fqdn":          r.cr.Spec.FQDN,
		"serial":        "3",
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-name"] = r.cr.Spec.ConfigRepositoryLocation.Name
		annotations["config-zuul-connection-name"] = r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName
	}

	logserverHost := "logserver"
	if r.cr.Spec.ConfigRepositoryLocation.LogserverHost != "" {
		logserverHost = r.cr.Spec.ConfigRepositoryLocation.LogserverHost
		annotations["logserver_host"] = logserverHost
	}

	// Create the statefulset
	sts := r.mkStatefulSet(GitServerIdent, base.GitServerImage(), r.getStorageConfOrDefault(r.cr.Spec.GitServer.Storage), apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels)
	sts.Spec.Template.ObjectMeta.Annotations = annotations
	GSVolumeMountsRO := []apiv1.VolumeMount{
		{
			Name:      GitServerIdent,
			MountPath: gsGitMountPath,
			ReadOnly:  true,
		},
	}
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = GSVolumeMountsRO
	sts.Spec.Template.Spec.Containers[0].Command = []string{"git", "daemon", "--base-path=/git", "--export-all"}
	var probeCmd = []string{"git", "ls-remote", "git://localhost:" + strconv.Itoa(gsGitPort) + "/system-config"}
	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessCMDProbe(probeCmd)
	sts.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupCMDProbe(probeCmd)
	base.SetContainerLimitsLowProfile(&sts.Spec.Template.Spec.Containers[0])

	// Add a second container to serve the git-server with RW access (should not be exposed)
	containerRW := base.MkContainer(GitServerIdentRW, base.GitServerImage())
	containerRW.Command = []string{"git", "daemon", "--base-path=/git",
		"--enable=receive-pack", "--export-all", "--port=" + strconv.Itoa(gsGitPortRW)}
	containerRW.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      GitServerIdent,
			MountPath: gsGitMountPath,
		},
	}

	var probeCmdCRW = []string{"git", "ls-remote", "git://localhost:" + strconv.Itoa(gsGitPortRW) + "/system-config"}
	containerRW.ReadinessProbe = base.MkReadinessCMDProbe(probeCmdCRW)
	containerRW.StartupProbe = base.MkStartupCMDProbe(probeCmdCRW)
	base.SetContainerLimitsLowProfile(&containerRW)

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, containerRW)
	sts.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM(GitServerIdent+"-pi", GitServerIdent+"-pi-config-map"),
	}

	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(gsGitPort, gsGitPortName),
	}

	// Define initContainer
	initContainer := base.MkContainer("init-config", base.GitServerImage())
	initContainer.Command = []string{"/bin/bash", "/entry/pre-init.sh"}
	initContainer.Env = []apiv1.EnvVar{
		base.MkEnvVar("FQDN", r.cr.Spec.FQDN),
		base.MkEnvVar("ZUUL_LOGSERVER_HOST", logserverHost),
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
			base.MkEnvVar("CONFIG_REPO_NAME", r.cr.Spec.ConfigRepositoryLocation.Name),
		)
	}

	sts.Spec.Template.Spec.InitContainers = []apiv1.Container{initContainer}
	sts.Spec.Template.Spec.HostAliases = base.CreateHostAliases(r.cr.Spec.HostAliases)

	current, changed := r.ensureStatefulset(sts)
	if changed {
		return false
	}

	// Create services exposed
	svc := base.MkServicePod(GitServerIdent, r.ns, GitServerIdent+"-0", []int32{gsGitPort}, gsGitPortName, r.cr.Spec.ExtraLabels)
	r.EnsureService(&svc)
	svcRW := base.MkServicePod(GitServerIdentRW, r.ns, GitServerIdent+"-0", []int32{gsGitPortRW}, gsGitPortName, r.cr.Spec.ExtraLabels)
	r.EnsureService(&svcRW)

	ready := r.IsStatefulSetReady(current)
	conds.UpdateConditions(&r.cr.Status.Conditions, GitServerIdent, ready)

	return ready
}
