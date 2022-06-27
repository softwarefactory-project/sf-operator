// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

func (r *SFController) DeployZuul(enabled bool) bool {
	var dep appsv1.Deployment
	found := r.GetM("zuul", &dep)
	if !found && enabled {
		r.EnsureSSHKey("gerrit-ssh-key")
		db_password, db_ready := r.EnsureDB("zuul")
		if db_ready {
			r.log.V(1).Info("zuul DB is ready, deploying the service now!")

			secret := apiv1.Secret{
				Data: map[string][]byte{
					"zuul-db-uri": []byte(fmt.Sprintf("mysql+pymysql:://zuul:%s@mariadb/zuul", db_password.Data["zuul-db-password"])),
				},
				ObjectMeta: metav1.ObjectMeta{Name: "zuul-db-uri", Namespace: r.ns},
			}
			r.CreateR(&secret)
			return true;
		}
	}
	if enabled {
		return false;
	} else {
		return true;
	}
}
