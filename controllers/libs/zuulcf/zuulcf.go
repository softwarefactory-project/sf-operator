// Package zuulcf provides a little library with utilities for Zuul Configurations
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

func GetZuulPipelineReporterVerified(verified string) GerritReporter {
	var verifiedstr GerritReporter
	switch verified {
	case "2":
		verifiedstr = VerifiedTwo
	case "1":
		verifiedstr = VerifiedOne
	case "0":
		verifiedstr = VerifiedZero
	case "-1":
		verifiedstr = VerifiedMinusOne
	case "-2":
		verifiedstr = VerifiedMinusTwo
	default:
		verifiedstr = VerifiedZero
	}
	return verifiedstr
}

type GerritReporter int8

const (
	// This is needed due to https://go.dev/ref/spec#The_zero_value
	// Zero Values for int's will not show while printing the structure
	VerifiedTwo      = 2
	VerifiedOne      = 1
	VerifiedZero     = 0
	VerifiedMinusOne = -1
	VerifiedMinusTwo = -2
)

// This struct defines Trigger Configurations for Gerrit, Gitlab and GitHub
// The fields defined are common to all three configurations

func GetGerritWorkflowValue(value string) GerritWorkflowColumn {
	var workflowvalue GerritWorkflowColumn
	switch value {
	case "1":
		workflowvalue = GerritWorkflowPlusOne
	case "0":
		workflowvalue = GerritWorkflowZero
	case "-1":
		workflowvalue = GerritWorkflowMinusOne
	default:
		workflowvalue = GerritWorkflowZero
	}
	return workflowvalue
}

const (
	GerritWorkflowPlusOne  = 1
	GerritWorkflowZero     = 0
	GerritWorkflowMinusOne = -1
)

type GerritWorkflowColumn int8

func GetGerritCodeReviewValue(value string) GerritCodeReview {
	var codereviewvalue GerritCodeReview
	switch value {
	case "2":
		codereviewvalue = GerritCodeReviewPlusTwo
	case "1":
		codereviewvalue = GerritCodeReviewPlusOne
	case "0":
		codereviewvalue = GerritCodeReviewZero
	case "-1":
		codereviewvalue = GerritCodeReviewMinusOne
	case "-2":
		codereviewvalue = GerritCodeReviewMinusTwo
	default:
		codereviewvalue = GerritWorkflowZero
	}
	return codereviewvalue
}

const (
	GerritCodeReviewPlusTwo  = 2
	GerritCodeReviewPlusOne  = 1
	GerritCodeReviewZero     = 0
	GerritCodeReviewMinusOne = -1
	GerritCodeReviewMinusTwo = -2
)

type GerritCodeReview int8

type PipelineRequireGerritApproval struct {
	Workflow   GerritWorkflowColumn `yaml:"Workflow,omitempty"`
	Verified   GerritReporter       `yaml:"Verified,omitempty,flow"`
	CodeReview GerritCodeReview     `yaml:"Code-Review,omitempty"`
}

type PipelineRequireApproval struct {
	Username        string                          `yaml:"username,omitempty"`
	Open            bool                            `yaml:"open,omitempty"`
	CurrentPatchset bool                            `yaml:"current-patchset,omitempty"`
	Verified        []GerritReporter                `yaml:"Verified,omitempty,flow"`
	GerritApproval  []PipelineRequireGerritApproval `yaml:"approval,omitempty"`
	Workflow        GerritWorkflowColumn            `yaml:"Workflow,omitempty"`
}

type PipelineTriggerGitGerrit struct {
	Approval []PipelineRequireApproval `yaml:"approval,omitempty"`
}

type PipelineTriggerGit struct {
	Event    string                          `yaml:"event"`
	Comment  string                          `yaml:"comment,omitempty"`
	Gerrit   PipelineTriggerGitGerrit        `yaml:"require,omitempty"`
	Ref      []string                        `yaml:"ref,omitempty"`
	Approval []PipelineRequireGerritApproval `yaml:"approval,omitempty"`
	Username string                          `yaml:"username,omitempty"`
}

type PipelineTriggerGitArray []PipelineTriggerGit

type PipelineGenericTrigger map[string]PipelineTriggerGitArray

type PipelineGerritReporter struct {
	Submit   bool           `yaml:"submit,omitempty"`
	Verified GerritReporter `yaml:"Verified"`
}

type GerritReporterMap map[string]PipelineGerritReporter
type PipelineReporter struct {
	GerritReporter GerritReporterMap `yaml:",inline"`
	SQLReporter    []string          `yaml:"sqlreporter,omitempty"`
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
	Name           string                 `yaml:"name"`
	Description    string                 `yaml:"description,omitempty"`
	Manager        PipelineManager        `yaml:"manager"`
	PostReview     bool                   `yaml:"post-review,omitempty"`
	Precedence     PipelinePrecedence     `yaml:"precedence,omitempty"`
	Supercedes     []string               `yaml:"supercedes,omitempty"`
	SuccessMessage string                 `yaml:"success-message,omitempty"`
	FailureMessage string                 `yaml:"failure-message,omitempty"`
	Require        PipelineRequire        `yaml:"require,omitempty"`
	Start          PipelineReporter       `yaml:"start,omitempty"`
	Success        PipelineReporter       `yaml:"success,omitempty"`
	Failure        PipelineReporter       `yaml:"failure,omitempty"`
	Trigger        PipelineGenericTrigger `yaml:"trigger,omitempty"`
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
