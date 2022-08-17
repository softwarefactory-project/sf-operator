// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the keycloak configuration.

package controllers

import (
	_ "embed"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const KC_PORT = 8080
const KC_PORT_NAME = "kc-port"
const KC_IMAGE = "quay.io/keycloak/keycloak:19.0.1"

//go:embed static/keycloak/post-init.sh
var kcPostInit string

//go:embed static/keycloak/entrypoint.sh
var kc_entrypoint string

func (r *SFController) KCPostInit() bool {
	var job batchv1.Job
	job_name := "kcadm-post-init"
	found := r.GetM(job_name, &job)

	if !found {
		container := apiv1.Container{
			Name:    "kcadm-client",
			Image:   KC_IMAGE,
			Command: []string{"sh", "-c", kcPostInit},
			Env: []apiv1.EnvVar{
				create_env("KC_PORT", strconv.Itoa(KC_PORT)),
				create_env("KEYCLOAK_ADMIN", "admin"),
				create_secret_env("KEYCLOAK_ADMIN_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			},
		}
		job := create_job(r.ns, job_name, container)

		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for kcadmin result")
		return false
	}
}

func (r *SFController) DeployKeycloak(enabled bool) bool {
	if enabled {
		r.GenerateSecretUUID("keycloak-admin-password")
		initContainers, _ := r.EnsureDBInit("keycloak")
		dep := create_deployment(r.ns, "keycloak", KC_IMAGE)
		dep.Spec.Template.Spec.InitContainers = initContainers
		dep.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", kc_entrypoint}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(KC_PORT, KC_PORT_NAME),
		}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("INGRESS_HOSTNAME", "keycloak."+r.cr.Spec.FQDN),
			create_secret_env("DB_PASSWORD", "keycloak-db-password", "keycloak-db-password"),
			create_env("KEYCLOAK_ADMIN", "admin"),
			create_secret_env("KEYCLOAK_ADMIN_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
		}
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", KC_PORT)
		r.GetOrCreate(&dep)
		srv := create_service(r.ns, "keycloak", "keycloak", KC_PORT, KC_PORT_NAME)
		r.GetOrCreate(&srv)

		ready := r.IsDeploymentReady(&dep)
		if ready {
			return r.KCPostInit()
		} else {
			return false
		}

	} else {
		r.DeleteDeployment("keycloak")
		r.DeleteService("keycloak")
		return true
	}
}

func (r *SFController) IngressKeycloak() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule("keycloak."+r.cr.Spec.FQDN, "keycloak", KC_PORT),
		// TODO: add admin service
	}
}
