// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/zuul.yaml
var zuul_objs string

//go:embed templates/zuul.conf
var zuul_dot_conf string

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
		db_password, db_ready := r.EnsureDB("zuul")
		if db_ready {
			r.EnsureZuulSecrets(&db_password)
			r.CreateYAMLs(zuul_objs)
			return r.IsStatefulSetReady("zuul-scheduler") && r.IsDeploymentReady("zuul-web")
		}
		return false
	} else {
		r.DeleteStatefulSet("zuul-scheduler")
		r.DeleteDeployment("zuul-web")
		r.DeleteService("zuul-web")
		return true
	}
}
