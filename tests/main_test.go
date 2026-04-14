// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/shlex"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}).WithTimeout(900 * time.Second).WithPolling(time.Second).WithContext(sfctx.Ctx).Should(Equal(true))
}

// Test global environment
var (
	sfctx sfop.SFKubeContext

	// The default sf CR from playbooks/files/sf.yaml
	sf sfv1.SoftwareFactory
)

// Test environment setup
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var err error
	sfctx, err = sfop.MkSFKubeContext("", "", "", false)
	if err != nil {
		panic(fmt.Sprintf("Failed to create SFKubeContext: %s", err))
	}

	sf, err = sfop.ReadSFYAML("../playbooks/files/sf.yaml")
	if err != nil {
		panic(fmt.Sprintf("sf resource read fail: %s", err))
	}

	sfctx.EnsureStandaloneOwner(sf.Spec)

})

// helpers
func mkMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: sfctx.Ns}
}

func readSecret(name string) map[string][]byte {
	return sfctx.ReadSecret(name)
}

func readSecretValue(name string, key string) string {
	return string(readSecret(name)[key])
}

func readCommandArgs(pod string, container string, args []string) string {
	var buf bytes.Buffer
	if err := sfctx.PodExecOut(pod, container, args, &buf); err != nil {
		panic(fmt.Sprintf("Command failed: kubectl exec %s -c %s -- %s\n output: %s", pod, container, strings.Join(args, " "), buf.String()))
	}
	return buf.String()
}

func readCommand(pod string, container string, command string) string {
	if args, err := shlex.Split(command); err != nil {
		panic(fmt.Sprintf("Bad command %s: %v", command, err))
	} else {
		return readCommandArgs(pod, container, args)
	}
}

func runOrDie(name string, arg ...string) {
	cmd := exec.Command(name, arg...)

	if out, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("%s %v failed: %v\n%s", name, arg, err, out))
	}
}

func ensureDelete(obj client.Object) {
	err := sfctx.Client.Delete(sfctx.Ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		panic(fmt.Sprintf("Couldn't delete %v", obj))
	}
}

func getStableVersion() string {
	return os.Getenv("SF_STABLE_VERSION")
}

// Git helpers for integration tests (clone, commit, push via Gerrit REST API).

// getGerritAdminRepoURL returns the HTTPS URL for cloning/pushing a repo as Gerrit admin
// (same as deploy/demo-tenant-config). Works without port-forward.
func getGerritAdminRepoURL(repoName string) string {
	apiKey := readSecretValue("gerrit-admin-api-key", "gerrit-admin-api-key")
	return fmt.Sprintf("https://admin:%s@gerrit.%s/a/%s", apiKey, sf.Spec.FQDN, repoName)
}

// cloneRepo clones the given HTTPS URL (e.g. from getGerritAdminRepoURL) with SSL verify disabled for dev.
func cloneRepo(destDir string, url string) {
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		panic(fmt.Sprintf("mkdir for clone dest: %v", err))
	}
	runOrDie("env", "GIT_SSL_NO_VERIFY=true", "git", "clone", "--depth", "1", "-c", "http.sslVerify=false", url, destDir)
}

func resetRepo(repoDir string) {
	runOrDie("git", "-C", repoDir, "fetch", "origin")
	runOrDie("git", "-C", repoDir, "reset", "--hard", "origin/master")
}

func createCommitWithIdentity(repoDir string, message string, name string, email string) {
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+name,
		"GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name,
		"GIT_COMMITTER_EMAIL="+email,
	)
	add := exec.Command("git", "-C", repoDir, "add", "-A")
	add.Env = env
	if out, err := add.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("git add failed: %v\n%s", err, out))
	}
	commit := exec.Command("git", "-C", repoDir, "commit", "-m", message)
	commit.Env = env
	if out, err := commit.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("git commit failed: %v\n%s", err, out))
	}
}

func gitPush(repoDir string) {
	runOrDie("git", "-C", repoDir, "push")
}

var _ = Describe("Test Env", Ordered, func() {
	It("Has namespace", func() {
		Ω(sfctx.Ns).ShouldNot(BeEmpty())
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

func readZuulCommand(command string) string {
	return readCommand("zuul-scheduler-0", "zuul-scheduler", command)
}

func readZuulCommandArgs(args []string) string {
	return readCommandArgs("zuul-scheduler-0", "zuul-scheduler", args)
}

func ensureNoConfigError() {
	Ω(getConfigErrors("internal")).Should(Equal([]any{}))
	Ω(getConfigErrors("demo-tenant")).Should(Equal([]any{}))
}
