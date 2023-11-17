// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the mariadb configuration.

package controllers

import (
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mariadbIdent       = "mariadb"
	mariadbPort        = 3306
	mariaDBPortName    = "mariadb-port"
	zuulDBConfigSecret = "zuul-db-connection"
)

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
	container := base.MkContainer("mariadb-client", base.MariabDBImage)
	container.Command = []string{"sh", "-c", `
	echo 'Running: mysql --host=" ` + mariadbIdent + `" --user=root --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"'
	ATTEMPT=0
	while ! mysql --host=mariadb --user=root --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"; do
		ATTEMPT=$[ $ATTEMPT + 1 ]
		if test $ATTEMPT -eq 10; then
			echo "Failed after $ATTEMPT attempt";
			exit 1
		fi
		sleep 10;
	done
	`}
	container.Env = []apiv1.EnvVar{
		base.MkSecretEnvVar("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
		{
			Name:  "USER_PASSWORD",
			Value: password,
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

func (r *SFController) DBPostInit(configSecret apiv1.Secret) apiv1.Secret {
	zuulOpts := ZuulDBOpts{
		Username: "zuul",
		Password: utils.NewUUIDString(),
		Host:     mariadbIdent,
		Port:     mariadbPort,
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
		configSecret.Data = zuulSecretData
		r.GetOrCreate(&configSecret)
	}

	return configSecret
}

func (r *SFController) DeployMariadb() bool {
	passName := "mariadb-root-password"
	r.EnsureSecretUUID(passName)

	sts := r.mkStatefulSet(mariadbIdent, base.MariabDBImage, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.DBStorage), apiv1.ReadWriteOnce)

	sts.Spec.VolumeClaimTemplates = append(
		sts.Spec.VolumeClaimTemplates,
		// TODO redirect logs to stdout so we don't need a volume
		base.MkPVC("mariadb-logs", r.ns, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.LogStorage), apiv1.ReadWriteOnce))
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      mariadbIdent,
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
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/var/lib/mysql"),
		base.MkSecretEnvVar("MYSQL_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
	}
	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(mariadbPort, mariaDBPortName),
	}

	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessTCPProbe(mariadbPort)
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkReadinessTCPProbe(mariadbPort)
	sts.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkEmptyDirVolume("mariadb-run"),
	}

	r.GetOrCreate(&sts)

	servicePorts := []int32{mariadbPort}
	srv := base.MkServicePod("mariadb", r.ns, "mariadb-0", servicePorts, mariaDBPortName)
	r.GetOrCreate(&srv)

	var zuulDBSecret apiv1.Secret

	if r.IsStatefulSetReady(&sts) {
		zuulDBSecret = apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      zuulDBConfigSecret,
				Namespace: r.ns,
			},
			Data: nil,
		}
		if !r.GetM(zuulDBConfigSecret, &zuulDBSecret) {
			r.log.V(1).Info("Starting DB Post Init")
			zuulDBSecret = r.DBPostInit(zuulDBSecret)
		}
	}

	isReady := r.IsStatefulSetReady(&sts) && zuulDBSecret.Data != nil

	conds.UpdateConditions(&r.cr.Status.Conditions, mariadbIdent, isReady)

	return isReady
}
