// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/zuul/zuul.conf
var zuul_dot_conf string

//go:embed static/zuul/init_tenant_config.sh
var zuul_init_tenant_config string

func is_statefulset(service string) bool {
	return service == "zuul-scheduler" || service == "zuul-executor" || service == "zuul-merger"
}

func create_zuul_container(service string) []apiv1.Container {
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
	}
	container := apiv1.Container{
		Name:    service,
		Image:   "quay.io/software-factory/" + service + "-ubi:5.2.2-3",
		Command: []string{service, "-f", "-d"},
		Env: []apiv1.EnvVar{
			create_secret_env("ZUUL_DB_URI", "zuul-db-uri", "dburi"),
			create_secret_env("ZUUL_KEYSTORE_PASSWORD", "zuul-keystore-password", ""),
			create_secret_env("ZUUL_ZK_HOSTS", "zk-hosts", ""),
		},
		VolumeMounts: volumes,
	}
	return []apiv1.Container{container}
}

func create_zuul_volumes(service string) []apiv1.Volume {
	volumes := []apiv1.Volume{
		create_volume_secret("zuul-config"),
		create_volume_secret("zuul-tenant-yaml"),
		create_volume_secret("zookeeper-client-tls"),
	}
	if !is_statefulset(service) {
		// statefulset already has a PV for the service-name,
		// for the other, we use an empty dir.
		volumes = append(volumes, create_empty_dir(service))
	}
	return volumes
}

func init_scheduler_config() apiv1.Container {
	return apiv1.Container{
		Name:    "init-scheduler-config",
		Image:   POST_INIT_IMAGE,
		Command: []string{"sh", "-c", zuul_init_tenant_config},
		VolumeMounts: []apiv1.VolumeMount{
			{Name: "zuul-scheduler", MountPath: "/var/lib/zuul"},
		},
	}
}

func (r *SFController) EnsureZuulServices(init_containers []apiv1.Container, config string) bool {
	annotations := map[string]string{
		"zuul-config": checksum([]byte(config)),
	}
	zs := create_statefulset(r.ns, "zuul-scheduler", "")
	zs.Spec.Template.ObjectMeta.Annotations = annotations
	zs.Spec.Template.Spec.InitContainers = append(init_containers, init_scheduler_config())
	zs.Spec.Template.Spec.Containers = create_zuul_container("zuul-scheduler")
	zs.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-scheduler")
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

	ze := create_statefulset(r.ns, "zuul-executor", "")
	ze.Spec.Template.ObjectMeta.Annotations = annotations
	ze.Spec.Template.Spec.Containers = create_zuul_container("zuul-executor")
	ze.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-executor")
	r.GetOrCreate(&ze)
	ze_dirty := false
	if !map_equals(&ze.Spec.Template.ObjectMeta.Annotations, &annotations) {
		ze.Spec.Template.ObjectMeta.Annotations = annotations
		ze_dirty = true
	}
	if !zs_dirty && ze_dirty {
		r.UpdateR(&ze)
	}

	zw := create_deployment(r.ns, "zuul-web", "")
	zw.Spec.Template.ObjectMeta.Annotations = annotations
	zw.Spec.Template.Spec.Containers = create_zuul_container("zuul-web")
	zw.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-web")
	r.GetOrCreate(&zw)
	zw_dirty := false
	if !map_equals(&zw.Spec.Template.ObjectMeta.Annotations, &annotations) {
		zw.Spec.Template.ObjectMeta.Annotations = annotations
		zw_dirty = true
	}
	if !zs_dirty && zw_dirty {
		r.UpdateR(&zw)
	}

	srv := create_service(r.ns, "zuul-web", "zuul-web", 9000, "zuul-web")
	r.GetOrCreate(&srv)

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

func (r *SFController) DeployZuul(enabled bool) bool {
	if enabled {
		init_containers, db_password := r.EnsureDBInit("zuul")
		config := zuul_dot_conf
		// TODO: add user defined connections
		r.EnsureZuulSecrets(&db_password, config)
		return r.EnsureZuulServices(init_containers, config)
	} else {
		r.DeleteStatefulSet("zuul-scheduler")
		r.DeleteDeployment("zuul-web")
		r.DeleteService("zuul-web")
		return true
	}
}

func (r *SFController) IngressZuul() netv1.IngressRule {
	return create_ingress_rule("zuul."+r.cr.Spec.FQDN, "zuul-web", 9000)
}
