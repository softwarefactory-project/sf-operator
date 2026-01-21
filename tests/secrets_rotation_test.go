// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "k8s.io/api/core/v1"
)

type Secret struct {
	name string
	key  string
}

var _ = Describe("Secret Rotations", Ordered, func() {
	var zuulConf string
	BeforeAll(func() {
		secrets := []Secret{
			{name: "zuul-auth-secret", key: "zuul-auth-secret"},
		}

		By("Deleting secrets")
		prevValues := make(map[string]map[string][]byte)
		for _, secret := range secrets {
			prevValues[secret.name] = readSecret(secret.name)
			ensureDelete(&apiv1.Secret{ObjectMeta: mkMeta(secret.name)})
		}

		By("Reconciling")
		runReconcile(sf)

		By("Validating secret changed")
		// Validate secrets changed
		for _, secret := range secrets {
			newValue := readSecret(secret.name)
			Ω(newValue).Should(HaveKey(secret.key))
			Ω(newValue).ShouldNot(Equal(prevValues[secret.name]))
		}
		zuulConf = zuulConfMatchSecret()
	})

	// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "zuul-auth"
	It("zuul-auth works", func() {
		By("Checking the secret is in the zuul.conf")
		newAuth := readSecretValue("zuul-auth-secret", "zuul-auth-secret")
		Ω(zuulConf).Should(ContainSubstring(newAuth))

		By("Checking zuul-client works")
		Ω(sfctx.PodExec("zuul-scheduler-0", "zuul-scheduler", strings.Split("zuul-client -v autohold --tenant internal --project system-config --job testytest --reason testy", " "))).Should(BeNil())
	})
})
