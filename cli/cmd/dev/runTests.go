/*
Copyright © 2024 Redhat
*/

package dev

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"

	ctrl "sigs.k8s.io/controller-runtime"
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
