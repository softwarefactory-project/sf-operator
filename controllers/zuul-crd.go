// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

// TODO: manage zuul-operator installation.
// In the meantime, run in another terminal:
// git clone https://github.com/softwarefactory-project/zuul-operator/
// tox -evenv
// WATCH_NAMESPACE=tristanc PYTHONPATH=$(pwd) ./.tox/venv/bin/kopf run zuul_operator/operator.py

package controllers

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/softwarefactory-project/sf-operator/api/zuul"
)

func (r *SFController) EnsureZuulDBSecret(db_password *apiv1.Secret) {
	secret := apiv1.Secret{
		Data: map[string][]byte{
			"dburi": []byte(fmt.Sprintf("mysql+pymysql://zuul:%s@mariadb/zuul", db_password.Data["zuul-db-password"])),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-db-uri", Namespace: r.ns},
	}
	r.GetOrCreate(&secret)

	// Initial config
	r.GetOrCreate(&apiv1.Secret{
		Data: map[string][]byte{
			"main.yaml": []byte("[]"),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-tenant-yaml", Namespace: r.ns},
	})
	r.GetOrCreate(&apiv1.Secret{
		Data: map[string][]byte{
			"nodepool.yaml": []byte(`
labels: []
providers: []
`),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-launcher-yaml", Namespace: r.ns},
	})
}

func (r *SFController) DeployZuulCRD(enabled bool) bool {
	if enabled {
		zuul := zuul.Zuul{
			ObjectMeta: metav1.ObjectMeta{Name: "zuul", Namespace: r.ns},
			Spec: zuul.ZuulSpec{
				Database: zuul.DatabaseSpec{
					SecretName: "zuul-db-uri",
				},
				Scheduler: zuul.SchedulerSpec{
					Config: zuul.SecretConfig{
						SecretName: "zuul-tenant-yaml",
					},
				},
				Launcher: zuul.LauncherSpec{
					Config: zuul.SecretConfig{
						SecretName: "zuul-launcher-yaml",
					},
				},
				Connections: map[string]zuul.ConnectionSpec{},
			},
		}
		return (zuul.Status.Ready)
	} else {
		// TODO: remove any left-over zuul
		return true
	}
}
