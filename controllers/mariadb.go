// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the mariadb configuration.

package controllers

import (
	apiv1 "k8s.io/api/core/v1"
)

const DBImage = "quay.io/software-factory/mariadb:10.5.16-4"

const MARIADB_PORT = 3306
const MARIADB_PORT_NAME = "mariadb-port"

func (r *SFController) EnsureDBInit(name string) ([]apiv1.Container, apiv1.Secret) {
	db_password := r.GenerateSecretUUID(name + "-db-password")
	c := "CREATE DATABASE IF NOT EXISTS " + name + " CHARACTER SET utf8 COLLATE utf8_general_ci; "
	g := "GRANT ALL PRIVILEGES ON " + name + ".* TO '" + name + "'@'%' IDENTIFIED BY '${USER_PASSWORD}' WITH GRANT OPTION; FLUSH PRIVILEGES;"
	container := apiv1.Container{
		Name:            "mariadb-client",
		Image:           DBImage,
		SecurityContext: create_security_context(false),
		Command: []string{"sh", "-c", `
echo 'Running: mysql --host=mariadb --user=root --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"'
ATTEMPT=0
while ! mysql --host=mariadb --user=root --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"; do
    ATTEMPT=$[ $ATTEMPT + 1 ]
    if test $ATTEMPT -eq 10; then
        echo "Failed after $ATTEMPT attempt";
        exit 1
    fi
    sleep 10;
done
`},
		Env: []apiv1.EnvVar{
			Create_secret_env("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
			Create_secret_env("USER_PASSWORD", name+"-db-password", name+"-db-password"),
		},
	}
	return []apiv1.Container{container}, db_password
}

func (r *SFController) DeployMariadb() bool {
	pass_name := "mariadb-root-password"
	r.GenerateSecretUUID(pass_name)

	replicas := int32(1)
	dep := r.create_statefulset("mariadb", DBImage, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.DBStorage), replicas)

	dep.Spec.VolumeClaimTemplates = append(
		dep.Spec.VolumeClaimTemplates,
		// TODO redirect logs to stdout so we don't need a volume
		r.create_pvc("mariadb-logs", r.getStorageConfOrDefault(r.cr.Spec.MariaDB.LogStorage)))
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      "mariadb",
			MountPath: "/var/lib/mysql",
		},
		{
			Name:      "mariadb-logs",
			MountPath: "/var/log/mariadb",
		},
		{
			Name:      "mariadb-run",
			MountPath: "/run/mariadb",
		},
	}
	dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		Create_env("HOME", "/var/lib/mysql"),
		Create_secret_env("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
	}
	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(MARIADB_PORT, MARIADB_PORT_NAME),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(MARIADB_PORT)
	dep.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_tcp_probe(MARIADB_PORT)
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_empty_dir("mariadb-run"),
	}

	r.GetOrCreate(&dep)
	service_ports := []int32{MARIADB_PORT}
	srv := r.create_service("mariadb", "mariadb", service_ports, MARIADB_PORT_NAME)
	r.GetOrCreate(&srv)

	return r.IsStatefulSetReady(&dep)
}
