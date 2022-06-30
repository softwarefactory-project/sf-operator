// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package controllers

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const IDENT = "gerrit"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"
const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const IMAGE = "quay.io/software-factory/gerrit:3.4.5-2"
const GERRIT_EP_MOUNT_PATH = "/entry"
const GERRIT_GIT_MOUNT_PATH = "/var/gerrit/git"
const GERRIT_INDEX_MOUNT_PATH = "/var/gerrit/index"
const GERRIT_ETC_MOUNT_PATH = "/var/gerrit/etc"
const GERRIT_SSH_MOUNT_PATH = "/var/gerrit/.ssh"
const GERRIT_LOGS_MOUNT_PATH = "/var/gerrit/logs"

// TODO: Attach a config-map for all Gerrit option (like sshd.maxConnectionsPerUser) then
// make entrypoint.sh uses ENV vars exposed by the configMap
const GERRIT_ENTRYPOINT = `
#!/bin/bash

set -ex

# The /dev/./urandom is not a typo. https://stackoverflow.com/questions/58991966/what-java-security-egd-option-is-for
JAVA_OPTIONS="-Djava.security.egd=file:/dev/./urandom"

# Un-comment to clear Gerrit data (we use a statefulset so PVs are kept and re-attached when statefulset is re-created)
# rm -Rf /var/gerrit/git/*
# rm -Rf /var/gerrit/etc/*
# rm -Rf /var/gerrit/etc/.admin_user_created
# rm -Rf /var/gerrit/.ssh/*

echo "Initializing Gerrit site ..."
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war init -d /var/gerrit --batch --no-auto-start --skip-plugins
java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war reindex -d /var/gerrit

echo "Installing plugins ..."
unzip -jo /var/gerrit/bin/gerrit.war WEB-INF/plugins/* -d /var/gerrit/plugins
for plugin in /var/gerrit-plugins/*; do
		cp -uv $plugin /var/gerrit/plugins/
done

echo "Creating admin account if needed"
cat << EOF > /var/gerrit/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF
if [ ! -f /var/gerrit/etc/.admin_user_created ]; then
	pynotedb create-admin-user --email "admin@${FQDN}" --pubkey "${GERRIT_ADMIN_SSH_PUB}" \
	  --all-users "/var/gerrit/git/All-Users.git" --scheme gerrit

	echo "Copy Gerrit Admin SSH keys on filesystem"
	echo "${GERRIT_ADMIN_SSH_PUB}" > /var/gerrit/.ssh/gerrit_admin.pub
	chmod 0444 /var/gerrit/.ssh/gerrit_admin.pub
  echo "${GERRIT_ADMIN_SSH}" > /var/gerrit/.ssh/gerrit_admin
	chmod 0400 /var/gerrit/.ssh/gerrit_admin

	cat << EOF > /var/gerrit/.ssh/config
Host gerrit
User admin
Hostname ${HOSTNAME}
Port 29418
IdentityFile /var/gerrit/.ssh/gerrit_admin
EOF
touch /var/gerrit/etc/.admin_user_created
else
	echo "Admin user already initialized"
fi

echo "Setting Gerrit config file ..."
git config -f /var/gerrit/etc/gerrit.config --replace-all auth.type "DEVELOPMENT_BECOME_ANY_ACCOUNT"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.listenaddress "*:29418"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.idleTimeout "2d"
git config -f /var/gerrit/etc/gerrit.config --replace-all sshd.maxConnectionsPerUser "${SSHD_MAX_CONNECTIONS_PER_USER:-10}"

echo "Running Gerrit ..."
exec java ${JAVA_OPTIONS} -jar /var/gerrit/bin/gerrit.war daemon -d /var/gerrit
`

func (r *SFController) GerritPostInitJob(name string) bool {
	var job batchv1.Job
	job_name := IDENT + "-" + name
	found := r.GetM(job_name, &job)

	postInitScript := `
	#!/bin/bash

	set -ex

	env

	mkdir /var/gerrit/.ssh
	chmod 0700 /var/gerrit/.ssh

	echo "${GERRIT_ADMIN_SSH}" > /var/gerrit/.ssh/gerrit_admin
	chmod 0400 /var/gerrit/.ssh/gerrit_admin

	cat << EOF > /var/gerrit/.ssh/config
Host gerrit
User admin
Hostname ${GERRIT_SSHD_PORT_29418_TCP_ADDR}
Port ${GERRIT_SSHD_SERVICE_PORT_GERRIT_SSHD}
IdentityFile /var/gerrit/.ssh/gerrit_admin
StrictHostKeyChecking no
EOF

	echo "Ensure we can connect to Gerrit ssh port"
	ssh gerrit gerrit version

  cat << EOF > /var/gerrit/.gitconfig
[user]
    name = SF initial configurator
    email = admin@${FQDN}
[gitreview]
    username = admin
[push]
    default = simple
EOF

	echo ""
	mkdir /tmp/All-projects && cd /tmp/All-projects
	git init .
	git remote add origin ssh://gerrit/All-Projects
	git fetch origin refs/meta/config:refs/remotes/origin/meta/config
	git checkout meta/config
	git reset --hard origin/meta/config
	gitConfig="git config -f project.config --replace-all "
	${gitConfig} capability.accessDatabase "group Administrators"
	${gitConfig} access.refs/*.push "group Administrators" ".*group Administrators"
	${gitConfig} access.refs/for/*.addPatchSet "group Administrators" "group Administrator"
	${gitConfig} access.refs/for/*.addPatchSet "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/heads/*.push "+force group Administrators" ".*group Administrators"
	${gitConfig} access.refs/heads/*.push "+force group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Service Users" ".*group Service Users"
	${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Administrators" ".*group Administrators"
	${gitConfig} access.refs/heads/*.label-Verified "-2..+2 group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/heads/*.label-Workflow "-1..+1 group Administrators" ".*group Administrators"
	${gitConfig} access.refs/heads/*.label-Workflow "-1..+1 group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/heads/*.submit "group Service Users" "group Service Users"
	${gitConfig} access.refs/heads/*.rebase "group Administrators" "group Administrators"
	${gitConfig} access.refs/heads/*.rebase "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/heads/*.rebase "group Service Users" "group Service Users"
	${gitConfig} access.refs/heads/*.abandon "group Administrators" "group Administrators"
	${gitConfig} access.refs/heads/*.abandon "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/meta/config.read "group Registered Users" "group Registered Users"
	${gitConfig} access.refs/meta/config.read "group Anonymous Users" "group Anonymous Users"
	${gitConfig} access.refs/meta/config.rebase "group Administrators" "group Administrators"
	${gitConfig} access.refs/meta/config.rebase "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/meta/config.abandon "group Administrators" "group Administrators"
	${gitConfig} access.refs/meta/config.abandon "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/meta/config.label-Verified "-2..+2 group Administrators" ".*group Administrators"
	${gitConfig} access.refs/meta/config.label-Verified "-2..+2 group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/meta/config.label-Workflow "-1..+1 group Administrators" ".*group Administrators"
	${gitConfig} access.refs/meta/config.label-Workflow "-1..+1 group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/tags/*.pushTag "+force group Administrators" ".*group Administrators"
	${gitConfig} access.refs/tags/*.pushTag "+force group Project Owners" ".*group Project Owners"
	${gitConfig} access.refs/tags/*.pushAnnotatedTag "group Administrators" "group Administrators"
	${gitConfig} access.refs/tags/*.pushAnnotatedTag "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/tags/*.pushSignedTag "group Administrators" "group Administrators"
	${gitConfig} access.refs/tags/*.pushSignedTag "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/tags/*.forgeAuthor "group Administrators" "group Administrators"
	${gitConfig} access.refs/tags/*.forgeAuthor "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/tags/*.forgeCommitter "group Administrators" "group Administrators"
	${gitConfig} access.refs/tags/*.forgeCommitter "group Project Owners" "group Project Owners"
	${gitConfig} access.refs/tags/*.push "group Administrators" "group Administrators"
	${gitConfig} access.refs/tags/*.push "group Project Owners" "group Project Owners"
	${gitConfig} label.Code-Review.copyAllScoresIfNoCodeChange "true"
	${gitConfig} label.Code-Review.value "-2 Do not submit" "-2.*"
	${gitConfig} label.Code-Review.value "-1 I would prefer that you didn't submit this" "-1.*"
	${gitConfig} label.Code-Review.value "+2 Looks good to me (core reviewer)" "\+2.*"
	${gitConfig} label.Verified.value "-2 Fails" "-2.*"
	${gitConfig} label.Verified.value "-1 Doesn't seem to work" "-1.*"
	${gitConfig} label.Verified.value "0 No score" "0.*"
	${gitConfig} label.Verified.value "+1 Works for me" "\+1.*"
	${gitConfig} label.Verified.value "+2 Verified" "\+2.*"
	${gitConfig} label.Workflow.value "-1 Work in progress" "-1.*"
	${gitConfig} label.Workflow.value "0 Ready for reviews" "0.*"
	${gitConfig} label.Workflow.value "+1 Approved" "\+1.*"
	${gitConfig} plugin.reviewers-by-blame.maxReviewers "5" ".*"
	${gitConfig} plugin.reviewers-by-blame.ignoreDrafts "true" ".*"
	${gitConfig} plugin.reviewers-by-blame.ignoreSubjectRegEx "'(WIP|DNM)(.*)'" ".*"

	git add project.config
	git commit -m"Set SF default Gerrit ACLs" && git push origin meta/config:meta/config || true
	`

	containerCommand := []string{
		"/bin/sh",
		"-c",
		"echo \"${GERRIT_INIT_SCRIPT}\" > /tmp/init.sh && bash /tmp/init.sh"}

	if !found {
		container := apiv1.Container{
			Name:    fmt.Sprintf("%s-container", job_name),
			Image:   IMAGE,
			Command: containerCommand,
			Env: []apiv1.EnvVar{
				create_env("GERRIT_INIT_SCRIPT", postInitScript),
				create_env("FQDN", r.cr.Spec.FQDN),
				create_secret_env("GERRIT_ADMIN_SSH", "gerrit-admin-ssh-key", "priv"),
			},
		}
		job := create_job(r.ns, job_name, container)
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

func (r *SFController) DeployGerrit(enabled bool) bool {
	if enabled {
		// r.log.V(1).Info("Deploying " + IDENT)

		// Set entrypoint.sh in a config map
		cm_ep_data := make(map[string]string)
		cm_ep_data["entrypoint.sh"] = GERRIT_ENTRYPOINT
		r.EnsureConfigMap("gerrit-ep", cm_ep_data)

		// Set Gerrit env vars in a config map
		cm_config_data := make(map[string]string)
		// Those variables should be populated via the SoftwareFactory CRD Spec
		cm_config_data["SSHD_MAX_CONNECTIONS_PER_USER"] = "20"
		cm_config_data["FQDN"] = r.cr.Spec.FQDN
		r.EnsureConfigMap("gerrit-config", cm_config_data)

		// Ensure Gerrit Admin user ssh key
		r.EnsureSSHKey("gerrit-admin-ssh-key")

		// Create the deployment
		dep := create_statefulset(r.ns, IDENT, IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{"/bin/bash", "/entry/entrypoint.sh"}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      IDENT + "-ep",
				MountPath: GERRIT_EP_MOUNT_PATH,
			},
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
			{
				Name:      IDENT + "-ssh",
				MountPath: GERRIT_SSH_MOUNT_PATH,
			},
			{
				Name:      IDENT + "-logs",
				MountPath: GERRIT_LOGS_MOUNT_PATH,
			},
		}

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, IDENT+"-git"),
			create_pvc(r.ns, IDENT+"-index"),
			create_pvc(r.ns, IDENT+"-config"),
			create_pvc(r.ns, IDENT+"-ssh"),
			create_pvc(r.ns, IDENT+"-logs"),
		)

		// This port definition is informational all ports exposed by the container
		// will be available to the network.
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
			create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
		}

		// Expose env vars
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("GERRIT_ADMIN_SSH", "gerrit-admin-ssh-key", "priv"),
			create_secret_env("GERRIT_ADMIN_SSH_PUB", "gerrit-admin-ssh-key", "pub"),
		}

		// Expose env vars from a config map
		dep.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{
			{
				ConfigMapRef: &apiv1.ConfigMapEnvSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "gerrit-config-config-map",
					},
				},
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(IDENT+"-ep", IDENT+"-ep-config-map"),
		}
		r.Apply(&dep)

		// Create services exposed by Gerrit
		httpd_service := create_service(r.ns, GERRIT_HTTPD_PORT_NAME, IDENT, GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME)
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
					"run": IDENT,
				},
			}}
		r.Apply(&httpd_service)
		r.Apply(&sshd_service)

		// Wait for the service to be ready.
		r.GetM(IDENT, &dep)
		if dep.Status.ReadyReplicas > 0 {
			return r.GerritPostInitJob("post-init")
		} else {
			r.log.V(1).Info("Waiting for Gerrit to be ready...")
			return false
		}
	} else {
		r.DeleteStatefulSet(IDENT)
		r.DeleteService(GERRIT_HTTPD_PORT_NAME)
		r.DeleteService(GERRIT_SSHD_PORT_NAME)
		return true
	}
}

func (r *SFController) IngressGerrit() []netv1.IngressRule {
	return []netv1.IngressRule{
		create_ingress_rule(IDENT+"."+r.cr.Spec.FQDN, GERRIT_HTTPD_PORT_NAME, GERRIT_HTTPD_PORT),
	}
}
