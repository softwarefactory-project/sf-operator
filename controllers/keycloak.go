// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the keycloak configuration.

package controllers

import (
	_ "embed"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const KC_PORT = 8080
const KC_PORT_NAME = "kc-port"
const KC_IMAGE = "quay.io/software-factory/keycloak:15.0.2"
const KC_DIR = "/opt/jboss/keycloak"

//go:embed static/keycloak/standalone.xml
var kc_config string

func (r *SFController) KCAdminJob(name string, command string) bool {
	var job batchv1.Job
	job_name := "kcadm-" + name
	found := r.GetM(job_name, &job)

	kcadm := fmt.Sprintf(
		"%s/bin/kcadm.sh %s --no-config --password $KC_ADMIN_PASS --realm master --server http://keycloak:%d/auth --user admin",
		KC_DIR, command, KC_PORT,
	)

	if !found {
		container := apiv1.Container{
			Name:  "kcadm-client",
			Image: KC_IMAGE,
			Command: []string{"sh", "-c", fmt.Sprintf(`
echo 'Running: %s'
set -xe
%s
`, kcadm, kcadm)},
			Env: []apiv1.EnvVar{
				create_secret_env("KC_ADMIN_PASS", "keycloak-admin-password", "keycloak-admin-password"),
			},
		}
		job := create_job(r.ns, job_name, container)

		r.log.V(1).Info("Creating job to ensure db", "name", name)
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
		r.EnsureSecret("keycloak-admin-password")
		_, db_ready := r.EnsureDB("keycloak")
		if db_ready {
			r.log.V(1).Info("Keycloak DB is ready")
			cm_data := make(map[string]string)
			cm_data["standalone.xml"] = kc_config
			r.EnsureConfigMap("keycloak", cm_data)
			dep := create_deployment(r.ns, "keycloak", "quay.io/software-factory/keycloak:15.0.2")
			dep.Spec.Template.Spec.Containers[0].Command = []string{
				// It seems like the entrypoint takes care of creating the initial admin user,
				// but it may be doing too much. Instead we should run the standalone.sh script,
				// and create the admin user manually using
				//   /opt/jboss/keycloak/bin/add-user-keycloak.sh
				"/opt/jboss/tools/docker-entrypoint.sh", "--server-config", "../../../../../../../etc/keycloak/standalone.xml"}
			dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
				create_container_port(KC_PORT, KC_PORT_NAME),
			}
			dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/keycloak",
				},
			}
			dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
				create_volume_cm("config-volume", "keycloak-config-map"),
			}
			dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
				create_env("KEYCLOAK_FRONTEND_URL", "https://"+r.cr.Spec.FQDN+"/auth"),
				create_env("PROXY_ADDRESS_FORWARDING", "true"),
				create_env("KEYCLOAK_HTTP_PORT", fmt.Sprintf("%v", KC_PORT)),
				create_env("DB_VENDOR", "mysql"),
				create_env("DB_ADDR", "mariadb"),
				create_env("DB_USER", "keycloak"),
				create_secret_env("DB_PASSWORD", "keycloak-db-password", "keycloak-db-password"),
				create_env("KEYCLOAK_USER", "admin"),
				create_secret_env("KEYCLOAK_PASSWORD", "keycloak-admin-password", "keycloak-admin-password"),
			}
			dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9990)
			r.Apply(&dep)
			srv := create_service(r.ns, "keycloak", "keycloak", KC_PORT, KC_PORT_NAME)
			r.Apply(&srv)

			ready := r.IsDeploymentReady("keycloak")
			if ready {
				return r.KCAdminJob("create-default-realm", "create realms --set realm=SF --set enabled=true")
			}
		}
		return false
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
