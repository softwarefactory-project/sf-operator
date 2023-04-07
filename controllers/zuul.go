// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"

	ini "gopkg.in/ini.v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const ZUUL_WEB_PORT = 9000

const ZUUL_EXECUTOR_PORT_NAME = "finger"
const ZUUL_EXECUTOR_PORT = 7900

//go:embed static/zuul/zuul.conf
var zuul_dot_conf string

//go:embed static/zuul/init-tenant-config.sh
var zuul_init_tenant_config string

//go:embed static/zuul/generate-tenant-config.sh
var zuul_generate_tenant_config string

//go:embed static/zuul/scheduler-sidecar-entrypoint.sh
var zuul_scheduler_sidecar_entrypoint string

func is_statefulset(service string) bool {
	return service == "zuul-scheduler" || service == "zuul-executor" || service == "zuul-merger"
}

func create_zuul_container(fqdn string, service string) []apiv1.Container {
	volumes := []apiv1.VolumeMount{
		{
			Name:      "zuul-config",
			MountPath: "/etc/zuul",
			ReadOnly:  true,
		},
		{
			Name:      "zookeeper-client-tls",
			MountPath: "/tls/client",
			ReadOnly:  true,
		},
		{
			Name:      service,
			MountPath: "/var/lib/zuul",
		},
		{
			Name:      "zuul-ssh-key",
			MountPath: "/var/lib/zuul-ssh",
			ReadOnly:  true,
		},
		{
			Name:      "ca-cert",
			MountPath: "/etc/pki/ca-trust/source/anchors/ca.crt",
			ReadOnly:  true,
			SubPath:   "ca.crt",
		},
	}
	command := []string{
		"sh", "-c",
		fmt.Sprintf("exec %s -f -d", service),
	}
	container := apiv1.Container{
		Name:    service,
		Image:   "quay.io/software-factory/" + service + ":8.2.0-2",
		Command: command,
		Env: []apiv1.EnvVar{
			create_secret_env("ZUUL_DB_URI", "zuul-db-uri", "dburi"),
			create_secret_env("ZUUL_KEYSTORE_PASSWORD", "zuul-keystore-password", ""),
			create_secret_env("ZUUL_ZK_HOSTS", "zk-hosts", ""),
			create_secret_env("ZUUL_AUTH_SECRET", "zuul-auth-secret", ""),
			create_env("ZUUL_FQDN", fqdn),
			create_env("REQUESTS_CA_BUNDLE", "/etc/ssl/certs/ca-bundle.crt"),
		},
		VolumeMounts: volumes,
	}
	return []apiv1.Container{container}
}

func create_zuul_volumes(service string) []apiv1.Volume {
	var mod int32 = 256     // decimal for 0400 octal
	var execmod int32 = 493 // decimal for 0755 octal
	volumes := []apiv1.Volume{
		create_volume_secret("ca-cert"),
		create_volume_secret("zuul-config"),
		create_volume_secret("zuul-tenant-yaml"),
		create_volume_secret("zookeeper-client-tls"),
		{
			Name: "zuul-ssh-key",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "zuul-ssh-key",
					DefaultMode: &mod,
				},
			},
		},
	}
	if !is_statefulset(service) {
		// statefulset already has a PV for the service-name,
		// for the other, we use an empty dir.
		volumes = append(volumes, create_empty_dir(service))
	}
	if service == "zuul-scheduler" {
		tooling_vol := apiv1.Volume{
			Name: "tooling-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "zuul-scheduler-tooling-config-map",
					},
					DefaultMode: &execmod,
				},
			},
		}
		volumes = append(volumes, tooling_vol)
	}
	return volumes
}

func (r *SFController) create_zuul_host_alias() []apiv1.HostAlias {
	return []apiv1.HostAlias{
		{
			IP: r.get_service_ip(GERRIT_HTTPD_PORT_NAME),
			Hostnames: []string{
				"gerrit." + r.cr.Spec.FQDN,
			},
		},
	}
}

func init_scheduler_config() apiv1.Container {
	return apiv1.Container{
		Name:    "init-scheduler-config",
		Image:   BUSYBOX_IMAGE,
		Command: []string{"sh", "-c", zuul_init_tenant_config},
		VolumeMounts: []apiv1.VolumeMount{
			{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
		},
		SecurityContext: &defaultContainerSecurityContext,
	}
}

func (r *SFController) scheduler_sidecar_container() apiv1.Container {
	config_url, config_user := r.getConfigRepoCNXInfo()
	container := apiv1.Container{
		Name:    "scheduler-sidecar",
		Image:   BUSYBOX_IMAGE,
		Command: []string{"sh", "-c", zuul_scheduler_sidecar_entrypoint},
		Env: []apiv1.EnvVar{
			create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
			{
				Name:  "CONFIG_REPO_URL",
				Value: config_url,
			},
			{
				Name:  "CONFIG_REPO_USER",
				Value: config_user,
			},
			{
				Name:  "HOME",
				Value: "/var/lib/zuul",
			},
		},
		VolumeMounts: []apiv1.VolumeMount{
			{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
			{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		},
		ReadinessProbe:  create_readiness_cmd_probe([]string{"cat", "/tmp/healthy"}),
		SecurityContext: &defaultContainerSecurityContext,
	}
	return container
}

func (r *SFController) EnsureZuulServices(init_containers []apiv1.Container, config string) bool {
	annotations := map[string]string{
		"zuul-config": checksum([]byte(config)),
	}

	// NOTE: Change to "defaultPodSecurityContext", when image will use non root user.
	podSecurityContext := &apiv1.PodSecurityContext{
		RunAsUser:    pointer.Int64(10001),
		FSGroup:      pointer.Int64(10001),
		RunAsNonRoot: pointer.Bool(true),
		SeccompProfile: &apiv1.SeccompProfile{
			Type: "RuntimeDefault",
		},
	}

	fqdn := r.cr.Spec.FQDN

	scheduler_tooling_data := make(map[string]string)
	scheduler_tooling_data["generate-zuul-tenant-yaml.sh"] = zuul_generate_tenant_config
	r.EnsureConfigMap("zuul-scheduler-tooling", scheduler_tooling_data)

	zs_volumes := create_zuul_volumes("zuul-scheduler")
	zs := create_statefulset(r.ns, "zuul-scheduler", "", get_storage_classname(r.cr.Spec))
	zs.Spec.Template.ObjectMeta.Annotations = annotations
	zs.Spec.Template.Spec.InitContainers = append(init_containers, init_scheduler_config())
	zs.Spec.Template.Spec.HostAliases = r.create_zuul_host_alias()
	zs.Spec.Template.Spec.Containers = append(
		create_zuul_container(fqdn, "zuul-scheduler"), r.scheduler_sidecar_container())
	zs.Spec.Template.Spec.Volumes = zs_volumes
	zs.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9090)
	zs.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", 9090)
	zs.Spec.Template.Spec.SecurityContext = podSecurityContext
	zs.Spec.Template.Spec.Containers[0].SecurityContext = &defaultContainerSecurityContext

	r.GetOrCreate(&zs)
	zs_dirty := false
	if !map_equals(&zs.Spec.Template.ObjectMeta.Annotations, &annotations) {
		zs.Spec.Template.ObjectMeta.Annotations = annotations
		zs_dirty = true
		r.log.V(1).Info("Zuul configuration changed, restarting the services...")
	}
	if zs_dirty {
		r.UpdateR(&zs)
	}

	ze := create_headless_statefulset(r.ns, "zuul-executor", "", get_storage_classname(r.cr.Spec))
	ze.Spec.Template.ObjectMeta.Annotations = annotations
	ze.Spec.Template.Spec.HostAliases = r.create_zuul_host_alias()
	ze.Spec.Template.Spec.Containers = create_zuul_container(fqdn, "zuul-executor")
	ze.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-executor")
	ze.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9090)
	ze.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", 9090)
	ze.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(ZUUL_EXECUTOR_PORT, ZUUL_EXECUTOR_PORT_NAME),
	}
	// NOTE(dpawlik): Zuul Executor needs to privileged pod, due error in the console log:
	// "bwrap: Can't bind mount /oldroot/etc/resolv.conf on /newroot/etc/resolv.conf: Permission denied""
	ze.Spec.Template.Spec.SecurityContext = podSecurityContext
	ze.Spec.Template.Spec.Containers[0].SecurityContext = &apiv1.SecurityContext{
		Privileged: boolPtr(true),
	}

	r.GetOrCreate(&ze)
	ze_dirty := false
	if !map_equals(&ze.Spec.Template.ObjectMeta.Annotations, &annotations) {
		ze.Spec.Template.ObjectMeta.Annotations = annotations
		ze_dirty = true
	}
	if ze_dirty {
		r.UpdateR(&ze)
	}

	zw := create_deployment(r.ns, "zuul-web", "")
	zw.Spec.Template.ObjectMeta.Annotations = annotations
	zw.Spec.Template.Spec.HostAliases = r.create_zuul_host_alias()
	zw.Spec.Template.Spec.Containers = create_zuul_container(fqdn, "zuul-web")
	zw.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-web")
	zw.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", 9090)
	zw.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", 9090)

	zw.Spec.Template.Spec.SecurityContext = podSecurityContext
	zw.Spec.Template.Spec.Containers[0].SecurityContext = &defaultContainerSecurityContext

	r.GetOrCreate(&zw)
	zw_dirty := false
	if !map_equals(&zw.Spec.Template.ObjectMeta.Annotations, &annotations) {
		zw.Spec.Template.ObjectMeta.Annotations = annotations
		zw_dirty = true
	}
	if zw_dirty {
		r.UpdateR(&zw)
	}

	srv := create_service(r.ns, "zuul-web", "zuul-web", ZUUL_WEB_PORT, "zuul-web")
	r.GetOrCreate(&srv)

	srv_ze := create_headless_service(r.ns, "zuul-executor", "zuul-executor", ZUUL_EXECUTOR_PORT, "zuul-executor")
	r.GetOrCreate(&srv_ze)

	return r.IsStatefulSetReady(&zs) && r.IsStatefulSetReady(&ze) && r.IsDeploymentReady(&zw)
}

func (r *SFController) EnsureZuulSecrets(db_password *apiv1.Secret, config string) {
	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"dburi": []byte(fmt.Sprintf("mysql+pymysql://zuul:%s@mariadb/zuul", db_password.Data["zuul-db-password"])),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-db-uri", Namespace: r.ns},
	})
	r.GenerateSecretUUID("zuul-keystore-password")
	r.GenerateSecretUUID("zuul-auth-secret")
	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"zuul.conf": []byte(config),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.ns},
	})
	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"zk-hosts": []byte(`zookeeper.` + r.ns + `:2281`),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zk-hosts", Namespace: r.ns},
	})
}

func (r *SFController) read_secret_content(name string) string {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		// Loosely return empty string when not found
		return ""
	}
	return string(secret.Data[name])
}

func (r *SFController) AddGerritConnection(cfg *ini.File, conn sfv1.GerritConnection) {
	section := "connection " + conn.Name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "gerrit")
	cfg.Section(section).NewKey("server", conn.Hostname)
	cfg.Section(section).NewKey("sshkey", "/var/lib/zuul-ssh/..data/priv")
	cfg.Section(section).NewKey("user", conn.Username)
	cfg.Section(section).NewKey("gitweb_url_template", "{baseurl}/plugins/gitiles/{project.name}/+/{sha}^!/")
	// Optional fields (set as omitempty in GerritConnection struct defintion)
	if conn.Port != "" {
		cfg.Section(section).NewKey("port", conn.Port)
	}
	if conn.Puburl != "" {
		cfg.Section(section).NewKey("baseurl", conn.Puburl)
	}
	if conn.Password != "" {
		password := r.read_secret_content(conn.Password)
		cfg.Section(section).NewKey("password", password)
	}
	if conn.Canonicalhostname != "" {
		cfg.Section(section).NewKey("canonical_hostname", conn.Canonicalhostname)
	}
	if conn.VerifySSL != "" {
		cfg.Section(section).NewKey("verify_ssl", conn.VerifySSL)
	}
}

func (r *SFController) LoadConfigINI(zuul_conf string) *ini.File {
	cfg, err := ini.Load([]byte(zuul_conf))
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

func (r *SFController) DumpConfigINI(cfg *ini.File) string {
	writer := bytes.NewBufferString("")
	cfg.WriteTo(writer)
	return writer.String()
}

func (r *SFController) DeployZuul() bool {

	init_containers, db_password := r.EnsureDBInit("zuul")
	r.EnsureSSHKey("zuul-ssh-key")

	gerrit_conns := r.cr.Spec.Zuul.GerritConns

	// Add local gerrit connection if needed
	r.GenerateSecretUUID("zuul-gerrit-api-key")
	gerrit_conn := sfv1.GerritConnection{
		Name:              "gerrit",
		Hostname:          GERRIT_SSHD_PORT_NAME,
		Port:              "29418",
		Puburl:            "http://" + "gerrit." + r.cr.Spec.FQDN,
		Username:          "zuul",
		Canonicalhostname: "gerrit." + r.cr.Spec.FQDN,
		Password:          "zuul-gerrit-api-key",
	}
	gerrit_conns = append(gerrit_conns, gerrit_conn)

	// Update base config to add connections
	cfg_ini := r.LoadConfigINI(zuul_dot_conf)
	for _, conn := range gerrit_conns {
		r.AddGerritConnection(cfg_ini, conn)
	}
	// Set Zuul web public URL
	cfg_ini.Section("web").NewKey("root", "https://zuul."+r.cr.Spec.FQDN)

	config := r.DumpConfigINI(cfg_ini)

	r.EnsureZuulSecrets(&db_password, config)
	return r.EnsureZuulServices(init_containers, config)
}

func (r *SFController) runZuulTenantConfigUpdate() bool {
	err1 := r.PodExec("zuul-scheduler-0", "scheduler-sidecar", []string{"generate-zuul-tenant-yaml.sh"})
	if err1 == nil {
		err2 := r.PodExec("zuul-scheduler-0", "zuul-scheduler", []string{"zuul-scheduler", "full-reconfigure"})
		if err2 == nil {
			return true
		}

	}
	return false
}

func (r *SFController) setupZuulIngress() {
	r.ensureHTTPSRoute(r.cr.Name+"-zuul", "zuul", "zuul-web", "/", ZUUL_WEB_PORT, map[string]string{})

	// Zuul ingress is special because the zuul-web container expect the
	// the files to be served at `/zuul/`, but it is listening on `/`.
	// Thus this ingress remove the `/zuul/` so that the javascript loads as
	// expected
	r.ensureHTTPSRoute(r.cr.Name+"-zuul-red", "zuul", "zuul-web", "/zuul", ZUUL_WEB_PORT, map[string]string{
		"haproxy.router.openshift.io/rewrite-target": "/",
	})
}
