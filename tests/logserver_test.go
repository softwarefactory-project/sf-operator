// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const (
	logserverCRIOAnnotationKey   = "io.kubernetes.cri-o.TrySkipVolumeSELinuxLabel"
	logserverCRIOAnnotationValue = "true"
	logserverName                = "logserver"
)

// Logserver pod annotations test: verifies that spec.logserver.podAnnotations
// (e.g. io.kubernetes.cri-o.TrySkipVolumeSELinuxLabel for CRI-O) are applied to
// the logserver StatefulSet pod template in the demo-tenant minikube deployment.
var _ = Describe("Logserver", Ordered, func() {
	Context("When PodAnnotations are set on the CR (e.g. for CRI-O TrySkipVolumeSELinuxLabel)", func() {
		var sfWithPodAnnotations sfv1.SoftwareFactory

		BeforeAll(func() {
			sfWithPodAnnotations = *sf.DeepCopy()
			if sfWithPodAnnotations.Spec.Logserver.PodAnnotations == nil {
				sfWithPodAnnotations.Spec.Logserver.PodAnnotations = make(map[string]string)
			}
			sfWithPodAnnotations.Spec.Logserver.PodAnnotations[logserverCRIOAnnotationKey] = logserverCRIOAnnotationValue
		})

		It("Reconciles with the annotation on the CR", func() {
			runReconcile(sfWithPodAnnotations)
		})

		It("Applies the pod annotation to the logserver StatefulSet pod template", func() {
			var sts appsv1.StatefulSet
			err := sfctx.Client.Get(sfctx.Ctx, client.ObjectKey{Namespace: sfctx.Ns, Name: logserverName}, &sts)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(sts.Spec.Template.Annotations).Should(HaveKeyWithValue(logserverCRIOAnnotationKey, logserverCRIOAnnotationValue))
		})
	})
})
