// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	sfop "github.com/softwarefactory-project/sf-operator/controllers"
)

// When running 'go test -v ./tests/...', this call this function
func TestSFOP(t *testing.T) {
	// Integrate ginkgo/gomega with golang testing standard
	RegisterFailHandler(Fail)
	RunSpecs(t, "Software Factory CI")
}

// Helper function to apply the CR and wait for reconcile loop to succeed
func runReconcile(cr sfv1.SoftwareFactory) {
	ctrl := sfop.MkSFController(sfctx, cr)

	Eventually(func() bool {
		status := ctrl.Step()
		return status.Ready
	}, time.Second*900, time.Second).Should(Equal(true))
}

// Test global environment
var (
	ctx       context.Context
	cancel    context.CancelFunc
	sfctx     sfop.SFUtilContext
	namespace string
	// The default sf CR from playbooks/files/sf.yaml
	sf sfv1.SoftwareFactory
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

	ctx, cancel = context.WithCancel(context.TODO())
	scheme := sfop.InitScheme()
	restConfig := sfop.GetConfigContextOrDie(contextName)
	clientConfig := sfop.GetConfigContextOrDie(contextName)
	client, err3 := client.New(clientConfig, client.Options{
		Scheme: scheme,
	})
	if err3 != nil {
		panic("client create error")
	}

	sf, err3 = sfop.ReadSFYAML("../playbooks/files/sf.yaml")
	if err3 != nil {
		panic(fmt.Sprintf("sf resource read fail: %s", err3))
	}

	owner, err5 := sfop.EnsureStandaloneOwner(ctx, client, namespace, sf.Spec)
	if err5 != nil {
		panic("sf owner resource creation failed")
	}

	sfctx = sfop.MkSFUtilContext(ctx, client, restConfig, namespace, &owner, true)
})

var _ = Describe("Test Env", Ordered, func() {
	It("Has namespace", func() {
		Ω(namespace).ShouldNot(BeEmpty())
	})
})
