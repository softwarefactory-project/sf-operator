// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func loadConfigFile(cmd *cobra.Command) (cliConfig SoftwareFactoryConfig, err error) {
	configPath, _ := cmd.Flags().GetString("config")
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&cliConfig)
	return
}

func getContextFromFile(cmd *cobra.Command) (ctxName string, cliContext SoftwareFactoryConfigContext, err error) {
	cliConfig, err := loadConfigFile(cmd)
	if err != nil {
		return
	}
	ctx, _ := cmd.Flags().GetString("context")
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

// Parse arguments from config file and the command line.
// CLI arguments take precedence over config file.
func getCLIContext(cmd *cobra.Command) (SoftwareFactoryConfigContext, error) {
	var cliContext SoftwareFactoryConfigContext
	var ctxName string
	var err error
	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		ctxName, cliContext, err = getContextFromFile(cmd)
		if err != nil {
			ctrl.Log.Error(err, "Could not load config file")
		} else {
			ctrl.Log.Info("Using configuration context " + ctxName)
		}
	}
	// Override with defaults
	// We don't set a default namespace here so as not to interfere with rootCmd.
	ns, _ := cmd.Flags().GetString("namespace")
	if cliContext.Namespace == "" {
		cliContext.Namespace = ns
	}
	fqdn, _ := cmd.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}
	if cliContext.FQDN == "" {
		cliContext.FQDN = fqdn
	}
	return cliContext, nil
}

func operatorCmd(cmd *cobra.Command, args []string) {
	cliCtx, err := getCLIContext(cmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
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
	cliCtx, err := getCLIContext(cmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
	sfResource, _ := cmd.Flags().GetString("cr")
	hasManifest := &cliCtx.Manifest
	if sfResource == "" && hasManifest != nil {
		sfResource = cliCtx.Manifest
	}
	if (sfResource != "" && ns == "") || (sfResource == "" && ns != "") {
		err := errors.New("standalone mode requires both --cr and --namespace to be set")
		ctrl.Log.Error(err, "Argument error:")
		os.Exit(1)
	} else if sfResource != "" && ns != "" {
		var sf sfv1.SoftwareFactory
		dat, err := os.ReadFile(sfResource)
		if err != nil {
			ctrl.Log.Error(err, "Error reading manifest:")
			os.Exit(1)
		}
		if err := yaml.Unmarshal(dat, &sf); err != nil {
			ctrl.Log.Error(err, "Error interpreting the SF custom resource:")
			os.Exit(1)
		}
		ctrl.Log.Info("Applying custom resource with the following parameters:",
			"CR", sf,
			"CR name", sf.ObjectMeta.Name,
			"Namespace", ns)
		controllers.Standalone(sf, ns, cliCtx.KubeContext)
		os.Exit(0)
	}
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
		sfResource           string

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

		standaloneCmd = &cobra.Command{
			Use:   "apply",
			Short: "Apply a SoftwareFactory Custom Resource to a cluster",
			Long: `This command can be used to deploy a SoftwareFactory resource without installing the operator or its associated CRDs on a cluster.
			This will run the operator runtime locally, deploy the resource's components on the cluster, then exit.`,
			Run: standaloneCmd,
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

	// Flags for the standalone command
	standaloneCmd.Flags().StringVar(&sfResource, "cr", "", "The path to the CR to apply.")

	// Add sub commands
	rootCmd.AddCommand(standaloneCmd)
	rootCmd.AddCommand(operatorCmd)

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	rootCmd.Execute()
}
