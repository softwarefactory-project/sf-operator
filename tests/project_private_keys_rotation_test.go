// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// run with go test -v ./tests/... -args --ginkgo.v --ginkgo.focus "Project Private Keys Rotation"
var _ = Describe("Project Private Keys Rotation", Ordered, func() {
	var projectKeysBefore string
	var originalSecretsYaml string
	var authorName = "zuul"
	var authorEmail = "zuul@sfop.me"
	var testSecretPassword string
	var secretsContentAfter []byte

	ensureNoConfigError := func() {
		By("Ensuring tenant have no config errors")
		Ω(getConfigErrors("internal")).Should(Equal([]any{}))
		Ω(getConfigErrors("demo-tenant")).Should(Equal([]any{}))
	}

	It("ensures no config errors", func() {
		ensureNoConfigError()
	})

	It("inject new secret in demo-project", func() {
		if getStableVersion() == "v0.0.65" || getStableVersion() == "v0.0.64" {
			Skip("This assertion only works with sf > 0.65")
		}
		By("Ensure a test ssh key is available")
		testSSHPath := "../deploy/test-ssh-key"
		if _, err := os.Stat(testSSHPath); os.IsNotExist(err) {
			cmd := exec.Command("ssh-keygen", "-f", testSSHPath, "-N", "")
			if out, err := cmd.CombinedOutput(); err != nil {
				panic(fmt.Sprintf("ssh-keygen failed: %v\n%s", err, out))
			}
		}

		By("Loading test secret from fixture (fake SSH key)")
		secretBytes, err := os.ReadFile(testSSHPath)
		Ω(err).Should(BeNil(), "read test secret fixture: %s", testSSHPath)
		// Trim to match runZuulDecryptSecretInPod() which returns TrimSpace(command output)
		testSecretPassword = strings.TrimSpace(string(secretBytes))

		By("Writing test secret into zuul-scheduler pod and encrypting with zuul-client")
		Ω(sfctx.PodExecIn("zuul-scheduler-0", "zuul-scheduler", []string{"bash", "-c", "cat > /tmp/secret-to-encrypt.txt"}, bytes.NewReader([]byte(testSecretPassword)))).Should(BeNil())
		encryptedBuf, err := sfctx.PodExecBytes("zuul-scheduler-0", "zuul-scheduler", []string{"bash", "-c",
			"cat /tmp/secret-to-encrypt.txt | zuul-client encrypt --tenant demo-tenant --project demo-project"})
		Ω(err).Should(BeNil())
		Ω(encryptedBuf.Bytes()).ShouldNot(BeEmpty())

		By("Replacing placeholders in secrets.yaml")
		originalSecretsYaml = replacePlaceholdersInSecretYaml(encryptedBuf.String())

		repoDir := "../deploy/demo-project"
		if _, err := os.Stat(repoDir); os.IsNotExist(err) {
			cloneRepo(repoDir, getGerritAdminRepoURL("demo-project"))
		}
		resetRepo(repoDir)

		By("Writing .zuul.d/secrets.yaml in repo")
		zuulDir := repoDir + "/.zuul.d"
		secretsPath := zuulDir + "/secrets.yaml"
		Ω(os.MkdirAll(zuulDir, 0755)).Should(BeNil())
		_ = os.Remove(secretsPath) // ensure clean file when repo already had secrets from a previous run
		Ω(os.WriteFile(secretsPath, []byte(originalSecretsYaml), 0644)).Should(BeNil())

		By("Committing and pushing locally as admin (HTTPS)")
		createCommitWithIdentity(repoDir, "Add in-repo secret for rotation test", "admin", "admin@"+sf.Spec.FQDN)
		gitPush(repoDir)
	})

	It("runs rotate-projects-private-keys", func() {
		By("Fetching project private keys state from ZooKeeper (to validate change after)")
		projectKeysBefore = getProjectKeysState()
		By(fmt.Sprintf("Project keys before rotation: %s", projectKeysBefore))
		Ω(projectKeysBefore).ShouldNot(BeEmpty(), "demo-project must have a key after seed (from zuul-client encrypt)")

		By("Preparing SSH key for rotation from zuul-ssh-key secret")
		secretData := readSecret("zuul-ssh-key")
		Ω(secretData).Should(HaveKey("priv"), "zuul-ssh-key secret must have 'priv' key for rotation test")
		err := os.WriteFile("../deploy/zuul-ssh-key", secretData["priv"], 0o600)
		Ω(err).Should(BeNil())

		By("Running rotate-projects-private-keys")
		Ω(sfctx.RotateProjectPrivateKey("../deploy/zuul-ssh-key", 0, authorName, authorEmail)).Should(BeNil())

		By("Reconciling")
		runReconcile(sf)
	})

	It("zuul-web config-errors remain empty after rotation", func() {
		ensureNoConfigError()
	})

	It("project private keys changed when rotation was applied", func() {
		if getStableVersion() == "v0.0.65" || getStableVersion() == "v0.0.64" {
			Skip("This assertion only works with sf > 0.65")
		}
		projectKeysAfter := getProjectKeysState()
		By(fmt.Sprintf("Project keys after rotation: %s", projectKeysAfter))
		Ω(projectKeysAfter).ShouldNot(Equal(projectKeysBefore),
			"Project keys state in ZK should differ after rotation (keys were rotated)")

		By("Pulling latest demo-project and reading .zuul.d/secrets.yaml")
		resetRepo("../deploy/demo-project")

		secretsPath := "../deploy/demo-project/.zuul.d/secrets.yaml"
		var errRead error
		secretsContentAfter, errRead = os.ReadFile(secretsPath)
		Ω(errRead).Should(BeNil(), "secrets.yaml should exist after rotation")
		Ω(string(secretsContentAfter)).ShouldNot(Equal(originalSecretsYaml),
			"secrets.yaml should have been re-encrypted by rotation (content must differ)")

		By("Writing secrets.yaml into zuul-scheduler pod for decryption")
		Ω(sfctx.PodExecIn("zuul-scheduler-0", "zuul-scheduler", []string{"bash", "-c", "cat > /tmp/secrets.yaml"}, bytes.NewReader(secretsContentAfter))).Should(BeNil())

		By("Loading project keys state (writes demo-project key to /tmp/demo-project.key)")
		getProjectKeysState()

		By("Decrypting with Zuul library")
		decrypted := runZuulDecryptSecretInPod()
		By(fmt.Sprintf("Decrypted secret: %s", decrypted))
		Ω(decrypted).Should(Equal(testSecretPassword), "secret plaintext after rotation must still match testSecretPassword")
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

// replacePlaceholdersInSecretYaml replaces "<name>" -> "test-secret", "<fieldname>" -> "password".
func replacePlaceholdersInSecretYaml(encryptedOut string) string {
	s := strings.ReplaceAll(encryptedOut, "<name>", "test-secret")
	s = strings.ReplaceAll(s, "<fieldname>", "password")
	return s
}

// getProjectKeysState returns a string representation of project keys in ZK (path, created, private_key_fingerprint) for comparison.
// It also writes the demo-project private key (unencrypted PEM) to /tmp/demo-project.key when found, for use by decrypt_secret.py.
// Empty string if there are no project keys or the script cannot run.
func getProjectKeysState() string {
	script := `python3 << 'PYEOF'
import configparser
import hashlib
from zuul.zk import ZooKeeperClient
from zuul.lib.keystorage import KeyStorage
from zuul.lib import encryption

config = configparser.ConfigParser()
config.read("/etc/zuul/zuul.conf")
password = config["keystore"]["password"].encode("utf-8")
zk_client = ZooKeeperClient.fromConfig(config)
zk_client.connect()
lines = []
for path, obj in KeyStorage(zk_client, "unused").exportKeys().get("keys", {}).items():
    if path.endswith("/secrets") and "keys" in obj and len(obj.get("keys", [])) == 1:
        created = obj["keys"][0]["created"]
        priv_pem = obj["keys"][0]["private_key"]
        fingerprint = hashlib.sha256(priv_pem.encode()).hexdigest()[:16]
        lines.append(f"{path} {created} {fingerprint}")
        if "demo-project" in path:
            priv, _ = encryption.deserialize_rsa_keypair(priv_pem.encode("utf-8"), password)
            pem = encryption.serialize_rsa_private_key(priv, None)
            with open("/tmp/demo-project.key", "wb") as f:
                f.write(pem)
print("\n".join(sorted(lines)))
PYEOF
`
	return readZuulCommandArgs([]string{"bash", "-c", script})
}

// runZuulDecryptSecretInPod decrypts secrets using Zuul's library (zuul.configloader, zuul.model.Secret.decrypt)
// with /tmp/demo-project.key and /tmp/secrets.yaml. Requires the key and secrets file to be written first.
// Returns the decrypted secret value (for our single secret with one field, the plaintext "test").
func runZuulDecryptSecretInPod() string {
	script := `python3 << 'PYEOF'
from zuul.lib import encryption
import zuul.configloader
import zuul.model

with open("/tmp/demo-project.key", "rb") as f:
    private_secrets_key, _ = encryption.deserialize_rsa_keypair(f.read())
with open("/tmp/secrets.yaml") as f:
    yaml_content = f.read()

parser = zuul.configloader.SecretParser(None)
sc = zuul.model.SourceContext(None, "project", None, "master", "path")
data = zuul.configloader.safe_load_yaml(yaml_content, sc)
for element in data:
    if "secret" not in element:
        continue
    secret = parser.fromYaml(element["secret"])
    decrypted = secret.decrypt(private_secrets_key).secret_data
    if isinstance(decrypted, dict) and "password" in decrypted:
        print(decrypted["password"], end="")
    elif isinstance(decrypted, str):
        print(decrypted, end="")
    else:
        print(decrypted, end="")
    break
PYEOF
`
	return strings.TrimSpace(readZuulCommandArgs([]string{"bash", "-c", script}))
}
