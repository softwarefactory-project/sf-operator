// Package bootstraptenantconfigrepo provides facilities for the sfconfig CLI
// Generates pipelines, jobs and playbooks for zuul
package bootstraptenantconfigrepo

import (
	"fmt"
	"os"
	"path/filepath"

	utils "github.com/softwarefactory-project/sf-operator/controllers/libs/zuulcf"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var zuuldropindir = "zuul.d"
var zuulplaybooks = "playbooks"

var zuulrootdir = ""

func createDirectoryStructure(path string) error {

	for _, dir := range []string{path, filepath.Join(path, zuulplaybooks), filepath.Join(path, zuuldropindir)} {
		_, err := os.Stat(dir)
		if err != nil && os.IsNotExist(err) {
			if err := os.Mkdir(dir, 0755); err != nil {
				return fmt.Errorf(err.Error())
			}
		}
	}

	return nil
}

func writeOrAppendFile[F any](filestructure F, path string) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		dataOutput, _ := yaml.Marshal(filestructure)
		if err := os.WriteFile(path, dataOutput, 0666); err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	if err == nil {
		dataOutput, _ := yaml.Marshal(filestructure)
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
		if _, err := file.WriteString(string(dataOutput)); err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	return nil
}

func zuulWriteToFile[F utils.TenantConfig | utils.ProjectConfig | utils.PipelineConfig | utils.JobConfig](filestructure F, path string) error {
	return writeOrAppendFile[F](filestructure, path)
}

func ansibleWriteToFile[F utils.AnsiblePlayBook](filestructure F, path string) error {
	return writeOrAppendFile[F](filestructure, path)
}

func createZuulPipelineFile(pipelines utils.PipelineConfig, path string) error {
	return zuulWriteToFile[utils.PipelineConfig](pipelines, path)
}

func createZuulJobFile(jobs utils.JobConfig, path string) error {
	return zuulWriteToFile[utils.JobConfig](jobs, path)
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
	Use:   "bootstrap-tenant-config-repo",
	Short: "Zuul Config Generate command",
	Long: `Zuul Config Generate command
Expands sfconfig command tool
Generates Base Zuul Configurations

This will generate the the following files:
<Path to Project>/zuul.d/<CONNECTION NAME>-base-jobs.yaml
<Path to Project>/zuul.d/<CONNECTION NAME>-pipeline.yaml
<Path to Project>/playbooks/<CONNECTION NAME>-pre.yaml

Note: If the directories does not exit they will be created

	`,
	Example: `
	./tools/sfconfig bootstrap-tenant-config-repo --connection gerrit-conn --driver gerrit --outpath <Path to Project>/
	./tools/sfconfig bootstrap-tenant-config-repo --connection github-conn --driver github --outpath <Path to Project>/
	`,
	Aliases: []string{"boot"},
	Run: func(cmd *cobra.Command, args []string) {

		connection, _ := cmd.Flags().GetString("connection")
		driver, _ := cmd.Flags().GetString("driver")
		outpath, _ := cmd.Flags().GetString("outpath")

		zuulrootdir = outpath

		if err := createDirectoryStructure(zuulrootdir); err != nil {
			fmt.Println(err)
		}

		// Check Pipeline
		requireCheck, err := utils.GetRequireCheckByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		triggerCheck, err := utils.GetTriggerCheckByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		reportersCheck, err := utils.GetReportersCheckByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		// Gate Pipeline
		requireGate, err := utils.GetRequireGateByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		triggerGate, err := utils.GetTriggerGateByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		reportersGate, err := utils.GetReportersGateByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}
		// Post Pipeline
		triggerPost, err := utils.GetTriggerPostByDriver(driver, connection)
		if err != nil {
			fmt.Println(err)
		}

		zuulpipelinefilepath := filepath.Join(zuulrootdir, zuuldropindir, connection+"-pipeline.yaml")
		if err := createZuulPipelineFile(utils.PipelineConfig{
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
		}, zuulpipelinefilepath); err != nil {
			fmt.Println(err)
		}

		zuuljobfilepath := filepath.Join(zuulrootdir, zuuldropindir, connection+"-base-jobs.yaml")
		if err := createZuulJobFile(utils.JobConfig{
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
		}, zuuljobfilepath); err != nil {
			fmt.Println(err)
		}

		zuuljobplaybookfilepath := filepath.Join(zuulrootdir, zuulplaybooks, connection+"-pre.yaml")
		if err := ansibleWriteToFile(utils.AnsiblePlayBook{
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
						"when": `'ansible_connection != "kubectl"'`,
					},
					{
						"block": []map[string]any{
							getAnsibleIncludeRole("prepare-workspace-openshift"),
							getAnsibleIncludeRole("remove-zuul-sshkey"),
						},
						"run_once": true,
						"when":     "ansible_connection != 'kubectl'",
					},
					{
						"import_role": map[string]string{
							"name": "ensure-output-dirs",
						},
						"when": "ansible_user_dir is defined",
					},
				},
			},
		}, zuuljobplaybookfilepath); err != nil {
			fmt.Println(err)
		}

		fmt.Println("Files generated at ", outpath)
	},
}

func init() {
	BootstrapTenantConfigRepoCmd.Flags().String("connection", "", "Name of the connection or a source")
	BootstrapTenantConfigRepoCmd.MarkFlagRequired("connection")
	BootstrapTenantConfigRepoCmd.Flags().String("driver", "", "Driver type of the connection")
	BootstrapTenantConfigRepoCmd.MarkFlagRequired("driver")
	BootstrapTenantConfigRepoCmd.Flags().String("outpath", "", "Path to create file structure")
	BootstrapTenantConfigRepoCmd.MarkFlagRequired("outpath")
}
