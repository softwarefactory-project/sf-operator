// Copyright (C) 2025 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	//nolint:golint
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SoftwareFactory controller", func() {
	Context("SoftwareFactory CR validation", func() {

		const TestName = "test-cr"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestName,
				Namespace: TestName,
			},
		}
		typeNamespaceName := types.NamespacedName{
			Name:      TestName,
			Namespace: TestName,
		}
		sf := &sfv1.SoftwareFactory{}
		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("creating the custom resource for the Kind SoftwareFactory")
			err = k8sClient.Get(ctx, typeNamespaceName, sf)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				sf = &sfv1.SoftwareFactory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TestName,
						Namespace: namespace.Name,
					},
					Spec: sfv1.SoftwareFactorySpec{
						ConfigRepositoryLocation: sfv1.ConfigRepositoryLocationSpec{
							Name:               "config",
							ZuulConnectionName: "conn",
						},
					},
				}

				err = k8sClient.Create(ctx, sf)
				Expect(err).To(Not(HaveOccurred()))
			}
		})
		AfterEach(func() {
			k8sClient.Delete(context.TODO(), sf)
		})

		It("Should use reconcile", func() {
			sfReconciler := &SoftwareFactoryReconciler{
				Client:     k8sClient,
				Scheme:     k8sClient.Scheme(),
				RESTConfig: cfg,
				CancelFunc: cancel,
			}

			sfReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})

		})
	})
})
