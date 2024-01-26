/*
Copyright © 2024 Redhat
*/

// Package microshift provides tools to deploy a MicroShift host via Ansible
package microshift

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/playbook"
	ctrl "sigs.k8s.io/controller-runtime"

	cutils "github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

//go:embed static/all.yaml.tmpl
var groupvars string

//go:embed static/inventory.yaml.tmpl
var inventory string

// playbooks
//
//go:embed static/ansible-microshift-role.yaml
var ansibleMicroshiftRoleYaml string

//go:embed static/local-setup.yaml
var localSetupYaml string

//go:embed static/deploy-microshift.yaml
var deployMicroshiftYaml string

//go:embed static/post-install.yaml
var postInstallYaml string

type InventoryData struct {
	User       string
	Host       string
	PullSecret string
}

type GroupVarsData struct {
	FQDN               string
	DiskFileSize       string
	MicroshiftRolePath string
}

type PlayBook struct {
	Path     string
	Contents string
}

var microshiftPlaybooks = map[string]PlayBook{
	"ansible-microshift-role": PlayBook{
		"ansible-microshift-role.yaml",
		ansibleMicroshiftRoleYaml,
	},
	"local-setup": PlayBook{
		"local-setup.yaml",
		localSetupYaml,
	},
	"deploy-microshift": PlayBook{
		"deploy-microshift.yaml",
		deployMicroshiftYaml,
	},
	"post-install": PlayBook{
		"post-install.yaml",
		postInstallYaml,
	},
}

func mkTemporaryPlaybookFile(rootDir string, pb PlayBook) {
	var data []byte
	var filePath string
	data = []byte(pb.Contents)
	filePath = rootDir + "/" + pb.Path
	if err := os.WriteFile(filePath, data, 0755); err != nil {
		ctrl.Log.Error(err, "Failure writing playbook "+pb.Path)
		os.Exit(1)
		ctrl.Log.V(5).Info("Created playbook file " + filePath)
	}
}

func MkTemporaryInventoryFile(host string, user string, pullSecret string, rootDir string) string {
	invData := InventoryData{
		user,
		host,
		pullSecret,
	}
	filePath := rootDir + "/inventory.yaml"
	template, err := cutils.ParseString(inventory, invData)
	if err != nil {
		ctrl.Log.Error(err, "Failure populating inventory template")
		os.Exit(1)
	}
	data := []byte(template)
	if err := os.WriteFile(filePath, data, 0700); err != nil {
		ctrl.Log.Error(err, "Failure writing temporary inventory file")
		os.Exit(1)
	}
	ctrl.Log.V(5).Info("Created inventory file " + filePath)
	return filePath
}

func MkAnsiblePlaybookOptions(host string, user string, pullSecret string, rootDir string) playbook.AnsiblePlaybookOptions {
	inventoryFile := MkTemporaryInventoryFile(host, user, pullSecret, rootDir)
	return playbook.AnsiblePlaybookOptions{
		Inventory: inventoryFile,
	}
}

func MkTemporaryVarsFile(fqdn string, diskFileSize string, microshiftRolePath string, rootDir string) string {
	varsData := GroupVarsData{
		fqdn,
		diskFileSize,
		microshiftRolePath,
	}
	filePath := rootDir + "/all.yaml"
	template, err := cutils.ParseString(groupvars, varsData)
	if err != nil {
		ctrl.Log.Error(err, "Failure populating group vars template")
		os.Exit(1)
	}
	data := []byte(template)
	if err := os.WriteFile(filePath, data, 0755); err != nil {
		ctrl.Log.Error(err, "Failure writing temporary group vars file")
		os.Exit(1)
	}
	ctrl.Log.V(5).Info("Created group vars file " + filePath)
	return filePath
}

func mkAnsiblePlaybookCmd(playbookPath string, sfOperatorRepoPath string, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) *playbook.AnsiblePlaybookCmd {
	rolesPath := fmt.Sprintf("%s:%s", sfOperatorRepoPath, ansibleMicroshiftRepoPath)
	return &playbook.AnsiblePlaybookCmd{
		Exec: execute.NewDefaultExecute(
			execute.WithEnvVar("ANSIBLE_ROLES_PATH", rolesPath),
		),
		Playbooks: []string{playbookPath},
		Options:   &options,
	}
}

func runPB(pb PlayBook, rootDir, sfOperatorRepoPath, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) {
	pbCmd := mkAnsiblePlaybookCmd(
		rootDir+"/"+pb.Path,
		sfOperatorRepoPath,
		ansibleMicroshiftRepoPath,
		options,
	)
	ctrl.Log.Info(pbCmd.String())
	if err := pbCmd.Run(context.TODO()); err != nil {
		ctrl.Log.Error(err, "Error running "+pb.Path)
		os.Exit(1)
	}

}

func MkMicroshiftRoleSetupPlaybook(rootDir string) {
	mkTemporaryPlaybookFile(rootDir, microshiftPlaybooks["ansible-microshift-role"])
}

func RunMicroshiftRoleSetup(rootDir string, sfOperatorRepoPath string, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) {
	pb := microshiftPlaybooks["ansible-microshift-role"]
	runPB(pb, rootDir, sfOperatorRepoPath, ansibleMicroshiftRepoPath, options)
}

func MkLocalSetupPlaybook(rootDir string) {
	mkTemporaryPlaybookFile(rootDir, microshiftPlaybooks["local-setup"])
}

func RunLocalSetup(rootDir string, sfOperatorRepoPath string, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) {
	pb := microshiftPlaybooks["local-setup"]
	runPB(pb, rootDir, sfOperatorRepoPath, ansibleMicroshiftRepoPath, options)
}

func MkDeployMicroshiftPlaybook(rootDir string) {
	mkTemporaryPlaybookFile(rootDir, microshiftPlaybooks["deploy-microshift"])
}

func RunDeploy(rootDir string, sfOperatorRepoPath string, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) {
	pb := microshiftPlaybooks["deploy-microshift"]
	runPB(pb, rootDir, sfOperatorRepoPath, ansibleMicroshiftRepoPath, options)
}

func MkPostInstallPlaybook(rootDir string) {
	mkTemporaryPlaybookFile(rootDir, microshiftPlaybooks["post-install"])
}

func RunPostInstall(rootDir string, sfOperatorRepoPath string, ansibleMicroshiftRepoPath string, options playbook.AnsiblePlaybookOptions) {
	pb := microshiftPlaybooks["post-install"]
	runPB(pb, rootDir, sfOperatorRepoPath, ansibleMicroshiftRepoPath, options)
}

func CreateTempRootDir() string {
	rootDir, err := os.MkdirTemp("", "microshift_")
	if err != nil {
		ctrl.Log.Error(err, "Error creating temporary work directory")
		os.Exit(1)
	}
	return rootDir
}
