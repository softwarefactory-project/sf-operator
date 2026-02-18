// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"os"
	"strconv"

	"k8s.io/client-go/rest"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"

	discovery "k8s.io/client-go/discovery"
)

type K8sDist int

const (
	Kubernetes K8sDist = iota
	Openshift
)

func kubernetesDistribution(restClient rest.Interface) K8sDist {
	// Create a DiscoveryClient for a given config
	discoveryClient := discovery.NewDiscoveryClient(restClient)

	// Get Api Resources Groups
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "command was not able to find the cluster server groups.\nCheck if the provided kubeconfig file is right.")
		os.Exit(1)
	}

	// Iterate list for config.openshift.io
	apiGroups := apiList.Groups
	for _, element := range apiGroups {
		if element.Name == "route.openshift.io" {
			return Openshift
		}
	}
	return Kubernetes
}

func checkOpenShift(restClient rest.Interface) bool {

	// Check if environment variable exists
	env := os.Getenv("OPENSHIFT_USER")

	if env != "" {
		openshiftUser, err := strconv.ParseBool(env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "The OPENSHIFT_USER environment variable must be set to true/false, it was set to '%s'\n", env)
			os.Exit(1)
		}
		return openshiftUser
	}

	// Discovering Kubernetes Distribution
	logging.LogI("OPENSHIFT_USER environment variable is not set, discovering Kubernetes Distribution\n")

	var flavour = kubernetesDistribution(restClient)
	switch flavour {
	case Openshift:
		logging.LogI("Kubernetes Distribution found: Openshift\n")
		return true
	default:
		logging.LogI("Kubernetes Distribution found: Kubernetes\n")
		return false
	}
}
