// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/shlex"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
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
	sfctx, err = sfop.MkSFKubeContext("", "")
	if err != nil {
		panic(fmt.Sprintf("Failed to create SFKubeContext: %s", err))
	}

	sf, err = sfop.ReadSFYAML("../playbooks/files/sf.yaml")
	if err != nil {
		panic(fmt.Sprintf("sf resource read fail: %s", err))
	}

	owner, err := sfop.EnsureStandaloneOwner(sfctx.Ctx, sfctx.Client, sfctx.Ns, sf.Spec)
	if err != nil {
		panic("sf owner resource creation failed")
	}
	sfctx.Owner = &owner

})

// helpers
func mkMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: sfctx.Ns}
}

func readSecret(name string) map[string][]byte {
	var sec apiv1.Secret
	err := sfctx.Client.Get(sfctx.Ctx, client.ObjectKey{Name: name, Namespace: sfctx.Ns}, &sec)
	if err != nil {
		return make(map[string][]byte)
	}
	return sec.Data
}

func readSecretValue(name string, key string) string {
	return string(readSecret(name)[key])
}

func readCommandArgs(pod string, container string, args []string) string {
	var buf bytes.Buffer
	if err := sfctx.PodExecOut(pod, container, args, &buf); err != nil {
		panic(fmt.Sprintf("Command failed: kubectl exec %s -c %s -- %s", pod, container, strings.Join(args, " ")))
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

func ensureDelete(obj client.Object) {
	err := sfctx.Client.Delete(sfctx.Ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		panic(fmt.Sprintf("Couldn't delete %v", obj))
	}
}

var _ = Describe("Test Env", Ordered, func() {
	It("Has namespace", func() {
		Ω(sfctx.Ns).ShouldNot(BeEmpty())
	})
})
