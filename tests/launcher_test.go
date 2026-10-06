// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sfop "github.com/softwarefactory-project/sf-operator/controllers"
	"k8s.io/client-go/tools/clientcmd"
)

// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Launcher Tests"
var _ = Describe("Launcher Tests", Ordered, func() {

	It("setup namespace", func() {
		By("Creating namespace")
		npctxs, err := sfop.MkSFKubeContext("", "nodepool", "", false)
		Ω(err).Should(BeNil())
		Ω(sfctx.SetupK8SProvider(&npctxs)).Should(BeNil())
		kubeconfig := readSecret("nodepool-providers-secrets")["kube.config"]
		Ω(kubeconfig).ShouldNot(BeEmpty())
		config, err := clientcmd.Load(kubeconfig)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(config.CurrentContext).Should(Equal(sfop.OpenshiftPodsKubeContext))
		Ω(config.Contexts).Should(HaveKey(sfop.OpenshiftPodsKubeContext))

		By("Add kubeconfig to the CR")
		runReconcile(sf)
	})

	It("add image, section and provider", func() {
		By("Submitting config")
		fmt.Printf("run git-review and wait for CI")
	})

	It("validate job are running in k8s", func() {
		fmt.Printf("tltl")
	})
})
