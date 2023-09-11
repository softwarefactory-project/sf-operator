// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	ini "gopkg.in/ini.v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const ZUUL_WEB_PORT = 9000

const ZUUL_EXECUTOR_PORT_NAME = "finger"
const ZUUL_EXECUTOR_PORT = 7900

const ZUUL_PROMETHEUS_PORT = 9090
const ZUUL_PROMETHEUS_PORT_NAME = "zuul-metrics"

//go:embed static/zuul/zuul.conf
var zuul_dot_conf string

//go:embed static/zuul/generate-tenant-config.sh
var zuul_generate_tenant_config string

// Common config sections for all Zuul components
var commonIniConfigSections = []string{"zookeeper", "keystore", "database"}

func Zuul_Image(service string) string {
	return "quay.io/software-factory/" + service + ":9.1.0-1"
}

func is_statefulset(service string) bool {
	return service == "zuul-scheduler" || service == "zuul-executor" || service == "zuul-merger"
}

func (r *SFController) create_zuul_container(service string) []apiv1.Container {
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
	envs := []apiv1.EnvVar{
		Create_env("REQUESTS_CA_BUNDLE", "/etc/ssl/certs/ca-bundle.crt"),
		Create_env("HOME", "/var/lib/zuul"),
	}
	if service == "zuul-scheduler" {
		volumes = append(volumes,
			apiv1.VolumeMount{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		)
		envs = append(envs, r.get_generate_tenants_envs()...)

	}
	command := []string{
		"sh", "-c",
		fmt.Sprintf("exec %s -f -d", service),
	}
	container := apiv1.Container{
		Name:         service,
		Image:        Zuul_Image(service),
		Command:      command,
		Env:          envs,
		VolumeMounts: volumes,
	}
	return []apiv1.Container{container}
}

func create_zuul_volumes(service string) []apiv1.Volume {
	var mod int32 = 256 // decimal for 0400 octal
	volumes := []apiv1.Volume{
		create_volume_secret("ca-cert"),
		create_volume_secret("zuul-config"),
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
					DefaultMode: &Execmod,
				},
			},
		}
		volumes = append(volumes, tooling_vol)
	}
	return volumes
}

func (r *SFController) get_generate_tenants_envs() []apiv1.EnvVar {
	if r.isConfigRepoSet() {
		return []apiv1.EnvVar{
			Create_env("CONFIG_REPO_SET", "TRUE"),
			Create_env("CONFIG_REPO_BASE_URL", strings.TrimSuffix(r.cr.Spec.ConfigLocation.BaseURL, "/")),
			Create_env("CONFIG_REPO_NAME", r.cr.Spec.ConfigLocation.Name),
			Create_env("CONFIG_REPO_CONNECTION_NAME", r.cr.Spec.ConfigLocation.ZuulConnectionName),
		}
	} else {
		return []apiv1.EnvVar{
			Create_env("CONFIG_REPO_SET", "FALSE"),
		}
	}
}

func (r *SFController) init_scheduler_config() apiv1.Container {
	return apiv1.Container{
		Name:    "init-scheduler-config",
		Image:   BUSYBOX_IMAGE,
		Command: []string{"/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		Env: append(r.get_generate_tenants_envs(),
			Create_env("HOME", "/var/lib/zuul"), Create_env("INIT_CONTAINER", "1")),
		VolumeMounts: []apiv1.VolumeMount{
			{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
			{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		},
		SecurityContext: create_security_context(false),
	}
}

func (r *SFController) EnsureZuulScheduler(init_containers []apiv1.Container, cfg *ini.File) bool {
	sections := IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "scheduler")

	annotations := map[string]string{
		"zuul-common-config":    IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": IniSectionsChecksum(cfg, sections),
		"zuul-image":            Zuul_Image("zuul-scheduler"),
		"serial":                "2",
	}

	if r.isConfigRepoSet() {
		annotations["config-repo-info-hash"] = r.cr.Spec.ConfigLocation.ZuulConnectionName + ":" +
			r.cr.Spec.ConfigLocation.BaseURL +
			r.cr.Spec.ConfigLocation.Name
	}

	var setAdditionalContainers = func(sts *appsv1.StatefulSet) {
		sts.Spec.Template.Spec.InitContainers = append(init_containers, r.init_scheduler_config())
		sts.Spec.Template.Spec.Containers = append(r.create_zuul_container("zuul-scheduler"))
	}

	scheduler_tooling_data := make(map[string]string)
	scheduler_tooling_data["generate-zuul-tenant-yaml.sh"] = zuul_generate_tenant_config
	r.EnsureConfigMap("zuul-scheduler-tooling", scheduler_tooling_data)

	zs_volumes := create_zuul_volumes("zuul-scheduler")
	zs_replicas := int32(1)
	zs := r.create_statefulset("zuul-scheduler", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Scheduler.Storage), zs_replicas, apiv1.ReadWriteOnce)
	zs.Spec.Template.ObjectMeta.Annotations = annotations
	setAdditionalContainers(&zs)
	zs.Spec.Template.Spec.Volumes = zs_volumes
	zs.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", ZUUL_PROMETHEUS_PORT)
	zs.Spec.Template.Spec.Containers[0].LivenessProbe = create_liveness_http_probe("/health/live", ZUUL_PROMETHEUS_PORT)
	zs.Spec.Template.Spec.Containers[0].StartupProbe = create_startup_http_probe("/health/ready", ZUUL_PROMETHEUS_PORT)
	zs.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(ZUUL_PROMETHEUS_PORT, ZUUL_PROMETHEUS_PORT_NAME),
	}

	current := appsv1.StatefulSet{}
	if r.GetM("zuul-scheduler", &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-scheduler configuration changed, rollout zuul-scheduler pods ...")
			current.Spec = zs.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zs
		r.CreateR(&current)
	}

	isStatefulSet := r.IsStatefulSetReady(&current)
	if isStatefulSet {
		refresh_condition(&r.cr.Status.Conditions, "zuul-scheduler", metav1.ConditionTrue, "Complete", "Initialization of zuul-scheduler service completed.")
	} else {
		refresh_condition(&r.cr.Status.Conditions, "zuul-scheduler", metav1.ConditionUnknown, "Awaiting", "Initializing zuul-scheduler service...")
	}

	return isStatefulSet
}

func (r *SFController) EnsureZuulExecutor(cfg *ini.File) bool {
	sections := IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "executor")
	annotations := map[string]string{
		"zuul-common-config":    IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": IniSectionsChecksum(cfg, sections),
		"zuul-image":            Zuul_Image("zuul-executor"),
		"replicas":              strconv.Itoa(int(r.cr.Spec.Zuul.Executor.Replicas)),
	}

	ze := r.create_headless_statefulset("zuul-executor", "", r.getStorageConfOrDefault(r.cr.Spec.Zuul.Scheduler.Storage), int32(r.cr.Spec.Zuul.Executor.Replicas), apiv1.ReadWriteOnce)
	ze.Spec.Template.ObjectMeta.Annotations = annotations
	ze.Spec.Template.Spec.Containers = r.create_zuul_container("zuul-executor")
	ze.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-executor")
	ze.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/health/ready", ZUUL_PROMETHEUS_PORT)
	ze.Spec.Template.Spec.Containers[0].LivenessProbe = create_readiness_http_probe("/health/live", ZUUL_PROMETHEUS_PORT)
	ze.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(ZUUL_PROMETHEUS_PORT, ZUUL_PROMETHEUS_PORT_NAME),
		Create_container_port(ZUUL_EXECUTOR_PORT, ZUUL_EXECUTOR_PORT_NAME),
	}
	// NOTE(dpawlik): Zuul Executor needs to privileged pod, due error in the console log:
	// "bwrap: Can't bind mount /oldroot/etc/resolv.conf on /newroot/etc/resolv.conf: Permission denied""
	ze.Spec.Template.Spec.Containers[0].SecurityContext = create_security_context(true)
	ze.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = pointer.Int64(1000)

	current := appsv1.StatefulSet{}
	if r.GetM("zuul-executor", &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-executor configuration changed, rollout zuul-executor pods ...")
			current.Spec = ze.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := ze
		r.CreateR(&current)
	}

	isStatefulSet := r.IsStatefulSetReady(&current)
	if isStatefulSet {
		refresh_condition(&r.cr.Status.Conditions, "zuul-executor", metav1.ConditionTrue, "Complete", "Initialization of zuul-executor service completed.")
	} else {
		refresh_condition(&r.cr.Status.Conditions, "zuul-executor", metav1.ConditionUnknown, "Awaiting", "Initializing zuul-executor service...")
	}

	return isStatefulSet
}

func (r *SFController) EnsureZuulWeb(cfg *ini.File) bool {
	sections := IniGetSectionNamesByPrefix(cfg, "connection")
	sections = append(sections, "scheduler")
	annotations := map[string]string{
		"zuul-common-config":    IniSectionsChecksum(cfg, commonIniConfigSections),
		"zuul-component-config": IniSectionsChecksum(cfg, sections),
		"zuul-image":            Zuul_Image("zuul-web"),
	}

	zw := r.create_deployment("zuul-web", "")
	zw.Spec.Template.ObjectMeta.Annotations = annotations
	zw.Spec.Template.Spec.Containers = r.create_zuul_container("zuul-web")
	zw.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-web")
	zw.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/api/info", ZUUL_WEB_PORT)
	zw.Spec.Template.Spec.Containers[0].LivenessProbe = create_liveness_http_probe("/api/info", ZUUL_WEB_PORT)
	zw.Spec.Template.Spec.Containers[0].StartupProbe = create_startup_http_probe("/api/info", ZUUL_WEB_PORT)
	zw.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		Create_container_port(ZUUL_PROMETHEUS_PORT, ZUUL_PROMETHEUS_PORT_NAME),
	}

	current := appsv1.Deployment{}
	if r.GetM("zuul-web", &current) {
		if !map_equals(&current.Spec.Template.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("zuul-web configuration changed, rollout zuul-web pods ...")
			current.Spec = zw.DeepCopy().Spec
			r.UpdateR(&current)
			return false
		}
	} else {
		current := zw
		r.CreateR(&current)
	}

	isDeploymentReady := r.IsDeploymentReady(&current)
	if isDeploymentReady {
		refresh_condition(&r.cr.Status.Conditions, "zuul-web", metav1.ConditionTrue, "Complete", "Initialization of zuul-web service completed.")
	} else {
		refresh_condition(&r.cr.Status.Conditions, "zuul-web", metav1.ConditionUnknown, "Awaiting", "Initializing zuul-web service...")
	}

	return isDeploymentReady
}

func (r *SFController) EnsureZuulComponentsFrontServices() {
	service_ports := []int32{ZUUL_WEB_PORT}
	srv := r.create_service("zuul-web", "zuul-web", service_ports, "zuul-web")
	r.GetOrCreate(&srv)

	headless_ports := []int32{ZUUL_EXECUTOR_PORT}
	srv_ze := r.create_headless_service("zuul-executor", "zuul-executor", headless_ports, "zuul-executor")
	r.GetOrCreate(&srv_ze)

}

func (r *SFController) EnsureZuulComponents(init_containers []apiv1.Container, cfg *ini.File) bool {

	zuul_services := map[string]bool{}
	zuul_services["scheduler"] = r.EnsureZuulScheduler(init_containers, cfg)
	zuul_services["executor"] = r.EnsureZuulExecutor(cfg)
	zuul_services["web"] = r.EnsureZuulWeb(cfg)

	return zuul_services["scheduler"] && zuul_services["executor"] && zuul_services["web"]
}

func (r *SFController) EnsureZuulConfigSecret(cfg *ini.File) {
	r.EnsureSecret(&apiv1.Secret{
		Data: map[string][]byte{
			"zuul.conf": []byte(DumpConfigINI(cfg)),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.ns},
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
	cfg.Section(section).NewKey("gitweb_url_template", "{baseurl}/plugins/gitiles/{project.name}/+/{sha}^!/")
	// Optional fields (set as omitempty in GerritConnection struct definition)
	cfg.Section(section).NewKey("user", conn.Username)
	cfg.Section(section).NewKey("port", strconv.Itoa(int(conn.Port)))
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
	cfg.Section(section).NewKey("verify_ssl", strconv.FormatBool(conn.VerifySSL))
	cfg.Section(section).NewKey("git_over_ssh", strconv.FormatBool(conn.GitOverSSH))
}

func AddGitConnection(cfg *ini.File, name string, baseurl string) {
	section := "connection " + name
	cfg.NewSection(section)
	cfg.Section(section).NewKey("driver", "git")
	cfg.Section(section).NewKey("baseurl", baseurl)
}

func AddWebClientSection(cfg *ini.File) {
	section := "webclient"
	cfg.NewSection(section)
	cfg.Section(section).NewKey("url", "http://zuul-web:"+strconv.FormatInt(ZUUL_WEB_PORT, 10))
}

func (r *SFController) AddDefaultConnections(cfg *ini.File) {
	// Internal git-server for system config
	AddGitConnection(cfg, "git-server", "git://git-server/")

	// Git connection to opendev.org
	AddGitConnection(cfg, "opendev.org", "https://opendev.org/")

	// Add Web Client for zuul-client
	AddWebClientSection(cfg)
}

func LoadConfigINI(zuul_conf string) *ini.File {
	cfg, err := ini.Load([]byte(zuul_conf))
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

func DumpConfigINI(cfg *ini.File) string {
	writer := bytes.NewBufferString("")
	cfg.WriteTo(writer)
	return writer.String()
}

func (r *SFController) DeployZuulSecrets() {
	r.EnsureSSHKey("zuul-ssh-key")
	r.GenerateSecretUUID("zuul-keystore-password")
	r.GenerateSecretUUID("zuul-auth-secret")
}

func (r *SFController) DeployZuul() bool {
	init_containers := []apiv1.Container{}
	db_password := apiv1.Secret{}
	if !r.GetM(ZUUL_DB_CONFIG_SECRET, &db_password) {
		r.log.Info("Waiting for db connection secret")
		return false
	}

	// Update base config to add connections
	cfg_ini := LoadConfigINI(zuul_dot_conf)
	for _, conn := range r.cr.Spec.Zuul.GerritConns {
		r.AddGerritConnection(cfg_ini, conn)
	}
	// Add default connections
	r.AddDefaultConnections(cfg_ini)

	// Enable prometheus metrics
	for _, srv := range []string{"web", "executor", "scheduler"} {
		cfg_ini.Section(srv).NewKey("prometheus_port", strconv.Itoa(ZUUL_PROMETHEUS_PORT))
	}
	// Set Zuul web public URL
	cfg_ini.Section("web").NewKey("root", "https://zuul."+r.cr.Spec.FQDN)

	// Set Database DB URI
	cfg_ini.Section("database").NewKey("dburi", fmt.Sprintf("mysql+pymysql://zuul:%s@mariadb/zuul", db_password.Data["password"]))

	// Set Zookeeper hosts
	cfg_ini.Section("zookeeper").NewKey("hosts", "zookeeper."+r.ns+":2281")

	// Set Keystore secret
	keystore_pass, err := r.getSecretData("zuul-keystore-password")
	if err != nil {
		r.log.Info("Waiting for zuul-keystore-password secret")
		return false
	}
	cfg_ini.Section("keystore").NewKey("password", string(keystore_pass))

	r.EnsureZuulConfigSecret(cfg_ini)
	r.EnsureZuulComponentsFrontServices()
	return r.EnsureZuulComponents(init_containers, cfg_ini) && r.setupZuulIngress()
}

func (r *SFController) runZuulFullReconfigure() bool {
	err := r.PodExec("zuul-scheduler-0", "zuul-scheduler", []string{"zuul-scheduler", "full-reconfigure"})
	return err == nil
}

func (r *SFController) setupZuulIngress() bool {
	route1_ready := r.ensureHTTPSRoute(r.cr.Name+"-zuul", "zuul", "zuul-web", "/", ZUUL_WEB_PORT,
		map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	// Zuul ingress is special because the zuul-web container expect the
	// the files to be served at `/zuul/`, but it is listening on `/`.
	// Thus this ingress remove the `/zuul/` so that the javascript loads as
	// expected
	route2_ready := r.ensureHTTPSRoute(r.cr.Name+"-zuul-red", "zuul", "zuul-web", "/zuul", ZUUL_WEB_PORT, map[string]string{
		"haproxy.router.openshift.io/rewrite-target": "/",
	}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)
	return route1_ready && route2_ready
}
