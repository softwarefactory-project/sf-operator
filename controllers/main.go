// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	apiroutev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	//+kubebuilder:scaffold:imports
)

var setupLog = ctrl.Log.WithName("setup")
var SetupLog = setupLog
var controllerScheme = runtime.NewScheme()

func init() {
	InitScheme()
}

func InitScheme() *runtime.Scheme {
	utilruntime.Must(clientgoscheme.AddToScheme(controllerScheme))
	utilruntime.Must(sfv1.AddToScheme(controllerScheme))
	utilruntime.Must(apiroutev1.AddToScheme(controllerScheme))
	return controllerScheme
}

func ReadSFYAML(fp string) (sfv1.SoftwareFactory, error) {
	var sf sfv1.SoftwareFactory
	dat, err := os.ReadFile(fp)
	if err != nil {
		ctrl.Log.Error(err, "Error reading manifest:")
		return sf, err
	}
	if err := yaml.Unmarshal(dat, &sf); err != nil {
		ctrl.Log.Error(err, "Error interpreting the SF custom resource:")
		return sf, err
	}
	return sf, nil
}

func getPodRESTClient(config *rest.Config) rest.Interface {

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	httpCli, err := rest.HTTPClientFor(config)
	if err != nil {
		setupLog.Error(err, "unable to get HTTP client")
		os.Exit(1)
	}

	restClient, err := apiutil.RESTClientForGVK(gvk, false, config, serializer.NewCodecFactory(controllerScheme), httpCli)
	if err != nil {
		setupLog.Error(err, "unable to create REST client")
	}

	return restClient
}

func Main(ns string, metricsAddr string, probeAddr string, enableLeaderElection bool, oneShot bool) {
	newCache := func(config *rest.Config, opts cache.Options) (cache.Cache, error) { return cache.New(config, opts) }
	if ns != "" {
		newCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.DefaultNamespaces = map[string]cache.Config{ns: {}}
			return cache.New(config, opts)
		}
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		NewCache: newCache,
		Scheme:   controllerScheme,
		Metrics: metrics.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "01752ab0.softwarefactory-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	config := mgr.GetConfig()
	restClient := getPodRESTClient(config)

	baseCtx := ctrl.SetupSignalHandler()
	var cancelFunc context.CancelFunc
	var ctx context.Context
	if oneShot {
		ctx, cancelFunc = context.WithCancel(baseCtx)
	} else {
		ctx = baseCtx
	}

	sfr := &SoftwareFactoryReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		RESTClient: restClient,
		RESTConfig: config,
		CancelFunc: cancelFunc,
		Completed:  false,
	}

	//+kubebuilder:scaffold:builder

	if err = sfr.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SoftwareFactory")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func Standalone(cliNS string, kubeContext string, dryRun bool, crPath string, remotePath string) error {
	var copyFrom string
	if remotePath != "" {
		// When deploying a remote executor, we need to copy some configuration from
		// the control plane:
		copyFrom = crPath
		// … and the resources to be deployed is the remote one:
		crPath = remotePath
	}

	var sf sfv1.SoftwareFactory
	sf, err := ReadSFYAML(crPath)
	if err != nil {
		ctrl.Log.Error(err, "Could not read resource")
		os.Exit(1)
	}

	kubeConfig := filepath.Dir(crPath) + "/kubeconfig"
	if _, err := os.Stat(kubeConfig); err == nil {
		ctrl.Log.Info("Using default kubeconfig", "path", kubeConfig)
	} else {
		kubeConfig = ""
	}

	env, err := MkSFKubeContext(kubeConfig, cliNS, kubeContext, dryRun)
	if err != nil {
		ctrl.Log.Error(err, "unable to create a client")
		os.Exit(1)
	}

	if copyFrom != "" {
		if sf.Spec.Zuul.Executor.Standalone == nil {
			fmt.Printf("%s: The remote CR is not standalone\n", remotePath)
			os.Exit(1)
		}
		if err := env.setupRemoteExecutorConfig(copyFrom, sf); err != nil {
			ctrl.Log.Error(err, "unable to setup remote executor config")
			os.Exit(1)
		}
	}

	return env.StandaloneReconcile(sf, false)
}

func RotateSecrets(cliNS string, kubeContext string, dryRun bool, crPath string) error {
	var sf sfv1.SoftwareFactory
	sf, err := ReadSFYAML(crPath)
	if err != nil {
		ctrl.Log.Error(err, "Could not read resource")
		os.Exit(1)
	}

	kubeConfig := filepath.Dir(crPath) + "/kubeconfig"
	if _, err := os.Stat(kubeConfig); err == nil {
		ctrl.Log.Info("Using default kubeconfig", "path", kubeConfig)
	} else {
		kubeConfig = ""
	}

	env, err := MkSFKubeContext(kubeConfig, cliNS, kubeContext, dryRun)
	if err != nil {
		ctrl.Log.Error(err, "unable to create a client")
		os.Exit(1)
	}

	env.EnsureStandaloneOwner(sf.Spec)

	if err := env.DoRotateSecrets(); err != nil {
		return nil
	}
	return env.StandaloneReconcile(sf, false)
}
