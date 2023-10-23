// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

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
	ns, _ := cmd.Flags().GetString("ns")
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

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		ns                   string

		rootCmd = &cobra.Command{
			Use:   "sf-operator",
			Short: "Start the SF Operator",
			Long:  `The root command starts the sf-operator using the controller-runtime.`,
			Run:   rootCmd,
		}
	)

	rootCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	rootCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	rootCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	rootCmd.Flags().StringVar(&ns, "namespace", "", "The namespace to listen to.")

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	rootCmd.Execute()
}
