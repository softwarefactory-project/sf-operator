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
	"fmt"
	"os"

	"github.com/softwarefactory-project/sf-operator/cli"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/gerrit"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/operator"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/sf"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sfconfig",
	Short: "sfconfig cli tool",
	Long: `sfconfig command line tool
	This tool is used to deploy and run tests for sf-operator`,
	Run: func(cmd *cobra.Command, args []string) {
		erase, _ := cmd.Flags().GetBool("erase")
		cli.Run(erase)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $PWD/sfconfig.yaml)")

	rootCmd.AddCommand(operator.OperatorCmd)
	rootCmd.AddCommand(sf.SfCmd)
	rootCmd.AddCommand(gerrit.GerritCmd)
	rootCmd.Flags().BoolP("erase", "", false, "Erase data")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		currentDir, err := os.Getwd()
		cobra.CheckErr(err)

		// Search config in home directory with name "sfconfig" (without extension).
		viper.AddConfigPath(currentDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("sfconfig")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	if cfgFile == "" {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
