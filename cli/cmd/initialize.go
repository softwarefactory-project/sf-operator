/*
Copyright © 2023 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	controllerutils "github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var initAllowedArgs = []string{"config", "manifest"}

type SFManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec sfv1.SoftwareFactorySpec `json:"spec"`
}

func initializeSFManifest(withAuth bool, withBuilder bool, full bool, connections []string) {
	var manifest SFManifest
	oneGi := controllerutils.Qty1Gi()

	manifest.APIVersion = "sf.softwarefactory-project.io/v1"
	manifest.Kind = "SoftwareFactory"
	manifest.Metadata.Name = "my-sf"

	manifest.Spec.ConfigRepositoryLocation.BaseURL = "http://" + connections[0]
	manifest.Spec.ConfigRepositoryLocation.Name = "config"
	manifest.Spec.ConfigRepositoryLocation.ZuulConnectionName = connections[0]

	manifest.Spec.Logserver.LoopDelay = 3600
	manifest.Spec.Logserver.RetentionDays = 15
	manifest.Spec.Logserver.Storage.Size = oneGi

	manifest.Spec.Zookeeper.Storage.Size = oneGi

	gerritConnection := sfv1.GerritConnection{
		Name:     "gerrit",
		Hostname: "gerrit",
	}
	gitConnection := sfv1.GitConnection{
		Name:      "git",
		Baseurl:   "https://git",
		PollDelay: 300,
	}
	gitlabConnection := sfv1.GitLabConnection{
		Name: "gitlab",
	}
	githubConnection := sfv1.GitHubConnection{
		Name: "github",
	}
	pagureConnection := sfv1.PagureConnection{
		Name: "pagure",
	}
	if withAuth {
		oidcAuth := sfv1.ZuulOIDCAuthenticatorSpec{
			Name:     "zuulAuth",
			Realm:    "zuul",
			ClientID: "zuul-clientid",
			IssuerID: "iss",
		}
		manifest.Spec.Zuul.OIDCAuthenticators = []sfv1.ZuulOIDCAuthenticatorSpec{oidcAuth}
		manifest.Spec.Zuul.DefaultAuthenticator = "zuulAuth"
	}
	manifest.Spec.Zuul.Executor.LogLevel = "INFO"
	manifest.Spec.Zuul.Executor.Storage.Size = oneGi
	for _, co := range connections {
		if co == "gerrit" {
			manifest.Spec.Zuul.GerritConns = []sfv1.GerritConnection{gerritConnection}
		} else if co == "git" {
			manifest.Spec.Zuul.GitConns = []sfv1.GitConnection{gitConnection}
		} else if co == "gitlab" {
			manifest.Spec.Zuul.GitLabConns = []sfv1.GitLabConnection{gitlabConnection}
		} else if co == "github" {
			manifest.Spec.Zuul.GitHubConns = []sfv1.GitHubConnection{githubConnection}
		} else if co == "pagure" {
			manifest.Spec.Zuul.PagureConns = []sfv1.PagureConnection{pagureConnection}
		} else {
			ctrl.Log.Info("Unknown connection " + co + ", skipping")
		}
	}

	manifest.Spec.FQDN = "sfop.me"
	manifest.Spec.StorageClassName = "topolvm-provisioner"

	if full {
		fbSpec := sfv1.FluentBitForwarderSpec{
			HTTPInputHost: "fluentbit",
			HTTPInputPort: 5140,
		}
		manifest.Spec.FluentBitLogForwarding = &fbSpec

		manifest.Spec.GitServer.Storage.Size = oneGi

		leSpec := sfv1.LetsEncryptSpec{
			Server: sfv1.LEServerStaging,
		}
		manifest.Spec.LetsEncrypt = &leSpec

		manifest.Spec.MariaDB.DBStorage.Size = oneGi
		manifest.Spec.MariaDB.LogStorage.Size = oneGi

		esConnection := sfv1.ElasticSearchConnection{
			Name: "example-es",
			URI:  "https://example-es",
		}
		manifest.Spec.Zuul.ElasticSearchConns = []sfv1.ElasticSearchConnection{esConnection}

		if withBuilder {
			manifest.Spec.Nodepool.Builder.LogLevel = "INFO"
			manifest.Spec.Nodepool.Builder.Storage.Size = oneGi
		}
		manifest.Spec.Nodepool.Launcher.LogLevel = "INFO"

		manifest.Spec.Zuul.Merger.LogLevel = "INFO"
		manifest.Spec.Zuul.Merger.Storage.Size = oneGi

		manifest.Spec.Zuul.Scheduler.LogLevel = "INFO"
		manifest.Spec.Zuul.Scheduler.Storage.Size = oneGi
		manifest.Spec.Zuul.Web.LogLevel = "INFO"
	}

	yamlData, err := yaml.Marshal(manifest)
	if err != nil {
		ctrl.Log.Error(err, "Could not serialize sample manifest")
		os.Exit(1)
	}
	fmt.Println(string(yamlData))
}

func initializeCLIConfig(isDevEnv bool) {
	var defaultContextConfig cliutils.SoftwareFactoryConfigContext

	defaultContextConfig.ConfigRepository = "/path/to/config-repo"
	defaultContextConfig.Manifest = "/path/to/manifest"
	defaultContextConfig.IsStandalone = false
	defaultContextConfig.Namespace = "sf"
	defaultContextConfig.KubeContext = "microshift"
	defaultContextConfig.FQDN = "sfop.me"
	defaultContextConfig.Components.Nodepool.CloudsFile = "/path/to/clouds.yaml"
	defaultContextConfig.Components.Nodepool.KubeFile = "/path/to/kube.config"

	if isDevEnv {
		defaultContextConfig.Dev.AnsibleMicroshiftRolePath = "/path/to/ansible-microshift-role"
		defaultContextConfig.Dev.SFOperatorRepositoryPath = "/path/to/sf-operator"
		defaultContextConfig.Dev.Microshift.Host = "microshift.dev"
		defaultContextConfig.Dev.Microshift.User = "cloud-user"
		defaultContextConfig.Dev.Microshift.OpenshiftPullSecret = "PULL SECRET"
		defaultContextConfig.Dev.Microshift.DiskFileSize = "30G"
		defaultContextConfig.Dev.Tests.ExtraVars = map[string]string{"foo": "bar"}
	}

	contexts := cliutils.SoftwareFactoryConfig{
		Contexts: map[string]cliutils.SoftwareFactoryConfigContext{
			"my-context": defaultContextConfig,
		},
		Default: "my-context",
	}

	yamlData, err := yaml.Marshal(contexts)
	if err != nil {
		ctrl.Log.Error(err, "Could not serialize sample config")
		os.Exit(1)
	}
	fmt.Println(string(yamlData))
}

func initialize(kmd *cobra.Command, args []string) {
	if args[0] == "config" {
		isDevEnv, _ := kmd.Flags().GetBool("dev")
		initializeCLIConfig(isDevEnv)
	} else if args[0] == "manifest" {
		withAuth, _ := kmd.Flags().GetBool("with-auth")
		withBuilder, _ := kmd.Flags().GetBool("with-builder")
		minimal, _ := kmd.Flags().GetBool("minimal")
		connections, _ := kmd.Flags().GetStringSlice("connection")
		initializeSFManifest(withAuth, withBuilder, minimal, connections)
	} else {
		ctrl.Log.Error(errors.New("argument must be in: "+strings.Join(initAllowedArgs, ", ")), "Incorrect target "+args[0])
		os.Exit(1)
	}
}

func MkInitCmd() *cobra.Command {
	var (
		isDevEnv    bool
		withAuth    bool
		withBuilder bool
		full        bool
		connections []string
		initCmd     = &cobra.Command{
			Use:       "init {config,manifest}",
			Short:     "CLI/project initialization subcommands",
			Long:      "These subcommands can be used to generate a sample CLI context, or a sample Software Factory manifest.",
			ValidArgs: initAllowedArgs,
			Run:       initialize,
		}
	)

	initCmd.Flags().BoolVar(&isDevEnv, "dev", false, "(config) include development-related config parameters")
	initCmd.Flags().BoolVar(&withAuth, "with-auth", false, "(manifest) include OIDC authentication section")
	initCmd.Flags().BoolVar(&withBuilder, "with-builder", false, "(manifest) include nodepool builder section")
	initCmd.Flags().BoolVar(&full, "full", false, "(manifest) return a manifest with optional parameters and entries")
	initCmd.Flags().StringSliceVar(&connections, "connection", []string{"gerrit"}, "(manifest) include connection (valid connections are gerrit, github, git, gitlab, pagure). The first connection will be assumed to host the config repo.")
	return initCmd
}
