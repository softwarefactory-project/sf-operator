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

const KC_PORT = 8443
const KC_PORT_NAME = "kc-port"
const KC_HTTP_PORT = 8080
const KC_HTTP_PORT_NAME = "kc-http-port"
const KC_IMAGE = "quay.io/keycloak/keycloak:19.0.1"
const KC_CERT_MOUNT_PATH = "/keycloak-cert"
const KC_DATA_MOUNT_PATH = "/keycloak-data"

//go:embed static/keycloak/post-init.sh
var kcPostInit string

//go:embed static/keycloak/entrypoint.sh
var kc_entrypoint string

//go:embed static/keycloak/init.sh
var keycloakInitScript string

func (r *SFController) KCInitContainer() apiv1.Container {
	return apiv1.Container{
		Name:    "keycloak-init",
		Image:   BUSYBOX_IMAGE,
		Command: []string{"sh", "-c", keycloakInitScript},
		Env: []apiv1.EnvVar{
			create_secret_env("KC_KEYSTORE_PASSWORD", "kc-keystore-password", "kc-keystore-password"),
			create_env("FQDN", r.cr.Spec.FQDN),
		},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "keycloak-client-tls",
				MountPath: KC_CERT_MOUNT_PATH,
				ReadOnly:  true,
			},
			{
				Name:      "keycloak",
				MountPath: KC_DATA_MOUNT_PATH,
			},
		},
	}
}

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
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "keycloak",
					MountPath: KC_DATA_MOUNT_PATH,
				},
			},
		}
		job := create_job(r.ns, job_name, container)
		job.Spec.Template.Spec.Volumes = []apiv1.Volume{
			{
				Name: "keycloak",
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: "keycloak-keycloak-0",
					},
				},
			},
		}
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for Keycloak PostInit result")
		return false
	}
}

func (r *SFController) DeployKeycloak(enabled bool) bool {
	if enabled {
		r.GenerateSecretUUID("keycloak-admin-password")
		// Create a certificate for Keycloak to feed the Keystore
		cert := r.create_client_certificate(
			r.ns, "keycloak-client", "ca-issuer", "keycloak-client-tls", "keycloak")
		r.GetOrCreate(&cert)
		// Ensure Keystore password
		r.GenerateSecretUUID("kc-keystore-password")
		initContainers, _ := r.EnsureDBInit("keycloak")
		dep := create_statefulset(r.ns, "keycloak", KC_IMAGE)
		dep.Spec.Template.Spec.InitContainers = append(initContainers, r.KCInitContainer())
		dep.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", kc_entrypoint}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "keycloak",
				MountPath: KC_DATA_MOUNT_PATH,
			},
		}
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_secret("keycloak-client-tls"),
		}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(KC_PORT, KC_PORT_NAME),
			create_container_port(KC_HTTP_PORT, KC_HTTP_PORT_NAME),
		}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("INGRESS_HOSTNAME", "keycloak."+r.cr.Spec.FQDN),
			create_secret_env("DB_PASSWORD", "keycloak-db-password", "keycloak-db-password"),
			create_env("KEYCLOAK_ADMIN", "admin"),
			create_secret_env("KEYCLOAK_ADMIN_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			create_secret_env("KC_KEYSTORE_PASSWORD", "kc-keystore-password", "kc-keystore-password"),
		}
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_https_probe("/health/ready", KC_PORT)
		r.GetOrCreate(&dep)
		srv := create_service(r.ns, "keycloak", "keycloak", KC_PORT, KC_PORT_NAME)
		r.GetOrCreate(&srv)
		srvhttp := create_service(r.ns, "keycloak-http", "keycloak", KC_HTTP_PORT, KC_HTTP_PORT_NAME)
		r.GetOrCreate(&srvhttp)

		ready := r.IsStatefulSetReady(&dep)
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
		create_ingress_rule("keycloak."+r.cr.Spec.FQDN, "keycloak-http", KC_HTTP_PORT),
		// TODO: add admin service
	}
}
