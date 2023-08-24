// Package utils provides utility functions for the CLI
package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	apiroutev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	opv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

type ENV struct {
	Cli client.Client
	Ns  string
	Ctx context.Context
}

// RunMake is a temporary hack until make target are implemented natively
func RunMake(arg string) {
	RunCmd("make", arg)
}

func RunCmd(cmdName string, args ...string) {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("%s failed: %w", args, err))
	}
}

func EnsureServiceAccount(env *ENV, name string) {
	var sa apiv1.ServiceAccount
	if !GetM(env, name, &sa) {
		sa.Name = name
		CreateR(env, &sa)
	}
}

func RenderYAML(o interface{}) string {
	y, err := yaml.Marshal(o)
	if err != nil {
		panic(fmt.Errorf("err: %v", err))
	}
	return string(y)
}

func GetConfigContextOrDie(contextName string) *rest.Config {
	var conf *rest.Config
	var err error
	if conf, err = config.GetConfigWithContext(contextName); err != nil {
		panic(fmt.Errorf("couldn't find context %s: %s", contextName, err))
	}
	return conf
}

func CreateKubernetesClient(contextName string) (client.Client, error) {
	scheme := runtime.NewScheme()
	monitoring.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiroutev1.AddToScheme(scheme))
	utilruntime.Must(opv1.AddToScheme(scheme))
	utilruntime.Must(sfv1.AddToScheme(scheme))
	var conf *rest.Config
	if contextName != "" {
		conf = GetConfigContextOrDie(contextName)
	} else {
		conf = config.GetConfigOrDie()
	}
	return client.New(conf, client.Options{
		Scheme: scheme,
	})
}

func CreateKubernetesClientOrDie(contextName string) client.Client {
	cli, err := CreateKubernetesClient(contextName)
	if err != nil {
		fmt.Println("failed to create client", err)
		os.Exit(1)
	}
	return cli
}

// ParseString allows to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func ParseString(text string, data any) (string, error) {

	template.New("StringtoParse").Parse(text)
	// Opening Template file
	template, err := template.New("StringtoParse").Parse(text)
	if err != nil {
		return "", fmt.Errorf("Text not in the right format: " + text)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", text)
	}

	return buf.String(), nil
}

// GetM is an helper to fetch a kubernetes resource by name, returns true when it is found.
func GetM(env *ENV, name string, obj client.Object) bool {
	err := env.Cli.Get(env.Ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: env.Ns,
		}, obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(fmt.Errorf("could not get %s: %s", name, err))
	}
	return true
}

// CreateR is an helper to create a kubernetes resource.
func CreateR(env *ENV, obj client.Object) {
	fmt.Fprintf(os.Stderr, "Creating %s in %s\n", obj.GetName(), env.Ns)
	obj.SetNamespace(env.Ns)
	if err := env.Cli.Create(env.Ctx, obj); err != nil {
		panic(fmt.Errorf("could not create %s: %s", obj, err))
	}
}

// UpdateR is an helper to update a kubernetes resource.
func UpdateR(env *ENV, obj client.Object) bool {
	fmt.Fprintf(os.Stderr, "Updating %s in %s\n", obj.GetName(), env.Ns)
	if err := env.Cli.Update(env.Ctx, obj); err != nil {
		panic(fmt.Errorf("could not update %s: %s", obj, err))
	}
	return true
}

func CreateTempPlaybookFile(content string) (*os.File, error) {
	file, e := os.CreateTemp("playbooks", "sfconfig-operator-create-")
	if e != nil {
		panic(e)
	}
	fmt.Println("Temp file name:", file.Name())
	_, e = file.Write([]byte(content))
	if e != nil {
		panic(e)
	}
	e = file.Close()
	return file, e
}

func RemoveTempPlaybookFile(file *os.File) {
	defer os.Remove(file.Name())
}

func GetSF(env *ENV, name string) (sfv1.SoftwareFactory, error) {
	var sf sfv1.SoftwareFactory
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: env.Ns,
		Name:      name,
	}, &sf)
	return sf, err
}

func IsCRDMissing(err error) bool {
	// FIXME: replace stringly check with something more solid?
	return strings.Contains(err.Error(), `no matches for kind "SoftwareFactory"`) ||
		// This case is encountered when make install has not been run prior
		strings.Contains(err.Error(), `sf.softwarefactory-project.io/v1: the server could not find the requested resource`)
}

func IsCertManagerRunning(env *ENV) bool {
	var dep appsv1.Deployment
	env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: "operators",
		Name:      "cert-manager-webhook",
	}, &dep)
	return dep.Status.ReadyReplicas >= 1
}

func GetSecret(env *ENV, name string) []byte {
	var secret apiv1.Secret
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: env.Ns,
		Name:      name,
	}, &secret)
	if err != nil {
		panic(err)
	}
	return secret.Data[name]
}

func GetFileContent(filePath string) ([]byte, error) {
	if _, err := os.Stat(filePath); err == nil {
		if data, err := os.ReadFile(filePath); err == nil {
			return data, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func GetKubernetesClientSet() (*rest.Config, *kubernetes.Clientset) {

	kubeConfig := config.GetConfigOrDie()

	// create the kubernetes Clientset
	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err.Error())
	}
	return kubeConfig, kubeClientset
}

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
	templateConfig, err := ParseString(TenantTemplate, tenant)

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

type JobBody struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description,omitempty"`
	ExtraVars   map[string]interface{} `yaml:"extra-vars,omitempty"`
	Parent      *string                `yaml:"parent"`
	PostRun     []string               `yaml:"post-run,omitempty"`
	PreRun      []string               `yaml:"pre-run,omitempty"`
	Roles       []JobRoles             `yaml:"roles,omitempty"`
	Secrets     JobSecrets             `yaml:"secrets,omitempty"`
	Timeout     uint16                 `yaml:"timeout,omitempty"`
	Attempts    uint8                  `yaml:"attempts,omitempty"`
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
