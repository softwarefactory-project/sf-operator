// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the keycloak configuration.

package controllers

import (
	_ "embed"

	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const KC_PORT = 8443
const KC_PORT_NAME = "kc-port"
const KC_HTTP_PORT = 8080
const KC_HTTP_PORT_NAME = "kc-http-port"
const KC_IMAGE = "quay.io/software-factory/keycloak:19.0.1-4"
const KC_CERT_MOUNT_PATH = "/keycloak-cert"
const KC_DATA_MOUNT_PATH = "/keycloak-data"
const KC_DEFAULT_REALM = "SF"

//go:embed static/keycloak/post-init.sh
var kcPostInit string

//go:embed static/keycloak/post-init-db.sh
var kcPostInitDB string

//go:embed static/keycloak/entrypoint.sh
var kc_entrypoint string

//go:embed static/keycloak/init.sh
var keycloakInitScript string

func (r *SFController) KCInitContainer() apiv1.Container {
	securityContext := &apiv1.SecurityContext{
		RunAsNonRoot: pointer.BoolPtr(false),
	}
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
		SecurityContext: securityContext,
	}
}

func (r *SFController) KCPostInitDB() bool {
	var job batchv1.Job
	job_name := "kc-post-init-db"
	found := r.GetM(job_name, &job)

	if !found {
		container := apiv1.Container{
			Name:    job_name + "-container",
			Image:   DBImage,
			Command: []string{"sh", "-c", kcPostInitDB},
			Env: []apiv1.EnvVar{
				create_secret_env("DB_PASSWORD", "keycloak-db-password", "keycloak-db-password"),
			},
		}
		job := create_job(r.ns, job_name, container)
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for Keycloak post job " + job_name)
		return false
	}
}

func (r *SFController) KCPostInit() bool {
	var job batchv1.Job
	job_name := "kc-post-init"
	found := r.GetM(job_name, &job)

	if !found {
		vars := []apiv1.EnvVar{
			create_env("KEYCLOAK_ADMIN", "admin"),
			create_env("FQDN", r.cr.Spec.FQDN),
			create_secret_env("KEYCLOAK_ADMIN_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			create_secret_env("KEYCLOAK_SF_ADMIN_PASSWORD", "keycloak-sf-admin-password", "keycloak-sf-admin-password"),
			create_secret_env("KEYCLOAK_SF_SERVICE_PASSWORD", "keycloak-sf-service-password", "keycloak-sf-service-password"),
		}
		if r.cr.Spec.Gerrit.Enabled {
			vars = append(vars,
				create_secret_env("KEYCLOAK_GERRIT_CLIENT_SECRET", "gerrit-kc-client-password", "gerrit-kc-client-password"),
			)
		}
		if r.cr.Spec.Zuul.Enabled {
			vars = append(vars,
				create_env("ZUUL_ENABLED", "true"),
			)
		}
		if r.cr.Spec.OpensearchDashboards.Enabled {
			vars = append(vars,
				create_secret_env("KEYCLOAK_OPENSEARCH_CLIENT_SECRET", "opensearch-kc-client-password", "opensearch-kc-client-password"),
			)
		}

		if r.cr.Spec.Grafana.Enabled {
			vars = append(vars,
				create_secret_env("KEYCLOAK_GRAFANA_CLIENT_SECRET", GRAFANA_IDENT+"-kc-client-password", GRAFANA_IDENT+"-kc-client-password"),
			)
		}

		container := apiv1.Container{
			Name:    job_name + "-container",
			Image:   KC_IMAGE,
			Command: []string{"sh", "-c", kcPostInit},
			Env:     vars,
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
		r.log.V(1).Info("Waiting for Keycloak post job " + job_name)
		return false
	}
}

func (r *SFController) DeployKeycloak() bool {

	if r.IsKeycloakEnabled() {
		// Admin master realm password
		r.GenerateSecretUUID("keycloak-admin-password")
		// Admin SF realm password
		r.GenerateSecretUUID("keycloak-sf-admin-password")
		// SF_SERVICE_USER SF realm password
		r.GenerateSecretUUID("keycloak-sf-service-password")
		// Create a certificate for Keycloak to feed the Keystore
		cert := r.create_client_certificate(
			r.ns, "keycloak-client", "ca-issuer", "keycloak-client-tls", "keycloak")
		r.GetOrCreate(&cert)
		// Ensure Keystore password
		r.GenerateSecretUUID("kc-keystore-password")
		initContainers, _ := r.EnsureDBInit("keycloak")

		securityContext := &apiv1.SecurityContext{
			RunAsNonRoot: pointer.BoolPtr(false),
		}

		dep := create_statefulset(r.ns, "keycloak", KC_IMAGE)
		dep.Spec.Template.Spec.InitContainers = append(initContainers, r.KCInitContainer())
		dep.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", kc_entrypoint}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "keycloak",
				MountPath: KC_DATA_MOUNT_PATH,
			},
		}
		dep.Spec.Template.Spec.Containers[0].SecurityContext = securityContext
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
			create_secret_env("KEYCLOAK_ADMIN_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			create_env("KEYCLOAK_ADMIN", "admin"),
			create_secret_env("KC_KEYSTORE_PASSWORD", "kc-keystore-password", "kc-keystore-password"),
			create_secret_env("MOSQUITTO_SERVICE_USER_PASSWORD", "mosquitto-sf-service-password", "mosquitto-sf-service-password"),
		}
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_https_probe("/health/ready", KC_PORT)
		r.GetOrCreate(&dep)

		// Expose HTTPS port of keycloak
		srv := apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "keycloak",
				Namespace: r.ns,
			},
			Spec: apiv1.ServiceSpec{
				Ports: []apiv1.ServicePort{
					{
						Name:       KC_PORT_NAME,
						Protocol:   apiv1.ProtocolTCP,
						Port:       443,
						TargetPort: intstr.FromInt(KC_PORT),
					},
				},
				Selector: map[string]string{
					"app": "sf",
					"run": "keycloak",
				},
			}}
		r.GetOrCreate(&srv)

		// Export HTTP port of keycloak
		srvhttp := create_service(r.ns, "keycloak-http", "keycloak", KC_HTTP_PORT, KC_HTTP_PORT_NAME)
		r.GetOrCreate(&srvhttp)

		ready := r.IsStatefulSetReady(&dep)
		if ready {
			return r.KCPostInitDB() && r.KCPostInit()
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
	}
}

func (r *SFController) IsKeycloakReady() bool {
	if r.IsKeycloakEnabled() {
		resource := appsv1.StatefulSet{}
		if r.GetM("keycloak", &resource) {
			return r.IsStatefulSetReady(&resource)
		}
		r.log.V(1).Info("Keycloak Stateful Set resource not found")
		return false
	} else {
		r.log.V(1).Info("Keycloak is not enabled")
		return false
	}
}

func (r *SFController) IsKeycloakEnabled() bool {
	// Keycloak is enable if Gerrit or Zuul or Opensearch or Grafana are Enabled
	return r.cr.Spec.Gerrit.Enabled || r.cr.Spec.Zuul.Enabled || r.cr.Spec.Opensearch.Enabled || r.cr.Spec.Grafana.Enabled
}
