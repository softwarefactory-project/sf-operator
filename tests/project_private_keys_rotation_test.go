// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Project Private Keys Rotation"
var _ = Describe("Project Private Keys Rotation", Ordered, func() {
	var projectKeysBefore string

	ensureNoConfigError := func() {
		By("Ensuring tenant have no config errors")
		Ω(getConfigErrors("internal")).Should(Equal([]any{}))
		Ω(getConfigErrors("demo-tenant")).Should(Equal([]any{}))
	}

	BeforeAll(func() {
		ensureNoConfigError()
		By("Fetching project private keys state from ZooKeeper (to validate change after)")
		projectKeysBefore = getProjectKeysState()
		Ω(projectKeysBefore).ShouldNot(BeEmpty())

		By("Preparing SSH key for rotation from zuul-ssh-key secret")
		secretData := readSecret("zuul-ssh-key")
		Ω(secretData).Should(HaveKey("priv"), "zuul-ssh-key secret must have 'priv' key for rotation test")
		tmpFile, err := os.CreateTemp("", "sf-test-ssh-key-*")
		Ω(err).Should(BeNil())
		_, err = tmpFile.Write(secretData["priv"])
		Ω(err).Should(BeNil())
		Ω(tmpFile.Close()).Should(BeNil())

		By("Running rotate-projects-private-keys")
		Ω(sfctx.RotateProjectPrivateKey(tmpFile.Name(), 0, "", "")).Should(BeNil())

		By("Reconciling")
		runReconcile(sf)
	})

	It("zuul-web config-errors remain empty after rotation", func() {
		ensureNoConfigError()
	})

	It("project private keys changed when rotation was applied", func() {
		projectKeysAfter := getProjectKeysState()
		Ω(projectKeysAfter).ShouldNot(Equal(projectKeysBefore),
			"Project keys state in ZK should differ after rotation (keys were rotated)")
	})
})

// getConfigErrors returns the JSON array of config-errors for the tenant from zuul-web.
func getConfigErrors(tenant string) []interface{} {
	out := readZuulCommand("curl -s zuul-web:9000/api/tenant/" + tenant + "/config-errors")
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	var list []interface{}
	err := json.Unmarshal([]byte(out), &list)
	Ω(err).Should(BeNil(), "config-errors response should be valid JSON: %s", out)
	return list
}

// getProjectKeysState returns a string representation of project keys in ZK (path,created) for comparison.
// Empty string if there are no project keys or the script cannot run.
func getProjectKeysState() string {
	script := `python3 << 'PYEOF'
import configparser
import sys
from zuul.zk import ZooKeeperClient
from zuul.lib.keystorage import KeyStorage
config = configparser.ConfigParser()
config.read("/etc/zuul/zuul.conf")
zk_client = ZooKeeperClient.fromConfig(config)
zk_client.connect()
lines = []
for path, obj in KeyStorage(zk_client, "unused").exportKeys().get("keys", {}).items():
    if path.endswith("/secrets") and "keys" in obj and len(obj.get("keys", [])) == 1:
        lines.append(path + " " + str(obj["keys"][0]["created"]))
print("\n".join(sorted(lines)))
PYEOF
`
	// now we comparing just path and created (time), probably need to decrypt the keys and compare them as well
	return readZuulCommandArgs([]string{"bash", "-c", script})
}
