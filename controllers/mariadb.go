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
	MariadbAdminPass   = "mariadb-root-password"
)

//go:embed static/mariadb/fluentbit/fluent-bit.conf.tmpl
var mariadbFluentBitForwarderConfig string

//go:embed static/mariadb/my.cnf.tmpl
var mariadbMyCNF string

type ZuulDBOpts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
	Params   map[string]string
}

func createLogForwarderSidecar(r *SFController, annotations map[string]string) ([]apiv1.Volume, apiv1.Container) {

	fbForwarderConfig := make(map[string]string)
	var loggingParams = logging.CreateForwarderConfigTemplateParams("mariadb", r.cr.Spec.FluentBitLogForwarding)

	fbForwarderConfig["fluent-bit.conf"], _ = utils.ParseString(
		mariadbFluentBitForwarderConfig,
		struct {
			ExtraKeys     []logging.FluentBitLabel
			LoggingParams logging.TemplateLoggingParams
		}{[]logging.FluentBitLabel{}, loggingParams})
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
	var fluentbitDebug = false
	if r.cr.Spec.FluentBitLogForwarding.Debug != nil {
		fluentbitDebug = *r.cr.Spec.FluentBitLogForwarding.Debug
	}
	sidecar, storageEmptyDir := logging.CreateFluentBitSideCarContainer(MariaDBIdent, []logging.FluentBitLabel{}, volumeMounts, fluentbitDebug)
	annotations["mariadb-fluent-bit.conf"] = utils.Checksum([]byte(fbForwarderConfig["fluent-bit.conf"]))
	annotations["mariadb-fluent-bit-image"] = sidecar.Image
	return []apiv1.Volume{volume, storageEmptyDir}, sidecar
}

func (r *SFController) CreateDBInitContainer(username string, password string, dbname string) apiv1.Container {
	c := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8 COLLATE utf8_general_ci;", dbname)
	g := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%' IDENTIFIED BY '${USER_PASSWORD}' WITH GRANT OPTION; FLUSH PRIVILEGES;", dbname, username)
	container := base.MkContainer("mariadb-client", base.MariaDBImage())
	base.SetContainerLimitsLowProfile(&container)
	container.Command = []string{"sh", "-c", `
	echo 'Running: mysql --host="` + MariaDBIdent + `" --user=root --password="$MARIADB_ROOT_PASSWORD" -e "` + c + g + `"'
	ATTEMPT=0
	while ! mysql --host=mariadb --user=root --password="$MARIADB_ROOT_PASSWORD" -e "` + c + g + `"; do
		ATTEMPT=$[ $ATTEMPT + 1 ]
		if test $ATTEMPT -eq 10; then
			echo "Failed after $ATTEMPT attempt";
			exit 1
		fi
		sleep 10;
	done
	`}
	container.Env = []apiv1.EnvVar{
		base.MkSecretEnvVar("MARIADB_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
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
	adminPassSecret := r.EnsureSecretUUID(MariadbAdminPass)

	myCNF, _ := utils.ParseString(mariadbMyCNF,
		struct {
			MYSQLRootPassword string
		}{MYSQLRootPassword: string(adminPassSecret.Data["mariadb-root-password"])})

	initfileSQL := fmt.Sprintf(
		`CREATE USER IF NOT EXISTS root@localhost IDENTIFIED BY '%s';
SET PASSWORD FOR root@localhost = PASSWORD('%s');
GRANT ALL ON *.* TO root@localhost WITH GRANT OPTION;
CREATE USER IF NOT EXISTS root@'%%' IDENTIFIED BY '%s';
SET PASSWORD FOR root@'%%' = PASSWORD('%s');
GRANT ALL ON *.* TO root@'%%' WITH GRANT OPTION;`,
		adminPassSecret.Data["mariadb-root-password"],
		adminPassSecret.Data["mariadb-root-password"],
		adminPassSecret.Data["mariadb-root-password"],
		adminPassSecret.Data["mariadb-root-password"])

	configSecret := apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-config-secrets",
			Namespace: r.ns,
		},
		Data: map[string][]byte{
			"my.cnf": []byte(myCNF),
		},
	}
	initDBSecret := apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-initdb-secrets",
			Namespace: r.ns,
		},
		Data: map[string][]byte{
			"initfile.sql": []byte(initfileSQL),
		},
	}
	r.EnsureSecret(&configSecret)
	r.EnsureSecret(&initDBSecret)

	sts := r.mkStatefulSet(MariaDBIdent, base.MariaDBImage(), r.getStorageConfOrDefault(r.cr.Spec.MariaDB.DBStorage), apiv1.ReadWriteOnce, r.cr.Spec.ExtraLabels)

	base.SetContainerLimitsHighProfile(&sts.Spec.Template.Spec.Containers[0])
	limitstr := base.UpdateContainerLimit(r.cr.Spec.MariaDB.Limits, &sts.Spec.Template.Spec.Containers[0])

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
		{
			Name:      "mariadb-config-secrets",
			SubPath:   "my.cnf",
			MountPath: "/var/lib/mysql/.my.cnf",
			ReadOnly:  true,
		},
		{
			Name:      "mariadb-initdb-secrets",
			SubPath:   "initfile.sql",
			MountPath: "/docker-entrypoint-initdb.d/initfile.sql",
			ReadOnly:  true,
		},
	}, volumeMountsStatsExporter...)
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/var/lib/mysql"),
		base.MkSecretEnvVar("MARIADB_ROOT_PASSWORD", "mariadb-root-password", "mariadb-root-password"),
		base.MkEnvVar("MARIADB_DISABLE_UPGRADE_BACKUP", "1"),
		base.MkEnvVar("MARIADB_AUTO_UPGRADE", "1"),
	}
	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(mariadbPort, mariaDBPortName),
	}

	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessCMDProbe([]string{"mysql", "-h", "127.0.0.1", "-e", "SELECT 1"})
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLivenessCMDProbe([]string{"mysqladmin", "ping"})
	sts.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkEmptyDirVolume("mariadb-run"),
		base.MkVolumeSecret("mariadb-config-secrets", "mariadb-config-secrets"),
		base.MkVolumeSecret("mariadb-initdb-secrets", "mariadb-initdb-secrets"),
	}

	annotations := map[string]string{
		"serial": "6",
		"image":  base.MariaDBImage(),
		"limits": limitstr,
	}
	if r.cr.Spec.FluentBitLogForwarding != nil {
		fbVolumes, fbSidecar := createLogForwarderSidecar(r, annotations)
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, fbSidecar)
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, fbVolumes...)
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(MariaDBIdent, volumeMountsStatsExporter)
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, statsExporter)

	sts.Spec.Template.ObjectMeta.Annotations = annotations

	current, changed := r.ensureStatefulset(sts)
	if changed {
		return false
	}

	servicePorts := []int32{mariadbPort}
	srv := base.MkServicePod(MariaDBIdent, r.ns, "mariadb-0", servicePorts, mariaDBPortName, r.cr.Spec.ExtraLabels)
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
			utils.LogI("Starting DB Post Init")
			zuulDBSecret = r.DBPostInit(zuulDBSecret)
		}
	}

	pvcDataReadiness := r.reconcileExpandPVC(MariaDBIdent+"-"+MariaDBIdent+"-0", r.cr.Spec.MariaDB.DBStorage)
	pvcLogsReadiness := r.reconcileExpandPVC(MariaDBIdent+"-logs-"+MariaDBIdent+"-0", r.cr.Spec.MariaDB.LogStorage)

	isReady := stsReady && pvcDataReadiness && pvcLogsReadiness && zuulDBSecret.Data != nil

	conds.UpdateConditions(&r.cr.Status.Conditions, MariaDBIdent, isReady)

	return isReady
}
