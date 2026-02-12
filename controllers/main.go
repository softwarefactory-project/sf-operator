// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package controllers

import (
	"fmt"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"sigs.k8s.io/yaml"

	apiroutev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
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
		env.EnsureStandaloneOwner(sf.Spec)
		if err := env.setupRemoteExecutorConfig(copyFrom, sf); err != nil {
			ctrl.Log.Error(err, "unable to setup remote executor config")
			os.Exit(1)
		}
	}

	return env.StandaloneReconcile(sf)
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

	sfCtrl := MkSFController(env, sf)
	sfCtrl.EnsureToolingVolume()

	if err := env.DoRotateSecrets(); err != nil {
		return err
	}
	return env.StandaloneReconcile(sf)
}
