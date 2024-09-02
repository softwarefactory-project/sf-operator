// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package cert provides various utility functions regarding handling cert-manager for the sf-operator
package cert

import (
	"time"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LocalCACertSecretName = "ca-cert"
)

var EonDuration, _ = time.ParseDuration("219000h") // 25 years

func MkBaseCertificate(name string, ns string, issuerName string,
	dnsNames []string, secretName string, isCA bool, duration time.Duration,
	usages []certv1.KeyUsage, commonName *string,
	privateKey *certv1.CertificatePrivateKey) certv1.Certificate {
	renewBefore, _ := time.ParseDuration("168h") // 7 days
	cert := certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.CertificateSpec{
			DNSNames:    dnsNames,
			Duration:    &metav1.Duration{Duration: EonDuration},
			RenewBefore: &metav1.Duration{Duration: renewBefore},
			SecretName:  secretName,
			IssuerRef: cmmeta.ObjectReference{
				Kind: "Issuer",
				Name: issuerName,
			},
			IsCA:   isCA,
			Usages: usages,
		},
	}
	if commonName != nil {
		cert.Spec.CommonName = *commonName
	}
	if privateKey != nil {
		cert.Spec.PrivateKey = privateKey
	}
	return cert
}

func MkCertificate(name string, ns string, issuerName string,
	dnsNames []string, secretName string, privateKey *certv1.CertificatePrivateKey, duration time.Duration) certv1.Certificate {
	usages := []certv1.KeyUsage{
		certv1.UsageServerAuth,
		certv1.UsageClientAuth,
		certv1.UsageDigitalSignature,
		certv1.UsageKeyEncipherment,
	}
	return MkBaseCertificate(
		name, ns, issuerName, dnsNames, secretName, false, duration, usages, nil, privateKey)
}

func IsCertificateReady(cert *certv1.Certificate) bool {
	for _, condition := range cert.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func MkLetsEncryptIssuer(name string, ns string, server string) certv1.Issuer {
	return certv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				ACME: &cmacme.ACMEIssuer{
					Server:         server,
					PreferredChain: "ISRG Root X1",
					PrivateKey: cmmeta.SecretKeySelector{
						LocalObjectReference: cmmeta.LocalObjectReference{
							Name: name,
						},
					},
					Solvers: []cmacme.ACMEChallengeSolver{
						{
							HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
								Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{},
							},
						},
					},
				},
			},
		},
	}
}

func MkSelfSignedIssuer(name string, ns string) certv1.Issuer {
	return certv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				SelfSigned: &certv1.SelfSignedIssuer{},
			},
		},
	}
}

func MkCAIssuer(name string, ns string) certv1.Issuer {
	return certv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				CA: &certv1.CAIssuer{
					SecretName: LocalCACertSecretName,
				},
			},
		},
	}
}
