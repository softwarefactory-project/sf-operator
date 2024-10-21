// Package bootstraptenantconfigrepo provides facilities for the sfconfig CLI
// Generates pipelines, jobs and playbooks for zuul
package bootstraptenantconfigrepo

import (
	"errors"
	"os"
	"path/filepath"

	utils "github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	ctrl "sigs.k8s.io/controller-runtime"
)

var zuuldropindir = "zuul.d"
var zuulplaybooks = "playbooks"

func createDirectoryStructureOrDie(path string) {

	for _, dir := range []string{path, filepath.Join(path, zuulplaybooks), filepath.Join(path, zuuldropindir)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			ctrl.Log.Error(err, "Unable to create directory structure")
			os.Exit(1)
		}
	}
}

func writeFileOrDie[F any](filestructure F, path string) {
	dataOutput, _ := yaml.Marshal(filestructure)
	if err := os.WriteFile(path, dataOutput, 0666); err != nil {
		ctrl.Log.Error(err, "Unable to write file "+path)
	}
}

func getAnsibleIncludeRole(rolename string) map[string]any {
	return map[string]any{
		"import_role": map[string]string{
			"name": rolename,
		},
	}
}

// BootstrapTenantConfigRepoCmd command
var BootstrapTenantConfigRepoCmd = &cobra.Command{
	Use:   "bootstrap-tenant <path to directory>",
	Short: "bootstrap a tenant's config repository",
	Long: `Initialize a Zuul tenant's config repository
with boilerplate code that define standard pipelines:

* "check" for pre-commit validation
* "gate" for approved commits gating
* "post for post-commit actions

it also includes a boilerplate job and pre-run playbook.

This will generate the following files:
<Path to Project>/zuul.d/<CONNECTION NAME>-base-jobs.yaml
<Path to Project>/zuul.d/<CONNECTION NAME>-pipeline.yaml
<Path to Project>/playbooks/<CONNECTION NAME>-pre.yaml

Note: If the directories does not exit they will be created.

	`,
	Run: func(cmd *cobra.Command, args []string) {

		connection, _ := cmd.Flags().GetString("connection")
		driver, _ := cmd.Flags().GetString("driver")

		if len(args) != 1 {
			ctrl.Log.Error(errors.New("incorrect argument"), "the command accepts only one argument as destination path")
			os.Exit(1)
		}
		outpath := args[0]

		InitConfigRepo(driver, connection, outpath)

		ctrl.Log.Info("Repository bootstrapped at " + outpath)
	},
}

func InitConfigRepo(driver string, connection string, zuulrootdir string) {

	createDirectoryStructureOrDie(zuulrootdir)

	// Check Pipeline
	requireCheck, err := utils.GetRequireCheckByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define check pipeline require config for "+driver)
		os.Exit(1)
	}
	triggerCheck, err := utils.GetTriggerCheckByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define check pipeline trigger config for "+driver)
		os.Exit(1)
	}
	reportersCheck, err := utils.GetReportersCheckByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define check pipeline reporters config for "+driver)
		os.Exit(1)
	}
	// Gate Pipeline
	requireGate, err := utils.GetRequireGateByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define gate pipeline require config for "+driver)
		os.Exit(1)
	}
	triggerGate, err := utils.GetTriggerGateByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define gate pipeline trigger config for "+driver)
		os.Exit(1)
	}
	reportersGate, err := utils.GetReportersGateByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define gate pipeline trigger config for "+driver)
		os.Exit(1)
	}
	// Post Pipeline
	triggerPost, err := utils.GetTriggerPostByDriver(driver, connection)
	if err != nil {
		ctrl.Log.Error(err, "Could not define post pipeline trigger config for "+driver)
		os.Exit(1)
	}

	zuulpipelinefilepath := filepath.Join(zuulrootdir, zuuldropindir, connection+"-pipeline.yaml")
	writeFileOrDie(utils.PipelineConfig{
		{
			Pipeline: utils.PipelineBody{
				Name: "check",
				Description: `Newly uploaded patchsets enter this
pipeline to receive an initial +/-1 Verified vote.`,
				Manager: utils.Independent,
				Require: requireCheck,
				Trigger: triggerCheck,
				Start:   reportersCheck[0],
				Success: reportersCheck[1],
				Failure: reportersCheck[2],
			},
		},
		{
			Pipeline: utils.PipelineBody{
				Name:           "gate",
				Description:    `Changes that have been approved by core developers are enqueued in order in this pipeline, and if they pass tests, will be merged.`,
				SuccessMessage: "Build succeeded (gate pipeline).",
				FailureMessage: "Build failed (gate pipeline).",
				Precedence:     utils.GetZuulPipelinePrecedence("high"),
				Supercedes:     []string{"check"},
				PostReview:     true,
				Manager:        utils.Dependent,
				Require:        requireGate,
				Trigger:        triggerGate,
				Start:          reportersGate[0],
				Success:        reportersGate[1],
				Failure:        reportersGate[2],
			},
		},
		{
			Pipeline: utils.PipelineBody{
				Name:        "post",
				PostReview:  true,
				Description: `This pipeline runs jobs that operate after each change is merged.`,
				Manager:     utils.Supercedent,
				Precedence:  utils.GetZuulPipelinePrecedence("low"),
				Trigger:     triggerPost,
			},
		},
	}, zuulpipelinefilepath)

	zuuljobfilepath := filepath.Join(zuulrootdir, zuuldropindir, connection+"-base-jobs.yaml")
	writeFileOrDie(utils.JobConfig{
		{
			Job: utils.JobBody{
				Name:        "base",
				Description: "The base job.",
				Parent:      nil,
				PreRun: []string{
					"playbooks/" + connection + "-pre.yaml",
				},
				Roles: []utils.JobRoles{
					map[string]string{
						"zuul": "zuul/zuul-jobs",
					},
				},
				Timeout:  1800,
				Attempts: 3,
			},
		},
	}, zuuljobfilepath)

	zuuljobplaybookfilepath := filepath.Join(zuulrootdir, zuulplaybooks, connection+"-pre.yaml")
	writeFileOrDie(utils.AnsiblePlayBook{
		{
			Hosts: "localhost",
			Tasks: []map[string]any{
				{
					"block": []map[string]any{
						getAnsibleIncludeRole("emit-job-header"),
						getAnsibleIncludeRole("log-inventory"),
					},
				},
			},
		},
		{
			Hosts: "all",
			Tasks: []map[string]any{
				getAnsibleIncludeRole("start-zuul-console"),
				{
					"block": []map[string]any{
						getAnsibleIncludeRole("validate-host"),
						getAnsibleIncludeRole("prepare-workspace"),
						getAnsibleIncludeRole("add-build-sshkey"),
					},
					"when": "ansible_connection != 'kubectl'",
				},
				{
					"block": []map[string]any{
						getAnsibleIncludeRole("prepare-workspace-openshift"),
						getAnsibleIncludeRole("remove-zuul-sshkey"),
					},
					"run_once": true,
					"when":     "ansible_connection == 'kubectl'",
				},
				{
					"import_role": map[string]string{
						"name": "ensure-output-dirs",
					},
					"when": "ansible_user_dir is defined",
				},
			},
		},
	}, zuuljobplaybookfilepath)

	zuulppfilepath := filepath.Join(zuulrootdir, zuuldropindir, connection+"-project-pipeline.yaml")
	writeFileOrDie(utils.ProjectConfig{{
		Project: utils.ZuulProjectBody{
			Pipeline: utils.ZuulProjectPipelineMap{
				"check": utils.ZuulProjectPipeline{
					Jobs: []string{
						"noop",
					},
				},
				"gate": utils.ZuulProjectPipeline{
					Jobs: []string{
						"noop",
					},
				},
			},
		}}}, zuulppfilepath)
}

func MkBootstrapCmd() *cobra.Command {
	var (
		connection string
		driver     string
	)
	BootstrapTenantConfigRepoCmd.Flags().StringVar(&connection, "connection", "", "Name of the connection or a source")
	BootstrapTenantConfigRepoCmd.Flags().StringVar(&driver, "driver", "", "Driver type of the connection. Supported drivers: gitlab, gerrit")
	return BootstrapTenantConfigRepoCmd
}
