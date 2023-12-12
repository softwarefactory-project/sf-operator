// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/softwarefactory-project/sf-operator/cli/cmd"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	//+kubebuilder:scaffold:imports
)

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	return utils.GetEnvVarValue("WATCH_NAMESPACE")
}

func operatorCmd(kmd *cobra.Command, args []string) {
	cliCtx, err := cmd.GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
	metricsAddr, _ := kmd.Flags().GetString("metrics-bind-address")
	probeAddr, _ := kmd.Flags().GetString("health-probe-bind-address")
	enableLeaderElection, _ := kmd.Flags().GetBool("leader-elect")
	if ns == "" {
		var err error
		ns, err = getWatchNamespace()
		if err != nil {
			controllers.SetupLog.Info("Unable to get WATCH_NAMESPACE env, " +
				"the manager will watch and manage resources in all namespaces")
		} else {
			controllers.SetupLog.Info("Got WATCH_NAMESPACE env, " +
				"the manager will watch and manage resources in " + ns + " namespace")
		}
	}
	controllers.Main(ns, metricsAddr, probeAddr, enableLeaderElection, false)
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		ns                   string
		fqdn                 string
		cliContext           string
		configFile           string

		rootCmd = &cobra.Command{
			Short: "SF Operator CLI",
			Long:  `Multi-purpose command line utility related to sf-operator, SF instances management, and development tools.`,
		}

		operatorCmd = &cobra.Command{
			Use:   "operator",
			Short: "Start the SF Operator",
			Long:  `This command starts the sf-operator service locally, for the cluster defined in the current kube context. The SF CRDs must be installed on the cluster.`,
			Run:   operatorCmd,
		}
	)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&ns, "namespace", "n", "", "The namespace on which to perform actions.")
	rootCmd.PersistentFlags().StringVarP(&fqdn, "fqdn", "d", "", "The FQDN of the deployment (if no manifest is provided).")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "C", "", "Path to the CLI configuration file.")
	rootCmd.PersistentFlags().StringVarP(&cliContext, "context", "c", "", "Context to use in the configuration file. Defaults to the \"default-context\" value in the config file if set, or the first available context in the config file.")

	// Flags for the operator command
	operatorCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	operatorCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	operatorCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// Add sub commands
	rootCmd.AddCommand(operatorCmd)
	subcommands := []*cobra.Command{
		cmd.MkApplyCmd(),
	}
	for _, c := range subcommands {
		rootCmd.AddCommand(c)
	}

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	rootCmd.Execute()
}
