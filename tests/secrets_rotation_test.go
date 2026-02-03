// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

type Secret struct {
	name string
	key  string
}

// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Secret Rotations"
var _ = Describe("Secret Rotations", Ordered, func() {
	var zuulConf string
	var builds string

	BeforeAll(func() {
		// Avoid executor getting stuck
		ensureDelete(&appsv1.StatefulSet{ObjectMeta: mkMeta("zuul-executor")})

		secrets := []Secret{
			{name: "zuul-auth-secret", key: "zuul-auth-secret"},
			{name: "zuul-db-connection", key: "password"},
			{name: "ca-cert", key: "ca.crt"},
			{name: "zookeeper-server-tls", key: "0-tls.key"},
			{name: "zookeeper-client-tls", key: "tls.key"},
		}

		By("Checking build database")
		builds = readZuulCommand("curl zuul-web:9000/api/tenant/demo-tenant/builds")

		By("Running secret rotation CLI")
		Ω(sfctx.DoRotateSecrets()).Should(BeNil())

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
		// Need to force zuul-client stderr to stdout, otherwise it only works when the command has terminal and stdin!?
		Ω(readZuulCommandArgs([]string{"bash", "-c", "zuul-client -v autohold --tenant internal --project system-config --job testytest --reason testy 2>&1"})).
			Should(ContainSubstring("Command autohold completed successfully"))
	})

	It("zuul-db works", func() {
		By("Checking the secret is in the zuul.conf")
		newAuth := readSecretValue("zuul-db-connection", "password")
		Ω(zuulConf).Should(ContainSubstring(newAuth))

		By("Checking zuul-web works")
		newBuilds := readZuulCommand("curl zuul-web:9000/api/tenant/demo-tenant/builds")
		Ω(newBuilds).Should(Equal(builds))
	})

	It("zuul-zk works", func() {
		By("Checking the key in zuul pod")
		clientKey := readZuulCommand("cat /tls/client/tls.key")
		newKey := readSecretValue("zookeeper-client-tls", "tls.key")
		Ω(clientKey).Should(Equal(newKey))

		By("Connecting with kazoo")
		kazoo := readZuulCommandArgs([]string{"python3", "-c", "import os, kazoo.client; os.chdir('/tls/client'); hosts=list(filter(lambda l: l.startswith('hosts'), open('/etc/zuul/zuul.conf').readlines()))[0].split('=')[1].strip(); client = kazoo.client.KazooClient(hosts, use_ssl=True, keyfile='tls.key', certfile='tls.crt', ca='ca.crt'); client.start(); print(client.get_children('/keystorage2'))"})
		Ω(kazoo).Should(ContainSubstring("'tenant'"))
	})
})
