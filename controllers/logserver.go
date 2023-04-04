// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the logserver configuration.

package controllers

import (
	_ "embed"
	"encoding/base64"

	"k8s.io/utils/pointer"

	apiv1 "k8s.io/api/core/v1"
)

const LOGSERVER_IDENT = "logserver"
const LOGSERVER_HTTPD_PORT = 8080
const LOGSERVER_HTTPD_PORT_NAME = "logserver-httpd"
const LOGSERVER_IMAGE = "registry.access.redhat.com/rhscl/httpd-24-rhel7:latest"

const LOGSERVER_SSHD_PORT = 2222
const LOGSERVER_SSHD_PORT_NAME = "logserver-sshd"
const LOGSERVER_SSHD_IMAGE = "quay.io/software-factory/rsync-server:1-2"

const CONTAINER_HTTP_BASE_DIR = "/opt/rh/httpd24/root"

const LOGSERVER_DATA = "/var/www"

//go:embed static/logserver/logserver.conf.tmpl
var logserverconf string

func (r *SFController) DeployLogserver() bool {

	cm_data := make(map[string]string)
	cm_data["logserver.conf"], _ = parse_string(logserverconf, struct {
		ServerPort    int
		ServerRoot    string
		LogserverRoot string
	}{
		ServerPort:    LOGSERVER_HTTPD_PORT,
		ServerRoot:    CONTAINER_HTTP_BASE_DIR,
		LogserverRoot: LOGSERVER_DATA,
	})
	cm_data["index.html"] = ""

	r.EnsureConfigMap(LOGSERVER_IDENT, cm_data)

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      LOGSERVER_IDENT + "-config-vol",
			MountPath: "/etc/httpd/conf.d/logserver.conf",
			ReadOnly:  true,
			SubPath:   "logserver.conf",
		},
		{
			Name:      LOGSERVER_IDENT + "-config-vol",
			MountPath: CONTAINER_HTTP_BASE_DIR + LOGSERVER_DATA + "/index.html",
			ReadOnly:  true,
			SubPath:   "index.html",
		},
		{
			Name:      LOGSERVER_IDENT,
			MountPath: CONTAINER_HTTP_BASE_DIR + LOGSERVER_DATA + "/logs",
		},
	}

	securityContext := &apiv1.SecurityContext{
		RunAsNonRoot: pointer.BoolPtr(true),
	}

	// Create the deployment
	dep := create_statefulset(r.ns, LOGSERVER_IDENT, LOGSERVER_IMAGE, get_storage_classname(r.cr.Spec))

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	// Expose volumes
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm(LOGSERVER_IDENT+"-config-vol", LOGSERVER_IDENT+"-config-map"),
	}

	dep.Spec.VolumeClaimTemplates = append(dep.Spec.VolumeClaimTemplates, create_pvc(r.ns, LOGSERVER_IDENT+"-keys", get_storage_classname(r.cr.Spec)))

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(LOGSERVER_HTTPD_PORT, LOGSERVER_HTTPD_PORT_NAME),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", LOGSERVER_HTTPD_PORT)

	dep.Spec.Template.Spec.Containers[0].SecurityContext = securityContext

	// Create services exposed by logserver
	httpd_service := create_service(
		r.ns, LOGSERVER_HTTPD_PORT_NAME, LOGSERVER_IDENT, LOGSERVER_HTTPD_PORT, LOGSERVER_HTTPD_PORT_NAME)

	r.GetOrCreate(&httpd_service)

	// Side Car Container

	volumeMounts_sidecar := []apiv1.VolumeMount{
		{
			Name:      LOGSERVER_IDENT,
			MountPath: "/home/data/rsync",
		},
		{
			Name:      LOGSERVER_IDENT + "-keys",
			MountPath: "/opt/ssh/keys",
		},
	}

	ports_sidecar := []apiv1.ContainerPort{
		create_container_port(LOGSERVER_SSHD_PORT, LOGSERVER_SSHD_PORT_NAME),
	}

	pub_key, err := r.getSecretDataFromKey("zuul-ssh-key", "pub")
	if err != nil {
		r.log.V(1).Error(err, "Unable to find the secret for the logserver ssh sidecar")
		return false
	}
	pub_key_b64 := base64.StdEncoding.EncodeToString(pub_key)

	env_sidecar := []apiv1.EnvVar{
		create_env("FQDN", r.cr.Spec.FQDN),
		create_env("AUTHORIZED_KEY_1", pub_key_b64),
	}

	// Setup the sidecar container for sshd
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, apiv1.Container{
		Name:         LOGSERVER_SSHD_PORT_NAME,
		Image:        LOGSERVER_SSHD_IMAGE,
		Command:      []string{"/entrypoint.sh"},
		VolumeMounts: volumeMounts_sidecar,
		Env:          env_sidecar,
		Ports:        ports_sidecar,
	})

	r.GetOrCreate(&dep)

	sshd_service := create_service(r.ns, LOGSERVER_SSHD_PORT_NAME, LOGSERVER_IDENT, LOGSERVER_SSHD_PORT, LOGSERVER_SSHD_PORT_NAME)
	r.GetOrCreate(&sshd_service)

	ready := r.IsStatefulSetReady(&dep)
	if ready {
		return true
	} else {
		return false
	}
}

func (r *SFController) setupLogserverIngress() {
	r.ensureHTTPSRoute(r.cr.Name+"-logserver", LOGSERVER_IDENT, LOGSERVER_HTTPD_PORT_NAME, "/", LOGSERVER_HTTPD_PORT, map[string]string{})
}
