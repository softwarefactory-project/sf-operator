// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package controllers

import (
	"context"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	apiroutev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	config "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	//+kubebuilder:scaffold:imports
)

var setupLog = ctrl.Log.WithName("setup")
var SetupLog = setupLog
var controllerScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(controllerScheme))

	utilruntime.Must(sfv1.AddToScheme(controllerScheme))
	utilruntime.Must(certv1.AddToScheme(controllerScheme))
	utilruntime.Must(apiroutev1.AddToScheme(controllerScheme))
	utilruntime.Must(monitoringv1.AddToScheme(controllerScheme))
	utilruntime.Must(apiextensions.AddToScheme(controllerScheme))
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

func GetConfigContextOrDie(contextName string) *rest.Config {
	var conf *rest.Config
	var err error
	if conf, err = config.GetConfigWithContext(contextName); err != nil {
		ctrl.Log.Error(err, "couldn't find context "+contextName)
		os.Exit(1)
	}
	return conf
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

func Standalone(sf sfv1.SoftwareFactory, ns string, kubeContext string) error {

	config := GetConfigContextOrDie(kubeContext)
	cl, err := client.New(config, client.Options{
		Scheme: controllerScheme,
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to create a client")
		os.Exit(1)
	}
	restClient := getPodRESTClient(config)
	ctx, cancelFunc := context.WithCancel(ctrl.SetupSignalHandler())
	sfr := &SoftwareFactoryReconciler{
		Client:     cl,
		Scheme:     controllerScheme,
		RESTClient: restClient,
		RESTConfig: config,
		CancelFunc: cancelFunc,
	}
	return sfr.StandaloneReconcile(ctx, ns, sf)
}
