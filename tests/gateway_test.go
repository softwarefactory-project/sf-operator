// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("gateway tests", Ordered, func() {
	maintenanceConfig := `
<VirtualHost *:8080>
	DocumentRoot "/var/www/html"
	ErrorDocument 503 /maintenance.html
	<IfModule mod_rewrite.c>
		RewriteEngine on
		RewriteCond %{REQUEST_URI} !=/maintenance.html
		RewriteRule ^.*$ - [R=503,L]
	</IfModule>
</VirtualHost>
`
	maintenanceHTML := "Under maintenance"
	Context("When setting up extra gateway configuration for a maintenance", func() {
		It("Should reconcile", func() {

			// create configmaps
			var cfData = make(map[string]string)
			cfData["00-maintenance.conf"] = maintenanceConfig
			var sfData = make(map[string]string)
			sfData["maintenance.html"] = maintenanceHTML
			var cfCM = apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "maintenance-config", Namespace: sfctx.Ns},
				Data:       cfData,
			}
			sfctx.CreateR(&cfCM)
			sfCMName := "maintenance-static-files"
			var sfCM = apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: sfCMName, Namespace: sfctx.Ns},
				Data:       sfData,
			}
			sfctx.CreateR(&sfCM)
			// add configmap references to sf spec
			gwSpec := sfv1.GatewaySpec{
				ExtraConfigurationConfigMap: "maintenance-config",
				ExtraStaticFilesConfigMap:   &sfCMName,
			}
			sf.Spec.Gateway = &gwSpec
			// reconcile
			runReconcile(sf)
		})
		It("Should serve HTTP 503 and the expected content", func() {
			// test gateway:8080/zuul URL from the scheduler pod
			var isInMaintenance string
			isInMaintenance = readZuulCommand("curl -o - -I gateway:8080/zuul/")
			Ω(isInMaintenance).Should(ContainSubstring("503"))
			isInMaintenance = readZuulCommand("curl gateway:8080/zuul/api/components")
			Ω(isInMaintenance).Should(ContainSubstring(maintenanceHTML))
		})
	})
	Context("When removing the gateway's extra config", func() {
		It("Should reconcile", func() {
			// remove configmap references from sf spec
			sf.Spec.Gateway = nil
			// reconcile
			runReconcile(sf)
		})
		It("Should serve the regular content", func() {
			// test gateway:8080/zuul URL from the scheduler pod
			var isInMaintenance string
			isInMaintenance = readZuulCommand("curl -o - -I gateway:8080/zuul/api/components")
			Ω(isInMaintenance).ShouldNot(ContainSubstring("503"))
			isInMaintenance = readZuulCommand("curl gateway:8080/zuul/api/components")
			Ω(isInMaintenance).ShouldNot(ContainSubstring(maintenanceHTML))
		})
	})

})
