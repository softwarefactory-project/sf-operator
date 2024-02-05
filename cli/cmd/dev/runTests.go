/*
Copyright © 2024 Redhat
*/

package dev

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	bootstraptenantconfigrepo "github.com/softwarefactory-project/sf-operator/cli/cmd/bootstrap-tenant-config-repo"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
)

var runTestsAllowedArgs = []string{"standalone", "olm", "upgrade"}

func mkTestPlaybook(vars map[string]string, sfOperatorRepoPath string, playbookName string, verbosity string) *playbook.AnsiblePlaybookCmd {

	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{}
	ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{}

	ansiblePlaybookOptions.AddExtraVar("hostname", "localhost")
	if verbosity == "verbose" {
		ansiblePlaybookOptions.VerboseV = true
	}
	if verbosity == "debug" {
		ansiblePlaybookOptions.VerboseVVVV = true
	}
	for keyV, valueV := range vars {
		ansiblePlaybookOptions.AddExtraVar(keyV, valueV)
	}

	pbFullPath := filepath.Join(sfOperatorRepoPath, playbookName)
	pb := &playbook.AnsiblePlaybookCmd{
		Exec: execute.NewDefaultExecute(
			execute.WithEnvVar("ANSIBLE_ROLES_PATH", sfOperatorRepoPath)),
		Playbooks:         []string{pbFullPath},
		Options:           ansiblePlaybookOptions,
		ConnectionOptions: ansiblePlaybookConnectionOptions,
	}
	return pb
}

func runPlaybook(pb *playbook.AnsiblePlaybookCmd) error {
	options.AnsibleForceColor()
	ctrl.Log.Info(pb.String())
	return pb.Run(context.TODO())
}

func runTestStandalone(extraVars map[string]string, sfOperatorRepoPath string, verbosity string) {
	pbName := "playbooks/main.yaml"
	pb := mkTestPlaybook(extraVars, sfOperatorRepoPath, pbName, verbosity)
	pb.Options.Tags = "standalone"
	pb.Options.AddExtraVar("mode", "standalone")
	if err := runPlaybook(pb); err != nil {
		ctrl.Log.Error(err, "Could not run standalone tests")
		os.Exit(1)
	}
}

func runTestOLM(extraVars map[string]string, sfOperatorRepoPath string, verbosity string) {
	pbName := "playbooks/main.yaml"
	pb := mkTestPlaybook(extraVars, sfOperatorRepoPath, pbName, verbosity)
	pb.Options.AddExtraVar("mode", "olm")
	if err := runPlaybook(pb); err != nil {
		ctrl.Log.Error(err, "Could not run OLM tests")
		os.Exit(1)
	}
}

func runTestUpgrade(extraVars map[string]string, sfOperatorRepoPath string, verbosity string) {
	pbName := "playbooks/upgrade.yaml"
	pb := mkTestPlaybook(extraVars, sfOperatorRepoPath, pbName, verbosity)
	if err := runPlaybook(pb); err != nil {
		ctrl.Log.Error(err, "Could not run upgrade tests")
		os.Exit(1)
	}
}

// Prepare test environment helper functions

func PushRepoIfNeeded(path string) {
	out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
	if err != nil {
		ctrl.Log.Error(err, "Could not fetch repo's status")
	}
	if len(out) > 0 {
		ctrl.Log.Info("Pushing local repo state to origin...")
		cliutils.RunCmdOrDie("git", "-C", path, "commit", "-m", "Automatic update", "-a")
		cliutils.RunCmdOrDie("git", "-C", path, "push", "origin")
	}
}

// ApplyCRDs assumes that "make manifest" was run prior to being invoked.
func ApplyCRDs(config *rest.Config, sfOperatorRepoPath string) {
	crdInstallOptions := envtest.CRDInstallOptions{
		Paths: []string{
			filepath.Join(sfOperatorRepoPath, "config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml"),
			filepath.Join(sfOperatorRepoPath, "config/crd/bases/sf.softwarefactory-project.io_logservers.yaml"),
		},
	}
	_, err := envtest.InstallCRDs(config, crdInstallOptions)
	if err != nil {
		ctrl.Log.Error(err, "Could not install CRDs")
	}
}

func SetupDemoConfigRepo(reposPath, zuulDriver, zuulConnection string, updateDemoTenantDefinition bool) {
	var (
		configRepoPath     = filepath.Join(reposPath, "config")
		demoConfigRepoPath = filepath.Join(reposPath, "demo-tenant-config")
	)
	// Setup demo-tenant-config
	ctrl.Log.Info("Initialize demo-tenant's pipelines and jobs... ")
	bootstraptenantconfigrepo.InitConfigRepo(zuulDriver, zuulConnection, demoConfigRepoPath)
	cliutils.RunCmdOrDie("git", "-C", demoConfigRepoPath, "add", "zuul.d/", "playbooks/")
	PushRepoIfNeeded(demoConfigRepoPath)
	// Update config if needed
	if updateDemoTenantDefinition {
		tenantDir := filepath.Join(configRepoPath, "zuul")
		if err := os.MkdirAll(tenantDir, 0755); err != nil {
			ctrl.Log.Error(err, "Could not create zuul dir in config repo")
			os.Exit(1)
		}
		tenantFile := filepath.Join(tenantDir, "main.yaml")

		tenantData := zuulcf.TenantConfig{
			{
				Tenant: zuulcf.TenantBody{
					Name: "demo-tenant",
					Source: zuulcf.TenantConnectionSource{
						"opendev.org": {
							UntrustedProjects: []string{"zuul/zuul-jobs"},
						},
						zuulConnection: {
							ConfigProjects:    []string{"demo-tenant-config"},
							UntrustedProjects: []string{"demo-project"},
						},
					},
				},
			},
		}

		templateDataOutput, _ := yaml.Marshal(tenantData)

		if err := os.WriteFile(tenantFile, []byte(templateDataOutput), 0644); err != nil {
			ctrl.Log.Error(err, "Could not write configuration to file")
			os.Exit(1)
		}
		ctrl.Log.Info("Creating or updating tenant demo-tenant... ")
		cliutils.RunCmdOrDie("git", "-C", configRepoPath, "add", "zuul/main.yaml")
		PushRepoIfNeeded(configRepoPath)
	}
}
