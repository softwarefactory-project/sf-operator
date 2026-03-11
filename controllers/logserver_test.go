// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"errors"
	"time"

	//nolint:golint
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const (
	logserverCRIOAnnotationKey   = "io.kubernetes.cri-o.TrySkipVolumeSELinuxLabel"
	logserverCRIOAnnotationValue = "true"
	logserverStatefulSetName     = "logserver"
)

// Re-run using:
// CGO_ENABLED=1 KUBEBUILDER_ASSETS="${HOME}/.local/share/kubebuilder-envtest/k8s/1.35.0-linux-amd64" go test -v ./controllers -args --ginkgo.v --ginkgo.focus "Logserver"
var _ = Describe("Logserver controller", func() {
	It("deploys the logserver StatefulSet", func() {
		cr := sfv1.SoftwareFactory{}
		r := MkSFController(sfctx, cr)
		r.DeployLogserver()

		By("Checking if statefulset was created")
		Eventually(func() bool {
			sts := &appsv1.StatefulSet{}
			return sfctx.GetOrDie(logserverStatefulSetName, sts)
		}, 5*time.Second, 50*time.Millisecond).Should(BeTrue())
	})

	It("Applies podAnnotations from the CR to the logserver pod template", func() {
		cr := sfv1.SoftwareFactory{
			Spec: sfv1.SoftwareFactorySpec{
				Logserver: sfv1.LogServerSpec{
					RetentionDays: 60,
					LoopDelay:     3600,
					PodAnnotations: map[string]string{
						logserverCRIOAnnotationKey: logserverCRIOAnnotationValue,
					},
				},
			},
		}
		r := MkSFController(sfctx, cr)
		r.DeployLogserver()

		By("Checking the logserver StatefulSet pod template has the CRI-O annotation")
		Eventually(func() error {
			sts := &appsv1.StatefulSet{}
			if !sfctx.GetOrDie(logserverStatefulSetName, sts) {
				return errors.New("statefulset not found")
			}
			v, ok := sts.Spec.Template.ObjectMeta.Annotations[logserverCRIOAnnotationKey]
			if !ok || v != logserverCRIOAnnotationValue {
				return errors.New("podAnnotations not yet applied")
			}
			return nil
		}, 5*time.Second, 50*time.Millisecond).Should(Succeed())

		sts := &appsv1.StatefulSet{}
		Expect(sfctx.GetOrDie(logserverStatefulSetName, sts)).To(BeTrue())
		Expect(sts.Spec.Template.ObjectMeta.Annotations).To(HaveKeyWithValue(logserverCRIOAnnotationKey, logserverCRIOAnnotationValue))
	})
})
