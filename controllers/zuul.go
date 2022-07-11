// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/zuul.conf
var zuul_dot_conf string

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
	}
	if service == "zuul-scheduler" {
		volumes = append(volumes, apiv1.VolumeMount{
			Name:      "zuul-tenant-yaml",
			MountPath: "/etc/zuul/tenant",
			ReadOnly:  true,
		})
	}
	if service == "zuul-scheduler" || service == "zuul-executor" || service == "zuul-merger" {
		volumes = append(volumes,
			apiv1.VolumeMount{
				Name:      service,
				MountPath: "/var/lib/zuul",
			})
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
	return []apiv1.Volume{
		create_volume_secret("zuul-config"),
		create_volume_secret("zuul-tenant-yaml"),
		create_volume_secret("zookeeper-client-tls"),
	}
}

func (r *SFController) EnsureZuulServices(init_containers []apiv1.Container) {
	zs := create_statefulset(r.ns, "zuul-scheduler", "")
	zs.Spec.Template.Spec.InitContainers = init_containers
	zs.Spec.Template.Spec.Containers = create_zuul_container("zuul-scheduler")
	zs.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-scheduler")
	r.Apply(&zs)

	zw := create_deployment(r.ns, "zuul-web", "")
	zw.Spec.Template.Spec.Containers = create_zuul_container("zuul-web")
	zw.Spec.Template.Spec.Volumes = create_zuul_volumes("zuul-scheduler")
	r.Apply(&zw)

	srv := create_service(r.ns, "zuul-web", "zuul-web", 9000, "zuul-web")
	r.Apply(&srv)
}

func (r *SFController) EnsureZuulSecrets(db_password *apiv1.Secret) {
	secret := apiv1.Secret{
		Data: map[string][]byte{
			"dburi": []byte(fmt.Sprintf("mysql+pymysql://zuul:%s@mariadb/zuul", db_password.Data["zuul-db-password"])),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-db-uri", Namespace: r.ns},
	}
	r.Apply(&secret)

	// Initial config
	r.Apply(&apiv1.Secret{
		Data: map[string][]byte{
			"main.yaml": []byte("[]"),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-tenant-yaml", Namespace: r.ns},
	})

	r.EnsureSecret("zuul-keystore-password")

	r.Apply(&apiv1.Secret{
		Data: map[string][]byte{
			"zuul.conf": []byte(zuul_dot_conf),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-config", Namespace: r.ns},
	})

	r.Apply(&apiv1.Secret{
		Data: map[string][]byte{
			"zk-hosts": []byte(`zookeeper.` + r.ns + `:2281`),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zk-hosts", Namespace: r.ns},
	})
}

func (r *SFController) DeployZuul(enabled bool) bool {
	if enabled {
		init_containers, db_password := r.EnsureDBInit("zuul")
		r.EnsureZuulSecrets(&db_password)
		r.EnsureZuulServices(init_containers)
		return r.IsStatefulSetReady("zuul-scheduler") && r.IsDeploymentReady("zuul-web")
	} else {
		r.DeleteStatefulSet("zuul-scheduler")
		r.DeleteDeployment("zuul-web")
		r.DeleteService("zuul-web")
		return true
	}
}
