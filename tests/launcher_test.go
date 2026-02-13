// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	sfop "github.com/softwarefactory-project/sf-operator/controllers"
	"k8s.io/client-go/tools/clientcmd"
)

// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Launcher Tests"
var _ = Describe("Launcher Tests", Ordered, func() {

	It("setup namespace", func() {
		By("Creating namespace")
		npctxs, err := sfop.MkSFKubeContext("", "nodepool", "", false)
		Ω(err).Should(BeNil())
		Ω(sfctx.SetupK8SProvider(&npctxs, "local")).Should(BeNil())
		kubeconfig := readSecret("nodepool-providers-secrets")["kube.config"]
		Ω(kubeconfig).ShouldNot(BeEmpty())
		config, err := clientcmd.Load(kubeconfig)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(config.CurrentContext).Should(Equal(sfop.OpenshiftPodsKubeContext))
		Ω(config.Contexts).Should(HaveKey(sfop.OpenshiftPodsKubeContext))

		By("Add connection to the CR")
		newSF := sf.DeepCopy()
		newSF.Spec.Zuul.KubernetesProviders = []sfv1.KubernetesProvider{
			{
				Name:   "local-kube",
				Secret: "local-kubeconfig",
			},
		}
		runReconcile(*newSF)
	})

	It("add image, section and provider", func() {
		By("Submitting config")
		os.WriteFile("../deploy/demo-tenant-config/zuul.d/launcher.yaml", []byte(`
- flavor:
    name: normal

- image:
    name: debian
    type: cloud

- label:
    name: debian-normal
    image: debian
    flavor: normal

- section:
    name: kube
    connection: local-kube
    launch-timeout: 600
    launch-attempts: 2
    label-defaults:
      boot-timeout: 120
    flavors:
      - name: normal
    images:
      - name: debian

- provider:
    name: kube-main
    section: kube
    labels:
      - name: debian-normal
        kind: pod
        spec:
          containers:
            - name: pod-custom
              image: ubuntu:jammy
              imagePullPolicy: IfNotPresent
              command: ["/bin/sh", "-c"]
              args: ["while true; do sleep 30; done;"]
`), 0600)
		fmt.Printf("run git-review and wait for CI")
	})

	It("validate job are running in k8s", func() {
		fmt.Printf("tltl")
	})
})
