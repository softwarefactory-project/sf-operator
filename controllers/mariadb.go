// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the mariadb configuration.

package controllers

import (
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DBImage = "quay.io/software-factory/mariadb:10.5.16-4"

const MARIADB_PORT = 3306
const MARIADB_PORT_NAME = "mariadb-port"

const ZUUL_DB_CONFIG_SECRET = "zuul-db-connection"

type ZuulDBOpts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
	Params   map[string]string
}

func (r *SFController) CreateDBInitContainer(username string, password string, dbname string) apiv1.Container {
	c := "CREATE DATABASE IF NOT EXISTS " + dbname + " CHARACTER SET utf8 COLLATE utf8_general_ci; "
	g := "GRANT ALL PRIVILEGES ON " + dbname + ".* TO '" + username + "'@'%' IDENTIFIED BY '${USER_PASSWORD}' WITH GRANT OPTION; FLUSH PRIVILEGES;"
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
			{
				Name:  "USER_PASSWORD",
				Value: password,
			},
		},
	}
	return container
}

func (r *SFController) CreateProvisionDBJob(database string, password string) batchv1.Job {
	var ttl int32 = 600
	var backoffLimit int32 = 5
	dbInitJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database + "-db-provision",
			Namespace: r.ns,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoffLimit,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					RestartPolicy: apiv1.RestartPolicyOnFailure,
					Containers: []apiv1.Container{
						r.CreateDBInitContainer(database, password, database),
					},
				},
			},
		},
	}
	r.GetOrCreate(dbInitJob)
	return *dbInitJob
}

func (r *SFController) DBPostInit(zuul_db_config_secret apiv1.Secret) apiv1.Secret {
	zuulOpts := ZuulDBOpts{
		Username: "zuul",
		Password: NewUUIDString(),
		Host:     "mariadb",
		Port:     MARIADB_PORT,
		Database: "zuul",
		Params:   map[string]string{},
	}

	config := mysql.NewConfig()
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%d", zuulOpts.Host, zuulOpts.Port)
	config.User = zuulOpts.Username
	config.Passwd = zuulOpts.Password
	config.DBName = zuulOpts.Database
	dsn := config.FormatDSN()

	zuulSecretData := map[string][]byte{
		"username": []byte(zuulOpts.Username),
		"password": []byte(zuulOpts.Password),
		"host":     []byte(zuulOpts.Host),
		"port":     []byte(strconv.Itoa(int(zuulOpts.Port))),
		"database": []byte(zuulOpts.Database),
		"dsn":      []byte(dsn),
	}

	dbInitJob := r.CreateProvisionDBJob(zuulOpts.Database, zuulOpts.Password)
	if *dbInitJob.Spec.Completions > 0 {
		zuul_db_config_secret.Data = zuulSecretData
		r.GetOrCreate(&zuul_db_config_secret)
	}

	return zuul_db_config_secret
}

func (r *SFController) DeployMariadb() bool {
	pass_name := "mariadb-root-password"
	r.GenerateSecretUUID(pass_name)

	replicas := int32(1)
	dep := r.create_statefulset("mariadb", DBImage, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.DBStorage), replicas, apiv1.ReadWriteOnce)

	dep.Spec.VolumeClaimTemplates = append(
		dep.Spec.VolumeClaimTemplates,
		// TODO redirect logs to stdout so we don't need a volume
		r.create_pvc("mariadb-logs", r.getStorageConfOrDefault(r.cr.Spec.MariaDB.LogStorage), apiv1.ReadWriteOnce))
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

	var zuul_db_config_secret apiv1.Secret

	if r.IsStatefulSetReady(&dep) {
		zuul_db_config_secret = apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ZUUL_DB_CONFIG_SECRET,
				Namespace: r.ns,
			},
			Data: nil,
		}
		if !r.GetM(ZUUL_DB_CONFIG_SECRET, &zuul_db_config_secret) {
			r.log.V(1).Info("Starting DB Post Init")
			zuul_db_config_secret = r.DBPostInit(zuul_db_config_secret)
		}
	}

	isStatefulSet := r.IsStatefulSetReady(&dep)
	updateConditions(&r.cr.Status.Conditions, "mariadb", isStatefulSet)

	return isStatefulSet && zuul_db_config_secret.Data != nil
}
