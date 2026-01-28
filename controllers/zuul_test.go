// Copyright (C) 2025 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"errors"
	"time"

	//nolint:golint
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"

	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Zuul controller", func() {
	Context("Zuul controller test", func() {

		const TestName = "test-zuul"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestName,
				Namespace: TestName,
			},
		}
		secretKey := client.ObjectKey{
			Namespace: TestName,
			Name:      "zuul-config",
		}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should handle user provided opendev connection", func() {
			r := SFController{
				SFKubeContext: SFKubeContext{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					Ns:     TestName,
					Ctx:    ctx,
					Owner:  namespace,
				},
				needOpendev: true,
			}

			r.DeployZuulSecrets()
			r.EnsureZuulConfigSecret(true, true)

			By("Checking if zuul-config was created with git connection to opendev.")
			Eventually(func() error {
				found := &corev1.Secret{}
				k8sClient.Get(ctx, secretKey, found)
				if !bytes.Contains(found.Data["zuul.conf"], []byte("https://opendev.org")) {
					return errors.New("opendev.org was not added!")
				}
				return nil
			}, 10*time.Second, time.Second).Should(Succeed())

			By("Checking if zuul-config was created with user provided gerrit connection to opendev.")
			r.cr.Spec.Zuul.GerritConns = []sfv1.GerritConnection{{
				Name:     "opendev.org",
				Hostname: "review.opendev.org",
			}}
			// Ideally we should use the main reconciller to handle connection validation
			// this is more like unit-testing for now...
			r.needOpendev = false
			r.EnsureZuulConfigSecret(true, true)
			Eventually(func() error {
				found := &corev1.Secret{}
				k8sClient.Get(ctx, secretKey, found)
				if bytes.Contains(found.Data["zuul.conf"], []byte("https://opendev.org")) || !bytes.Contains(found.Data["zuul.conf"], []byte("review.opendev.org")) {
					return errors.New("opendev.org was added!")
				}
				return nil
			}, 10*time.Second, time.Second).Should(Succeed())
		})
	})
	Context("DumpConfigINI test", func() {
		It("should sort keys alphabetically within each section", func() {

			cfg := ini.Empty()
			sec, err := cfg.NewSection("zookeeper")
			Expect(err).ToNot(HaveOccurred())
			_, err = sec.NewKey("hosts", "zookeeper.sfoperator.svc:2281")
			Expect(err).ToNot(HaveOccurred())
			_, err = sec.NewKey("ca", "/tls/client/ca.crt")
			Expect(err).ToNot(HaveOccurred())

			sec2, err := cfg.NewSection("scheduler")
			Expect(err).ToNot(HaveOccurred())
			_, err = sec2.NewKey("max_hold_expiration", "86400")
			Expect(err).ToNot(HaveOccurred())
			_, err = sec2.NewKey("default_hold_expiration", "3600")
			Expect(err).ToNot(HaveOccurred())

			actual := DumpConfigINI(cfg)

			expected := `[zookeeper]
ca    = /tls/client/ca.crt
hosts = zookeeper.sfoperator.svc:2281

[scheduler]
default_hold_expiration = 3600
max_hold_expiration     = 86400
`
			Expect(actual).To(Equal(expected))
		})
	})
})
