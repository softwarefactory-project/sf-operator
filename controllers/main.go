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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	apiroutev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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
	//+kubebuilder:scaffold:scheme
}

func Main(ns string, metricsAddr string, probeAddr string, enableLeaderElection bool, oneShot bool) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Namespace:              ns,
		Scheme:                 controllerScheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "01752ab0.softwarefactory-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	httpCli, err := rest.HTTPClientFor(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to get HTTP client")
		os.Exit(1)
	}

	restClient, err := apiutil.RESTClientForGVK(gvk, false, mgr.GetConfig(), serializer.NewCodecFactory(mgr.GetScheme()), httpCli)
	if err != nil {
		setupLog.Error(err, "unable to create REST client")
	}

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
		RESTConfig: mgr.GetConfig(),
		CancelFunc: cancelFunc,
		Completed:  false,
	}

	lgr := &LogServerReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		RESTClient: restClient,
		RESTConfig: mgr.GetConfig(),
	}

	//+kubebuilder:scaffold:builder

	if err = sfr.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SoftwareFactory")
		os.Exit(1)
	}
	if err = lgr.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogServer")
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
