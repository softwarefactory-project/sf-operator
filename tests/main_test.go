// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"bytes"
	"context"
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

// helpers
func mkMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace}
}

func readSecret(name string) map[string][]byte {
	var sec apiv1.Secret
	err := sfctx.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &sec)
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
	err := sfctx.Client.Delete(ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		panic(fmt.Sprintf("Couldn't delete %v", obj))
	}
}

var _ = Describe("Test Env", Ordered, func() {
	It("Has namespace", func() {
		Ω(namespace).ShouldNot(BeEmpty())
	})
})
