// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	"bytes"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

const IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"
const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const IMAGE = "quay.io/software-factory/gerrit:3.4.3-0"
const JAVA_OPTIONS = "-Djava.security.egd=file:/dev/./urandom"
const GERRIT_GIT_MOUNT_PATH = "/var/gerrit/git"
const GERRIT_INDEX_MOUNT_PATH = "/var/gerrit/index"
const GERRIT_ETC_MOUNT_PATH = "/var/gerrit/etc"

type GerritOptions struct {
	HttpdPort int
	SshdPort  int
}

// TODO: As the gerrit.config file is edited by gerrit we need to refactor this way:
// Instead create the entrypoint.sh file add it as a configmap and mount the config map
// in a volume. The entrypoint.sh is a template string that we use to init gerrit but
// also to set its config file with git config -f /var/gerrit/etc/gerrit.config
// The gerrit.config is populated after the gerrit init.

const GERRIT_CONFIG_TEMPLATE = `
[gerrit]
        basePath = git
[database]
        type = h2
        database = db/ReviewDB
[index]
        type = LUCENE
[auth]
        type = DEVELOPMENT_BECOME_ANY_ACCOUNT
[sendemail]
        smtpServer = localhost
[sshd]
        listenAddress = *:{{ .SshdPort }}
[httpd]
        listenUrl = http://*:{{ .HttpdPort }}/
        filterClass = com.googlesource.gerrit.plugins.ootb.FirstTimeRedirect
        firstTimeRedirectUrl = /login/%23%2F?account_id=1000000
[cache]
        directory = cache
[plugins]
        allowRemoteAdmin = true
`

var entrypoint = []string{"/bin/bash", "-c",
	fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war init -d /var/gerrit --batch --no-auto-start --skip-plugins", JAVA_OPTIONS) +
		" && " + fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war reindex -d /var/gerrit", JAVA_OPTIONS) +
		" && " + fmt.Sprintf("/usr/bin/java %s -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit", JAVA_OPTIONS)}

func CreateGerritConfig(options GerritOptions) string {
	buf := &bytes.Buffer{}
	t, err := template.New("config").Parse(GERRIT_CONFIG_TEMPLATE)
	if err != nil {
		panic(err)
	}
	err = t.Execute(buf, options)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func (r *SFController) DeployGerrit(enabled bool) bool {
	var dep appsv1.StatefulSet
	found := r.GetM(IDENT, &dep)
	if enabled && !found {
		r.log.V(1).Info("Deploying " + IDENT)
		// Create config file in config map
		var gerritConfigFile = CreateGerritConfig(GerritOptions{
			HttpdPort: GERRIT_HTTPD_PORT,
			SshdPort:  GERRIT_SSHD_PORT})
		r.EnsureConfigMap("gerrit", "gerrit.config", gerritConfigFile)
		// Create the deployment
		dep = create_statefulset(r.ns, IDENT, IMAGE)
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      IDENT + "-git",
				MountPath: GERRIT_GIT_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-index",
				MountPath: GERRIT_INDEX_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-config",
				MountPath: GERRIT_ETC_MOUNT_PATH,
			},
		}

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, IDENT+"-git"),
			create_pvc(r.ns, IDENT+"-index"),
			create_pvc(r.ns, IDENT+"-config"))

		// This port defintion is informational all ports exposed by the container
		// will be available to the network.
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
		}
		dep.Spec.Template.Spec.Containers[0].Command = entrypoint
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(IDENT+"-config", IDENT+"-config-map"),
		}
		r.CreateR(&dep)
		srv := create_service(r.ns, IDENT, GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME)
		r.CreateR(&srv)
	}
	if !enabled && found {
		r.log.V(1).Info("Destroying " + IDENT)
		if err := r.Delete(r.ctx, &dep); err != nil {
			panic(err.Error())
		}
	}
	if enabled && found {
		// Wait for the service to be ready.
		return (dep.Status.ReadyReplicas > 0)
	}
	return true
}
