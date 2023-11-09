// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"

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

func rootCmd(cmd *cobra.Command, args []string) {
	ns, _ := cmd.Flags().GetString("namespace")
	metricsAddr, _ := cmd.Flags().GetString("metrics-bind-address")
	probeAddr, _ := cmd.Flags().GetString("health-probe-bind-address")
	enableLeaderElection, _ := cmd.Flags().GetBool("leader-elect")
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

func standaloneCmd(cmd *cobra.Command, args []string) {
	ns, _ := cmd.Flags().GetString("namespace")
	sfResource, _ := cmd.Flags().GetString("cr")
	ctrl.Log.Info("Running standalone mode into " + ns)
	if (sfResource != "" && ns == "") || (sfResource == "" && ns != "") {
		ctrl.Log.Info("When running the standalon mode, both --cr and --namespace option must be set")
		os.Exit(1)
	} else if sfResource != "" && ns != "" {
		var sf sfv1.SoftwareFactory
		dat, err := os.ReadFile(sfResource)
		if err != nil {
			panic(err.Error())
		}
		if err := yaml.Unmarshal(dat, &sf); err != nil {
			panic(err.Error())
		}
		ctrl.Log.Info("Standalone reconciling from CR passed by parameter",
			"CR", sf,
			"CR name", sf.ObjectMeta.Name,
			"NS", ns)
		controllers.Standalone(sf, ns)
		os.Exit(0)
	}
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		ns                   string
		sfResource           string

		rootCmd = &cobra.Command{
			Use:   "operator",
			Short: "Start the SF Operator as normal mode",
			Long:  `This command starts the sf-operator using the controller-runtime.`,
			Run:   rootCmd,
		}

		standaloneCmd = &cobra.Command{
			Use:   "standalone",
			Short: "Start the SF Operator as standalone mode",
			Long:  `This command starts the sf-operator using the go client. This mode does not expect CRs to be installed on the cluster.`,
			Run:   standaloneCmd,
		}
	)

	// Flags for the root command
	rootCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	rootCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	rootCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	rootCmd.Flags().StringVar(&ns, "namespace", "", "The namespace to listen to.")

	// Flags for the standalone command
	standaloneCmd.Flags().StringVar(&ns, "namespace", "sf", "The namespace where to reconcile the deployment.")
	standaloneCmd.Flags().StringVar(&sfResource, "cr", "", "The path to the CR to reconcile.")

	// Add sub commands
	rootCmd.AddCommand(standaloneCmd)

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	rootCmd.Execute()
}
