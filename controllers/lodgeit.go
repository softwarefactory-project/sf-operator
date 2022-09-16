// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the lodgeit configuration.
package controllers

import (
	_ "embed"
	"fmt"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const LODGEIT_IDENT string = "lodgeit"
const LODGEIT_IMAGE string = "quay.io/software-factory/lodgeit:0.3-2"

const LODGEIT_PORT = 5000
const LODGEIT_PORT_NAME = "lodgeit"

func (r *SFController) DeployLodgeit() bool {
	if r.cr.Spec.Lodgeit.Enabled {
		initContainers, lodgeitmysqlpasswordsecret := r.EnsureDBInit("lodgeit")

		// Generating Lodgeit Passwords
		r.GenerateSecretUUID("lodgeit-secret-key")

		lodgeitmysqlpassword := string(lodgeitmysqlpasswordsecret.Data["lodgeit-db-password"])

		dep := create_deployment(r.ns, LODGEIT_IDENT, LODGEIT_IMAGE)
		dep.Spec.Template.Spec.InitContainers = initContainers
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"/usr/local/bin/uwsgi"}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("LODGEIT_SECRET_KEY", "lodgeit-secret-key",
				"lodgeit-secret-key"),
			create_env("UWSGI_HTTP_SOCKET", ":"+strconv.Itoa(LODGEIT_PORT)),
			create_env("LODGEIT_DBURI", fmt.Sprintf("mysql+pymysql://%s:%s@%s/%s", LODGEIT_IDENT, lodgeitmysqlpassword, "mariadb", LODGEIT_IDENT)),
		}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(LODGEIT_PORT, LODGEIT_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", LODGEIT_PORT)

		r.GetOrCreate(&dep)
		srv := create_service(r.ns, LODGEIT_IDENT, LODGEIT_IDENT, LODGEIT_PORT, LODGEIT_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(LODGEIT_IDENT)
		r.DeleteService(LODGEIT_PORT_NAME)
		r.DeleteSecret("lodgeit-secret-key")
		r.DeleteSecret("lodgeit-db-password")
		r.DeleteConfigMap("lodgeit-ep-config-map")
		return true
	}
}

func (r *SFController) IngressLodgeit() netv1.IngressRule {
	return create_ingress_rule(LODGEIT_IDENT+"."+r.cr.Spec.FQDN, LODGEIT_PORT_NAME, LODGEIT_PORT)
}
