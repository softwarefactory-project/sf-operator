// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// When running 'go test -v ./tests/...', this call this function
func TestSFOP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Software Factory CI")
}

// Test global environment
var (
	namespace string
)

// Test environment setup
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	// Load kubeconfig from the dev host
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		panic("No kube config")
	}
	contextName := config.CurrentContext
	kcontext, err2 := config.Contexts[contextName]
	if !err2 {
		panic("No kube context")
	}

	// Important: discover the default namespace:
	namespace = kcontext.Namespace
})

var _ = Describe("Test Env", Ordered, func() {
	It("Has namespace", func() {
		Ω(namespace).ShouldNot(BeEmpty())
	})
})
