// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the mariadb configuration.

package controllers

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	logging "github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MariaDBIdent       = "mariadb"
	mariadbPort        = 3306
	mariaDBPortName    = "mariadb-port"
	zuulDBConfigSecret = "zuul-db-connection"
)

//go:embed static/mariadb/fluentbit/fluent-bit.conf.tmpl
var mariadbFluentBitForwarderConfig string

type ZuulDBOpts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
	Params   map[string]string
}

func createLogForwarderSidecar(r *SFController, annotations map[string]string) (apiv1.Volume, apiv1.Container) {

	fbForwarderConfig := make(map[string]string)
	fbForwarderConfig["fluent-bit.conf"], _ = utils.ParseString(
		mariadbFluentBitForwarderConfig,
		struct {
			ExtraKeys              []logging.FluentBitLabel
			FluentBitHTTPInputHost string
			FluentBitHTTPInputPort string
		}{[]logging.FluentBitLabel{}, r.cr.Spec.FluentBitLogForwarding.HTTPInputHost, strconv.Itoa(int(r.cr.Spec.FluentBitLogForwarding.HTTPInputPort))})
	r.EnsureConfigMap("fluentbit-mariadb-cfg", fbForwarderConfig)

	volume := base.MkVolumeCM("mariadb-log-forwarder-config",
		"fluentbit-mariadb-cfg-config-map")

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      "mariadb-logs",
			MountPath: "/watch/",
		},
		{
			Name:      "mariadb-log-forwarder-config",
			MountPath: "/fluent-bit/etc/",
		},
	}
	sidecar := logging.CreateFluentBitSideCarContainer("mariadb", []logging.FluentBitLabel{}, volumeMounts)
	annotations["mariadb-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	annotations["mariadb-fluent-bit-image"] = base.FluentBitImage
	return volume, sidecar
}

func (r *SFController) CreateDBInitContainer(username string, password string, dbname string) apiv1.Container {
	c := "CREATE DATABASE IF NOT EXISTS " + dbname + " CHARACTER SET utf8 COLLATE utf8_general_ci; "
	g := "GRANT ALL PRIVILEGES ON " + dbname + ".* TO '" + username + "'@'%' IDENTIFIED BY '${USER_PASSWORD}' WITH GRANT OPTION; FLUSH PRIVILEGES;"
	container := base.MkContainer("mariadb-client", base.MariabDBImage)
	container.Command = []string{"sh", "-c", `
	echo 'Running: mysql --host=" ` + MariaDBIdent + `" --user=root --password="$MYSQL_ROOT_PASSWORD" -e "` + c + g + `"'
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
		Host:     MariaDBIdent,
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

	sts := r.mkStatefulSet(MariaDBIdent, base.MariabDBImage, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.DBStorage), apiv1.ReadWriteOnce)

	sts.Spec.VolumeClaimTemplates = append(
		sts.Spec.VolumeClaimTemplates,
		// TODO redirect logs to stdout so we don't need a volume
		base.MkPVC("mariadb-logs", r.ns, r.getStorageConfOrDefault(r.cr.Spec.MariaDB.LogStorage), apiv1.ReadWriteOnce))

	volumeMountsStatsExporter := []apiv1.VolumeMount{
		{
			Name:      MariaDBIdent,
			MountPath: "/var/lib/mysql",
		},
		{
			Name:      "mariadb-logs",
			MountPath: "/var/log/mariadb",
		},
	}

	sts.Spec.Template.Spec.Containers[0].VolumeMounts = append([]apiv1.VolumeMount{
		{
			Name:      "mariadb-run",
			MountPath: "/run/mariadb",
		},
	}, volumeMountsStatsExporter...)
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

	annotations := map[string]string{
		"serial": "3",
	}
	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolume, fbSidecar := createLogForwarderSidecar(r, annotations)
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, fbSidecar)
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, fbVolume)
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(MariaDBIdent, volumeMountsStatsExporter)
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, statsExporter)

	sts.Spec.Template.ObjectMeta.Annotations = annotations

	current, changed := r.ensureStatefulset(sts)
	if changed {
		return false
	}

	servicePorts := []int32{mariadbPort}
	srv := base.MkServicePod(MariaDBIdent, r.ns, "mariadb-0", servicePorts, mariaDBPortName)
	r.EnsureService(&srv)

	var zuulDBSecret apiv1.Secret

	stsReady := r.IsStatefulSetReady(current)

	if stsReady {
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

	isReady := stsReady && zuulDBSecret.Data != nil

	conds.UpdateConditions(&r.cr.Status.Conditions, MariaDBIdent, isReady)

	return isReady
}
