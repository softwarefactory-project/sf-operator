// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the mariadb configuration.

package controllers

import (
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
)

// The official image seems to consume all the available memory, and it gets OOMed.
// const DBImage = "quay.io/software-factory/mariadb:10.3.10-1"
const DBImage = "docker.io/linuxserver/mariadb"

const MYSQL_PORT = 3306
const MYSQL_PORT_NAME = "mariadb-port"

func (r *SFController) EnsureDB(name string) (apiv1.Secret, bool) {
	var job batchv1.Job
	job_name := "create-db" + name
	found := r.GetM(job_name, &job)

	db_password := r.EnsureSecret(name + "-db-password")

	if !found {
		c := "CREATE DATABASE IF NOT EXISTS " + name + " CHARACTER SET utf8 COLLATE utf8_general_ci; "
		g := "GRANT ALL PRIVILEGES ON " + name + ".* TO '" + name + "'@'%' IDENTIFIED BY '${USER_PASSWORD}' WITH GRANT OPTION; FLUSH PRIVILEGES;"
		container := apiv1.Container{
			Name:  "mariadb-client",
			Image: DBImage,
			Command: []string{"sh", "-c", `
echo 'Running: mysql --host=mariadb --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"'
ATTEMPT=0
while ! mysql --host=mariadb --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"; do
    ATTEMPT=$[ $ATTEMPT + 1 ]
    if test $ATTEMPT -eq 10; then
        echo "Failed after $ATTEMPT attempt";
        exit 1
    fi
    sleep 10;
done
`},
			Env: []apiv1.EnvVar{
				create_secret_env("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
				create_secret_env("USER_PASSWORD", name+"-db-password", name+"-db-password"),
			},
		}
		// https://pkg.go.dev/k8s.io/api/batch/v1#Job
		job := create_job(r.ns, job_name, container)

		r.log.V(1).Info("Creating job to ensure db", "name", name)
		r.CreateR(&job)
		return db_password, false
	}
	if (job.Status.Succeeded < 1) {
		r.log.V(1).Info("Waiting for ensure db job", "name", name, "status", job.Status)
	}
	return db_password, job.Status.Succeeded >= 1
}

func (r *SFController) DeployMariadb(enabled bool) bool {
	if enabled {
		pass_name := "mariadb-root-password"
		r.EnsureSecret(pass_name)
		dep := create_statefulset(r.ns, "mariadb", DBImage)
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "mariadb",
				MountPath: "/var/lib/mysql",
			},
		}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
		}
		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(MYSQL_PORT, MYSQL_PORT_NAME),
		}
		// TODO: add ready probe
		r.Apply(&dep)
		srv := create_service(r.ns, "mariadb", "mariadb", MYSQL_PORT, MYSQL_PORT_NAME)
		r.Apply(&srv)

		return r.IsStatefulSetReady("mariadb")
	} else {
		r.DeleteStatefulSet("mariadb")
		r.DeleteService("mariadb")
		return true
	}
}
