// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package zuulcf contains a library to build Zuul configurations.
package zuulcf

import (
	"fmt"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

var TenantTemplate = `- tenant:
    name: {{ .Name }}
    source:
      {{ .Source }}:
        {{ if .Trusted }}config-projects:
        {{ range .Trusted}}- {{ . }}
        {{end}}
        {{- end -}}
        {{ if .Untrusted  }}untrusted-projects:
        {{ range .Untrusted}}- {{ . }}
        {{ end -}}
        {{- end }}
`

type TenantTemplateStruct struct {
	Name      string
	Source    string
	Trusted   []string
	Untrusted []string
}

func GenerateZuulTemplateFile(tenant TenantTemplateStruct) (string, error) {
	templateConfig, err := utils.ParseString(TenantTemplate, tenant)

	if err != nil {
		fmt.Print(err)
		return "", err
	}
	return templateConfig, nil
}

type TenantConnProjects struct {
	ConfigProjects    []string `yaml:"config-projects"`
	UntrustedProjects []string `yaml:"untrusted-projects"`
}

type TenantConnectionSource map[string]TenantConnProjects

type TenantBody struct {
	Name   string                 `yaml:"name"`
	Source TenantConnectionSource `yaml:"source,omitempty"`
}

type Tenant struct {
	Tenant TenantBody `yaml:"tenant"`
}

type TenantConfig []Tenant

func GetZuulProjectMergeMode(mergemode string) ZuulProjectMergeMode {
	var mergemodestr ZuulProjectMergeMode
	switch mergemode {
	case "merge":
		mergemodestr = Merge
	case "merge-resolve":
		mergemodestr = MergeResolve
	case "cherry-pick":
		mergemodestr = CherryPick
	case "squash-merge":
		mergemodestr = SquashMerge
	case "rebase":
		mergemodestr = Rebase
	default:
		mergemodestr = Empty
	}
	return mergemodestr
}

type ZuulProjectPipeline struct {
	Jobs     []string `yaml:"jobs,omitempty"`
	Debug    bool     `yaml:"debug,omitempty"`
	FailFast bool     `yaml:"fail-fast,omitempty"`
}

type ZuulProjectPipelineMap map[string]ZuulProjectPipeline

const (
	Merge        ZuulProjectMergeMode = "merge"
	MergeResolve ZuulProjectMergeMode = "merge-resolve"
	CherryPick   ZuulProjectMergeMode = "cherry-pick"
	SquashMerge  ZuulProjectMergeMode = "squash-merge"
	Rebase       ZuulProjectMergeMode = "rebase"
	Empty        ZuulProjectMergeMode = ""
)

type ZuulProjectMergeMode string
type ZuulProjectBody struct {
	Name          string                 `yaml:"name,omitempty"`
	Templates     []string               `yaml:"templates,omitempty"`
	DefaultBranch string                 `yaml:"default-branch,omitempty"`
	MergeMode     ZuulProjectMergeMode   `yaml:"merge-mode,omitempty"`
	Vars          map[string]interface{} `yaml:"vars,omitempty"`
	Queue         string                 `yaml:"queue,omitempty"`
	Pipeline      ZuulProjectPipelineMap `yaml:",inline"`
}

type Project struct {
	Project ZuulProjectBody `yaml:"project"`
}

type ProjectConfig []Project

type JobSecrets struct {
	Name         string `yaml:"name"`
	Secret       string `yaml:"secret"`
	PassToParent bool   `yaml:"pass-to-parent,omitempty"`
}

type JobRoles map[string]string

type JobRunNameAndSemaphore struct {
	Name      string `yaml:"name"`
	Semaphore string `yaml:"semaphore,omitempty"`
}

type JobRunName struct {
	Name string `yaml:",inline"`
}

type JobBody struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description,omitempty"`
	ExtraVars   map[string]interface{}   `yaml:"extra-vars,omitempty"`
	Parent      *string                  `yaml:"parent"`
	PostRun     []string                 `yaml:"post-run,omitempty"`
	PreRun      []string                 `yaml:"pre-run,omitempty"`
	Roles       []JobRoles               `yaml:"roles,omitempty"`
	Secrets     []interface{}            `yaml:"secrets,omitempty"`
	Timeout     uint16                   `yaml:"timeout,omitempty"`
	Attempts    uint8                    `yaml:"attempts,omitempty"`
	Final       bool                     `yaml:"final,omitempty"`
	Run         []interface{}            `yaml:"run,omitempty"`
	NodeSet     NodeSetBody              `yaml:"nodeset,omitempty"`
	Semaphores  []JobRunNameAndSemaphore `yaml:"semaphores,omitempty"`
}

type Job struct {
	Job JobBody `yaml:"job"`
}

type JobConfig []Job

func GetZuulPipelineManager(manager string) PipelineManager {
	var managerstr PipelineManager
	switch manager {
	case "independent":
		managerstr = Independent
	case "dependent":
		managerstr = Dependent
	case "serial":
		managerstr = Serial
	case "supercedent":
		managerstr = Supercedent
	default:
		managerstr = Independent
	}
	return managerstr
}

func GetZuulPipelinePrecedence(precedence string) PipelinePrecedence {
	var precedencestr PipelinePrecedence
	switch precedence {
	case "high":
		precedencestr = High
	case "low":
		precedencestr = Low
	case "normal":
	default:
		precedencestr = Normal
	}
	return precedencestr
}

func GetZuulPipelineReporterVerified(verified string) GerritVotePoint {
	var verifiedstr GerritVotePoint
	switch verified {
	case "2":
		verifiedstr = GerritVotePointTwo
	case "1":
		verifiedstr = GerritVotePointOne
	case "0":
		verifiedstr = GerritVotePointZero
	case "-1":
		verifiedstr = GerritVotePointMinusOne
	case "-2":
		verifiedstr = GerritVotePointMinusTwo
	default:
		verifiedstr = GerritVotePointZero
	}
	return verifiedstr
}

type GerritVotePoint int8

const (
	// This is needed due to https://go.dev/ref/spec#The_zero_value
	// Zero Values for int's will not show while printing the structure
	GerritVotePointTwo      = 2
	GerritVotePointOne      = 1
	GerritVotePointZero     = 0
	GerritVotePointMinusOne = -1
	GerritVotePointMinusTwo = -2
)

// This struct defines Trigger Configurations for Gerrit, Gitlab and GitHub
// The fields defined are common to all three configurations

func GetGerritWorkflowValue(value string) GerritWorkflow {
	var workflowvalue GerritWorkflow
	switch value {
	case "1":
		workflowvalue = GerritVotePointOne
	case "0":
		workflowvalue = GerritVotePointZero
	case "-1":
		workflowvalue = GerritVotePointMinusOne
	default:
		workflowvalue = GerritVotePointZero
	}
	return workflowvalue
}

type GerritWorkflow GerritVotePoint

func GetGerritCodeReviewValue(value string) GerritCodeReview {
	var codereviewvalue GerritCodeReview
	switch value {
	case "2":
		codereviewvalue = GerritVotePointTwo
	case "1":
		codereviewvalue = GerritVotePointOne
	case "0":
		codereviewvalue = GerritVotePointZero
	case "-1":
		codereviewvalue = GerritVotePointMinusOne
	case "-2":
		codereviewvalue = GerritVotePointMinusTwo
	default:
		codereviewvalue = GerritVotePointZero
	}
	return codereviewvalue
}

type GerritCodeReview GerritVotePoint

type PipelineRequireGerritApproval struct {
	Workflow   GerritWorkflow   `yaml:"Workflow,omitempty"`
	Verified   GerritVotePoint  `yaml:"Verified,omitempty,flow"`
	CodeReview GerritCodeReview `yaml:"Code-Review,omitempty"`
}

type PipelineGerritRequirement struct {
	Username        string                          `yaml:"username,omitempty"`
	CurrentPatchset bool                            `yaml:"current-patchset,omitempty"`
	Verified        []GerritVotePoint               `yaml:"Verified,omitempty,flow"`
	GerritApproval  []PipelineRequireGerritApproval `yaml:"approval,omitempty"`
	Workflow        GerritWorkflow                  `yaml:"Workflow,omitempty"`
}

type PipelineGitLabRequirement struct {
	Merged   bool     `yaml:"merged,omitempty"`
	Approved bool     `yaml:"approved,omitempty"`
	Labels   []string `yaml:"labels,omitempty"`
}

type PipelineRequireApproval struct {
	Open   bool                      `yaml:"open,omitempty"`
	Gerrit PipelineGerritRequirement `yaml:",inline,omitempty"`
	Gitlab PipelineGitLabRequirement `yaml:",inline,omitempty"`
}

type PipelineTriggerGitGerrit struct {
	Approval []PipelineRequireApproval `yaml:"approval,omitempty"`
}

type GitGLMergeRequests string

const (
	Opened     GitGLMergeRequests = "opened"
	Changed    GitGLMergeRequests = "changed"
	Merged     GitGLMergeRequests = "merged"
	Comment    GitGLMergeRequests = "comment"
	Approved   GitGLMergeRequests = "approved"
	Unapproved GitGLMergeRequests = "unapproved"
	Labeled    GitGLMergeRequests = "labeled"
)

type PipelineTriggerGitLab struct {
	Action   []GitGLMergeRequests `yaml:"action,omitempty"`
	Labels   []string             `yaml:"labels,omitempty"`
	Unlabels []string             `yaml:"unlabels,omitempty"`
}

type PipelineTriggerGit struct {
	Gerrit   PipelineTriggerGitGerrit        `yaml:"require,omitempty"`
	Approval []PipelineRequireGerritApproval `yaml:"approval,omitempty"`
	Username string                          `yaml:"username,omitempty"`
}

type PipelineTrigger struct {
	Event         string                `yaml:"event"`
	Comment       string                `yaml:"comment,omitempty"`
	Ref           []string              `yaml:"ref,omitempty"`
	GitTrigger    PipelineTriggerGit    `yaml:",inline"`
	GitLabTrigger PipelineTriggerGitLab `yaml:",inline"`
}

type PipelineTriggerArray []PipelineTrigger

type PipelineGenericTrigger map[string]PipelineTriggerArray

type PipelineGitLabReporter struct {
	Comment  bool     `yaml:"comment"`
	Approval bool     `yaml:"approval"`
	Merge    bool     `yaml:"merge,omitempty"`
	Labels   []string `yaml:"labels,omitempty"`
	Unlabels []string `yaml:"unlabels,omitempty"`
}

type PipelineGerritReporter struct {
	Submit   bool            `yaml:"submit,omitempty"`
	Verified GerritVotePoint `yaml:"Verified"`
}

type PipelineDriverReporter struct {
	Gitlab *PipelineGitLabReporter `yaml:",inline,omitempty"`
	Gerrit *PipelineGerritReporter `yaml:",inline,omitempty"`
}

type ReporterMap map[string]PipelineDriverReporter

type GitLabReporterMap map[string]PipelineGitLabReporter

type GerritReporterMap map[string]PipelineGerritReporter

type PipelineReporter struct {
	Reporter    ReporterMap `yaml:",inline"`
	SQLReporter []string    `yaml:"sqlreporter,omitempty"`
}

type PipelinePrecedence string

const (
	High   PipelinePrecedence = "high"
	Low    PipelinePrecedence = "low"
	Normal PipelinePrecedence = "normal"
)

type PipelineManager string

const (
	Dependent   PipelineManager = "dependent"
	Independent PipelineManager = "independent"
	Serial      PipelineManager = "serial"
	Supercedent PipelineManager = "supercedent"
)

type PipelineRequire map[string]PipelineRequireApproval

type PipelineBody struct {
	Name                 string                 `yaml:"name"`
	Description          string                 `yaml:"description,omitempty"`
	Manager              PipelineManager        `yaml:"manager"`
	PostReview           bool                   `yaml:"post-review,omitempty"`
	Precedence           PipelinePrecedence     `yaml:"precedence,omitempty"`
	Supercedes           []string               `yaml:"supercedes,omitempty"`
	SuccessMessage       string                 `yaml:"success-message,omitempty"`
	FailureMessage       string                 `yaml:"failure-message,omitempty"`
	Require              PipelineRequire        `yaml:"require,omitempty"`
	Start                PipelineReporter       `yaml:"start,omitempty"`
	Success              PipelineReporter       `yaml:"success,omitempty"`
	Failure              PipelineReporter       `yaml:"failure,omitempty"`
	Trigger              PipelineGenericTrigger `yaml:"trigger,omitempty"`
	WindowFloor          uint8                  `yaml:"window-floor,omitempty"`
	WindowIncreaseFactor uint8                  `yaml:"window-increase-factor,omitempty"`
}

type Pipeline struct {
	Pipeline PipelineBody `yaml:"pipeline"`
}

type PipelineConfig []Pipeline

type AnsiblePlay struct {
	Hosts string           `yaml:"hosts"`
	Roles []map[string]any `yaml:"roles,omitempty"`
	Vars  []map[string]any `yaml:"vars,omitempty"`
	Tasks []map[string]any `yaml:"tasks,omitempty"`
}

type AnsiblePlayBook []AnsiblePlay

type NodeSetNodesBody struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label"`
}

type NodeSetNodesGroupBody struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label"`
}

type NodesSetAlternatives string

type NodeSetBody struct {
	Name         string                  `yaml:"name,omitempty"`
	Nodes        []NodeSetNodesBody      `yaml:"nodes,omitempty"`
	Groups       []NodeSetNodesGroupBody `yaml:"groups,omitempty"`
	Alternatives []NodesSetAlternatives  `yaml:"alternatives,omitempty"`
}

type NodeSet struct {
	NodeSet NodeSetBody `yaml:"nodeset"`
}

type NodeSets []NodeSet

type SemaphoreBody struct {
	Name string `yaml:"name"`
	Max  int8   `yaml:"max,omitempty"`
}

type Semaphore struct {
	Semaphore SemaphoreBody `yaml:"semaphore"`
}

type Semaphores []Semaphore

func GetRequireCheckByDriver(driver string, connection string) (PipelineRequire, error) {
	require := PipelineRequire{}

	switch driver {
	case "gerrit":
		require = PipelineRequire{
			connection: PipelineRequireApproval{
				Open: true,
				Gerrit: PipelineGerritRequirement{
					CurrentPatchset: true,
				},
			},
		}
	case "gitlab":
		require = PipelineRequire{
			connection: PipelineRequireApproval{
				Open: true,
			},
		}
	default:
		return require, fmt.Errorf("check Pipeline Require: Driver of type \"%s\" is not supported", driver)
	}

	return require, nil
}

func GetRequireGateByDriver(driver string, connection string) (PipelineRequire, error) {
	require := PipelineRequire{}

	switch driver {
	case "gerrit":
		require = PipelineRequire{
			connection: PipelineRequireApproval{
				Open: true,
				Gerrit: PipelineGerritRequirement{
					CurrentPatchset: true,
					GerritApproval: []PipelineRequireGerritApproval{
						{
							Workflow: GetGerritWorkflowValue("1"),
						},
					},
				},
			},
		}
	case "gitlab":
		require = PipelineRequire{
			connection: PipelineRequireApproval{
				Open: true,
				Gitlab: PipelineGitLabRequirement{
					Approved: true,
					Labels: []string{
						"gateit",
					},
				},
			},
		}
	default:
		return require, fmt.Errorf("gate Pipeline Require: Driver of type \"%s\" is not supported", driver)
	}

	return require, nil
}

func GetTriggerCheckByDriver(driver string, connection string) (PipelineGenericTrigger, error) {
	trigger := PipelineGenericTrigger{}
	switch driver {
	case "gerrit":
		trigger = PipelineGenericTrigger{
			connection: PipelineTriggerArray{
				{
					Event: "patchset-created",
				},
				{
					Event: "change-restored",
				},
				{
					Event:   "comment-added",
					Comment: "(?i)^(Patch Set [0-9]+:)?( [\\w\\+-]*)*(\\n\\n)?\\s*(recheck|reverify)",
				},
			},
		}
	case "gitlab":
		trigger = PipelineGenericTrigger{
			connection: PipelineTriggerArray{
				{
					Event: "gl_merge_request",
					GitLabTrigger: PipelineTriggerGitLab{
						Action: []GitGLMergeRequests{
							GitGLMergeRequests(Comment),
						},
					},
					Comment: "(?i)^\\s*recheck\\s*$",
				},
				{
					Event: "gl_merge_request",
					GitLabTrigger: PipelineTriggerGitLab{
						Action: []GitGLMergeRequests{
							GitGLMergeRequests(Opened),
							GitGLMergeRequests(Changed),
						},
					},
				},
			},
		}
	default:
		return trigger, fmt.Errorf("check Pipeline Trigger: Driver of type \"%s\" is not supported", driver)
	}

	return trigger, nil
}

func GetTriggerGateByDriver(driver string, connection string) (PipelineGenericTrigger, error) {
	trigger := PipelineGenericTrigger{}
	switch driver {
	case "gerrit":
		trigger = PipelineGenericTrigger{
			connection: PipelineTriggerArray{
				{
					Event: "comment-added",
					GitTrigger: PipelineTriggerGit{
						Approval: []PipelineRequireGerritApproval{
							{
								Workflow: GetGerritWorkflowValue("1"),
							},
						},
					},
				},
				{
					Event: "comment-added",
					GitTrigger: PipelineTriggerGit{
						Approval: []PipelineRequireGerritApproval{
							{
								Verified: GetZuulPipelineReporterVerified("1"),
							},
						},
						Username: "zuul",
					},
				},
			},
		}
	case "gitlab":
		trigger = PipelineGenericTrigger{
			connection: PipelineTriggerArray{
				{
					Event: "gl_merge_request",
					GitLabTrigger: PipelineTriggerGitLab{
						Action: []GitGLMergeRequests{
							GitGLMergeRequests(Approved),
						},
					},
				},
				{
					Event: "gl_merge_request",
					GitLabTrigger: PipelineTriggerGitLab{
						Action: []GitGLMergeRequests{
							GitGLMergeRequests(Labeled),
						},
						Labels: []string{
							"gateit",
						},
					},
				},
			},
		}
	default:
		return trigger, fmt.Errorf("gate Pipeline Trigger: Driver of type \"%s\" is not supported", driver)
	}

	return trigger, nil
}

func GetTriggerPostByDriver(driver string, connection string) (PipelineGenericTrigger, error) {
	trigger := PipelineGenericTrigger{
		"git-server": PipelineTriggerArray{
			{
				Event: "ref-updated",
			},
		},
	}

	switch driver {
	case "gerrit":
		trigger[connection] = PipelineTriggerArray{
			{
				Event: "ref-updated",
				Ref: []string{
					"^refs/heads/master$",
					"^refs/heads/main$",
				},
			},
		}
	case "gitlab":
		trigger[connection] = PipelineTriggerArray{
			{
				Event: "gl_push",
				Ref: []string{
					"^refs/heads/master$",
					"^refs/heads/main$",
				},
			},
		}
	default:
		return trigger, fmt.Errorf("post Pipeline Trigger: Driver of type \"%s\" is not supported", driver)
	}

	return trigger, nil
}

func GetReportersCheckByDriver(driver string, connection string) ([]PipelineReporter, error) {
	reporters := []PipelineReporter{}
	switch driver {
	case "gerrit":
		// Start
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("0"),
						},
					},
				},
			})
		// Success
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("1"),
						},
					},
				},
			})
		// Failure
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("-1"),
						},
					},
				},
			})
	case "gitlab":
		// Start
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: false,
						},
					},
				},
			})
		// Success
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: true,
						},
					},
				},
			})
		// Failure
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: false,
						},
					},
				},
			})
	default:
		reporters = append(reporters, PipelineReporter{},
			PipelineReporter{},
			PipelineReporter{})
		return reporters, fmt.Errorf("check Pipeline Reporters: Driver of type \"%s\" is not supported", driver)

	}

	return reporters, nil
}

func GetReportersGateByDriver(driver string, connection string) ([]PipelineReporter, error) {
	reporters := []PipelineReporter{}
	switch driver {
	case "gerrit":
		// Start
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("0"),
						},
					},
				},
			})
		// Success
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("2"),
							Submit:   true,
						},
					},
				},
			})
		// Failure
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gerrit: &PipelineGerritReporter{
							Verified: GetZuulPipelineReporterVerified("-2"),
						},
					},
				},
			})
	case "gitlab":
		// Start
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: false,
						},
					},
				},
			})
		// Success
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: true,
							Merge:    true,
						},
					},
				},
			})
		// Failure
		reporters = append(reporters,
			PipelineReporter{
				Reporter: ReporterMap{
					connection: PipelineDriverReporter{
						Gitlab: &PipelineGitLabReporter{
							Comment:  true,
							Approval: false,
						},
					},
				},
			})
	default:
		reporters = append(reporters, PipelineReporter{},
			PipelineReporter{},
			PipelineReporter{})
		return reporters, fmt.Errorf("gate Pipeline Reporters: Driver of type \"%s\" is not supported", driver)
	}

	return reporters, nil
}
