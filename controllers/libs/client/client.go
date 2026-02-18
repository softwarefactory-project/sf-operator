// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package client wraps the low-level kubernetes client.
package client

import (
	"context"
	"fmt"
	"os"

	apiroutev1 "github.com/openshift/api/route/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

type KubeClient struct {
	Client      client.Client
	Scheme      *runtime.Scheme
	RESTClient  rest.Interface
	RESTConfig  *rest.Config
	ClientSet   *kubernetes.Clientset
	Ns          string
	Ctx         context.Context
	Cancel      context.CancelFunc
	Owner       client.Object
	IsOpenShift bool
	Standalone  bool
	DryRun      bool
}

func MkKubeClient(kubeconfig string, kubecontext string, namespace string, dryRun bool) (KubeClient, error) {
	// Discover the kubeconfig file
	if kubeconfig != "" {
		if _, err := os.Stat(kubeconfig); err != nil {
			return KubeClient{}, fmt.Errorf("%s: missing kubeconfig", kubeconfig)
		}
	} else {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	// Discover the context
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{
		CurrentContext: kubecontext,
	})

	// Discover the namespace
	if namespace == "" {
		rawConfig, err := config.RawConfig()
		if err != nil {
			return KubeClient{}, err
		}
		if kubecontext == "" {
			kubecontext = rawConfig.CurrentContext
		}
		namespace = rawConfig.Contexts[kubecontext].Namespace
	}

	// Setup clients
	restconfig, err := config.ClientConfig()
	if err != nil {
		return KubeClient{}, err
	}
	clientset, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		return KubeClient{}, err
	}
	restClient := clientset.CoreV1().RESTClient()
	c, err := client.New(restconfig, client.Options{Scheme: scheme})
	if err != nil {
		return KubeClient{}, err
	}

	ctx, cancel := context.WithCancel(context.TODO())
	return KubeClient{
		Client:      c,
		Scheme:      scheme,
		RESTClient:  restClient,
		RESTConfig:  restconfig,
		ClientSet:   clientset,
		Ns:          namespace,
		Ctx:         ctx,
		Cancel:      cancel,
		Owner:       &apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ""}},
		IsOpenShift: checkOpenShift(restClient),
		DryRun:      dryRun,
		Standalone:  true,
	}, nil
}

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sfv1.AddToScheme(scheme))
	utilruntime.Must(apiroutev1.AddToScheme(scheme))
}
