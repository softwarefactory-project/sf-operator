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

// Package cmd provides subcommands for the main.go CLI
package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"
)

// CLI config struct
type SoftwareFactoryConfigContext struct {
	ConfigRepository string `mapstructure:"config-repository-path"`
	Manifest         string `mapstructure:"manifest-file"`
	IsStandalone     bool   `mapstructure:"standalone"`
	Namespace        string `mapstructure:"namespace"`
	KubeContext      string `mapstructure:"kube-context"`
	FQDN             string `mapstructure:"fqdn"`
	Dev              struct {
		AnsibleMicroshiftRolePath string `mapstructure:"ansible-microshift-role-path"`
		Microshift                struct {
			Host          string `mapstructure:"host"`
			User          string `mapstructure:"user"`
			InventoryFile string `mapstructure:"inventory-file"`
		} `mapstructure:"microshift"`
		Tests struct {
			ExtraVars map[string]string `mapstructure:"extra-vars"`
		} `mapstructure:"tests"`
	} `mapstructure:"development"`
	Components struct {
		Nodepool struct {
			CloudsFile string `mapstructure:"clouds-file"`
			KubeFile   string `mapstructure:"kube-file"`
		} `mapstructure:"nodepool"`
	} `mapstructure:"components"`
}

type SoftwareFactoryConfig struct {
	Contexts map[string]SoftwareFactoryConfigContext `mapstructure:"contexts"`
	Default  string                                  `mapstructure:"default-context"`
}

func loadConfigFile(command *cobra.Command) (cliConfig SoftwareFactoryConfig, err error) {
	configPath, _ := command.Flags().GetString("config")
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&cliConfig)
	return
}

func getContextFromFile(command *cobra.Command) (ctxName string, cliContext SoftwareFactoryConfigContext, err error) {
	cliConfig, err := loadConfigFile(command)
	if err != nil {
		return
	}
	ctx, _ := command.Flags().GetString("context")
	if ctx == "" {
		ctx = cliConfig.Default
	}
	for c := range cliConfig.Contexts {
		if ctx == "" || ctx == c {
			return c, cliConfig.Contexts[c], nil
		}
	}
	return ctxName, cliContext, errors.New("context not found")
}

func GetCLIContext(command *cobra.Command) (SoftwareFactoryConfigContext, error) {
	var cliContext SoftwareFactoryConfigContext
	var ctxName string
	var err error
	configPath, _ := command.Flags().GetString("config")
	if configPath != "" {
		ctxName, cliContext, err = getContextFromFile(command)
		if err != nil {
			ctrl.Log.Error(err, "Could not load config file")
		} else {
			ctrl.Log.Info("Using configuration context " + ctxName)
		}
	}
	// Override with defaults
	// We don't set a default namespace here so as not to interfere with rootcommand.
	ns, _ := command.Flags().GetString("namespace")
	if cliContext.Namespace == "" {
		cliContext.Namespace = ns
	}
	fqdn, _ := command.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}
	if cliContext.FQDN == "" {
		cliContext.FQDN = fqdn
	}
	return cliContext, nil
}
