// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *SFController) EnsureZuulDBSecret(db_password *apiv1.Secret) {
	var secret apiv1.Secret
	found := r.GetM("zuul-db-uri", &secret)
	if !found {
		secret := apiv1.Secret{
			Data: map[string][]byte{
				"zuul-db-uri": []byte(fmt.Sprintf("mysql+pymysql:://zuul:%s@mariadb/zuul", db_password.Data["zuul-db-password"])),
			},
			ObjectMeta: metav1.ObjectMeta{Name: "zuul-db-uri", Namespace: r.ns},
		}
		r.CreateR(&secret)
	}
}

func (r *SFController) DeployZuul(enabled bool) bool {
	var dep appsv1.StatefulSet
	found := r.GetM("zuul-scheduler", &dep)
	if !found && enabled {
		db_password, db_ready := r.EnsureDB("zuul")
		if db_ready {
			r.log.V(1).Info("zuul DB is ready, deploying the service now!")
			r.EnsureZuulDBSecret(&db_password)
			r.CreateYAMLs(ZUUL_OBJS)
			return false
		}
	} else if found {
		if !enabled {
			r.log.V(1).Info("Zuul deployment found, but it's not enabled, deleting it now")
			r.DeleteR(&dep)
		}
	}
	if enabled {
		// Wait for the service to be ready.
		return (dep.Status.ReadyReplicas > 0)
	} else {
		return true
	}
}

const ZUUL_OBJS = `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: zookeeper-client
spec:
  keyEncoding: pkcs8
  secretName: zookeeper-client-tls
  commonName: client
  usages:
    - digital signature
    - key encipherment
    - server auth
    - client auth
  issuerRef:
    name: ca-issuer
    kind: Issuer
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: zuul-scheduler
spec:
  replicas: 1
  serviceName: zuul-scheduler
  selector:
    matchLabels:
      app.kubernetes.io/name: zuul
      app.kubernetes.io/part-of: zuul
      app.kubernetes.io/component: zuul-scheduler
  template:
    metadata:
      labels:
        app.kubernetes.io/name: zuul
        app.kubernetes.io/part-of: zuul
        app.kubernetes.io/component: zuul-scheduler
    spec:
      containers:
      - name: scheduler
        image: docker.io/zuul/zuul-scheduler:latest
        args: ["/usr/local/bin/zuul-scheduler", "-f", "-d"]
        volumeMounts:
        - name: zuul-scheduler
          mountPath: /var/lib/zuul
        - name: zookeeper-client-tls
          mountPath: /tls/client
          readOnly: true
      volumes:
      - name: zookeeper-client-tls
        secret:
          secretName: zookeeper-client-tls
  volumeClaimTemplates:
  - metadata:
      name: zuul-scheduler
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 8Gi
`
