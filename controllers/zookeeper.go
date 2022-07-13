// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/zookeeper.yaml
var zk_objs string

func create_client_certificate(ns string, name string, issuer string, secret string) certv1.Certificate {
	return certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.CertificateSpec{
			CommonName: "client",
			SecretName: secret,
			PrivateKey: &certv1.CertificatePrivateKey{
				Encoding: certv1.PKCS8,
			},
			IssuerRef: certmetav1.ObjectReference{
				Name: issuer,
				Kind: "Issuer",
			},
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageServerAuth,
				certv1.UsageClientAuth,
			},
		},
	}
}

func (r *SFController) DeployZK(enabled bool) bool {
	if enabled {
		r.CreateYAMLs(strings.ReplaceAll(zk_objs, "{{ NS }}", r.ns))
		cert := create_client_certificate(r.ns, "zookeeper-client", "ca-issuer", "zookeeper-client-tls")
		r.GetOrCreate(&cert)
		var dep appsv1.StatefulSet
		r.GetM("zookeeper", &dep)
		return r.IsStatefulSetReady(&dep)
	} else {
		r.DeleteStatefulSet("zookeeper")
		r.DeleteService("zookeeper")
		r.DeleteService("zookeeper-headless")
		return true
	}
}
