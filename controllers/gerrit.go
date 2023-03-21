// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/utils/pointer"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const GERRIT_IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"

const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const GERRIT_IMAGE = "quay.io/software-factory/gerrit:3.5.4-1"
const GERRIT_EP_MOUNT_PATH = "/entry"
const GERRIT_SITE_MOUNT_PATH = "/gerrit"
const GERRIT_CERT_MOUNT_PATH = "/gerrit-cert"

//go:embed static/gerrit/post-init.sh
var postInitScript string

//go:embed static/gerrit/set-ci-user.sh
var setCIUser string

//go:embed static/gerrit/entrypoint.sh
var entrypoint string

//go:embed static/gerrit/init.sh
var gerritInitScript string

func (r *SFController) GerritInitContainers(volumeMounts []apiv1.VolumeMount, spec sfv1.GerritSpec) []apiv1.Container {
	var sshd_max_conns_per_user string
	if spec.SshdMaxConnectionsPerUser == "" {
		sshd_max_conns_per_user = "64"
	} else {
		sshd_max_conns_per_user = spec.SshdMaxConnectionsPerUser
	}
	certVolume := apiv1.VolumeMount{
		Name:      GERRIT_IDENT + "-client-tls",
		MountPath: GERRIT_CERT_MOUNT_PATH,
		ReadOnly:  true,
	}

	securityContext := &apiv1.SecurityContext{
		RunAsNonRoot: pointer.BoolPtr(false),
	}
	container := apiv1.Container{
		Name:    "gerrit-init",
		Image:   GERRIT_IMAGE,
		Command: []string{"sh", "-c", gerritInitScript},
		Env: []apiv1.EnvVar{
			create_secret_env("GERRIT_KEYSTORE_PASSWORD", "gerrit-keystore-password", "gerrit-keystore-password"),
			create_secret_env("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
			create_env("SSHD_MAX_CONNECTIONS_PER_USER", sshd_max_conns_per_user),
			create_env("FQDN", r.cr.Spec.FQDN),
		},
		VolumeMounts:    append(volumeMounts, certVolume),
		SecurityContext: securityContext,
	}
	return []apiv1.Container{container}
}

func (r *SFController) GerritPostInitJob(name string, has_config_repo bool) bool {
	var job batchv1.Job
	job_name := GERRIT_IDENT + "-" + name
	found := r.GetM(job_name, &job)

	if !found {
		cm_data := make(map[string]string)
		cm_data["set-ci-user.sh"] = setCIUser
		cm_data["sf.dhall"] = sfDhall
		cm_data["resources.dhall"] = resourcesDhall
		r.EnsureConfigMap("gerrit-pi", cm_data)

		env := []apiv1.EnvVar{
			create_env("FQDN", r.cr.Spec.FQDN),
			create_env("HAS_CONFIG_REPOSITORY", strconv.FormatBool(has_config_repo)),
			create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
			create_secret_env("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
		}
		ci_users := []apiv1.EnvVar{}

		ci_users = append(
			ci_users,
			create_secret_env("CI_USER_SSH_zuul", "zuul-ssh-key", "pub"),
			create_secret_env("CI_USER_API_zuul", "zuul-gerrit-api-key", "zuul-gerrit-api-key"))

		container := apiv1.Container{
			Name:    fmt.Sprintf("%s-container", job_name),
			Image:   BUSYBOX_IMAGE,
			Command: []string{"sh", "-c", postInitScript},
			Env:     append(env, ci_users...),
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      GERRIT_IDENT + "-pi",
					MountPath: "/entry",
				},
			},
		}
		job := create_job(r.ns, job_name, container)
		job.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(GERRIT_IDENT+"-pi", GERRIT_IDENT+"-pi-config-map"),
		}
		r.log.V(1).Info("Creating Gerrit post init job", "name", name)
		r.CreateR(&job)
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for Gerrit post init job result")
		return false
	}
}

func (r *SFController) DeployGerrit() bool {

	spec := r.cr.Spec.Gerrit

	has_config_repo := r.cr.Spec.ConfigLocations.ConfigRepo == ""

	// Ensure Gerrit Keystore password
	r.GenerateSecretUUID("gerrit-keystore-password")
	// Ensure Gerrit Admin API password
	r.GenerateSecretUUID("gerrit-admin-api-key")

	// Create a certificate for Gerrit
	cert := r.create_client_certificate(r.ns, GERRIT_IDENT+"-client", "ca-issuer", GERRIT_IDENT+"-client-tls", "gerrit")
	r.GetOrCreate(&cert)

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      GERRIT_IDENT,
			MountPath: GERRIT_SITE_MOUNT_PATH,
		},
	}
	securityContext := &apiv1.SecurityContext{
		RunAsNonRoot: pointer.BoolPtr(false),
	}
	podSecurityContext := &apiv1.PodSecurityContext{
		RunAsUser: pointer.Int64(10002),
		FSGroup:   pointer.Int64(10002),
	}

	// Create the deployment
	dep := create_statefulset(r.ns, GERRIT_IDENT, GERRIT_IMAGE, get_storage_classname(r.cr.Spec))

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", entrypoint}
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
		create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
	}
	dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		create_secret_env("GERRIT_KEYSTORE_PASSWORD", "gerrit-keystore-password", "gerrit-keystore-password"),
		create_env("FQDN", r.cr.Spec.FQDN),
		create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
		create_secret_env("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
	}
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", GERRIT_HTTPD_PORT)
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GERRIT_SSHD_PORT)
	dep.Spec.Template.Spec.Containers[0].SecurityContext = securityContext
	dep.Spec.Template.Spec.SecurityContext = podSecurityContext

	dep.Spec.Template.Spec.InitContainers = r.GerritInitContainers(volumeMounts, spec)

	// Expose a volume that contain certmanager certs for Gerrit
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_secret(GERRIT_IDENT + "-client-tls"),
	}

	// Create annotations based on Gerrit parameters
	jsonSpec, _ := json.Marshal(spec)
	annotations := map[string]string{
		"fqdn": r.cr.Spec.FQDN,
		"spec": string(jsonSpec),
	}
	dep.Spec.Template.ObjectMeta.Annotations = annotations
	r.GetOrCreate(&dep)
	if !map_equals(&dep.Spec.Template.ObjectMeta.Annotations, &annotations) {
		// Update the annotation - this force the statefulset controler to respawn the container
		dep.Spec.Template.ObjectMeta.Annotations = annotations
		// ReInit initContainers to ensure new spec is used
		dep.Spec.Template.Spec.InitContainers = r.GerritInitContainers(volumeMounts, spec)
		r.log.V(1).Info("Gerrit configuration changed, restarting ...")
		// Update the deployment resource
		r.UpdateR(&dep)
	}

	// Create services exposed by Gerrit
	httpd_service := create_service(
		r.ns, GERRIT_HTTPD_PORT_NAME, GERRIT_IDENT, GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME)
	sshd_service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_SSHD_PORT_NAME,
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     GERRIT_SSHD_PORT_NAME,
					Protocol: apiv1.ProtocolTCP,
					Port:     GERRIT_SSHD_PORT,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": GERRIT_IDENT,
			},
		}}
	r.GetOrCreate(&httpd_service)
	r.GetOrCreate(&sshd_service)

	ready := r.IsStatefulSetReady(&dep)
	if ready {
		return r.GerritPostInitJob("post-init", has_config_repo)
	} else {
		return false
	}
}

func (r *SFController) setupGerritIngress() {
	rule := create_ingress_rule(
		GERRIT_IDENT+"."+r.cr.Spec.FQDN, GERRIT_HTTPD_PORT_NAME, GERRIT_HTTPD_PORT, "/")

	r.ensureHTTPSIngress(r.cr.Name+"-gerrit", rule, map[string]string{})
}
