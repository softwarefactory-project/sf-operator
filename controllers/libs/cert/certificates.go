// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package cert provides various utility functions regarding handling the local CA for the sf-operator
package cert

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
)

const (
	LocalCACertSecretName = "ca-cert"
	// Valid for 30 years, which should be way more than the expected runtime of a deployment.
	validity = 30
)

func X509CA() (*x509.Certificate, *rsa.PrivateKey, *bytes.Buffer, *bytes.Buffer) {
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "SF Local CA",
			Organization: []string{"Red Hat, INC."},
		},
		NotBefore:             time.Now().AddDate(-1, 0, 0),
		NotAfter:              time.Now().AddDate(validity, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		logging.LogE(err, "Unable to generate a private key for the local CA")
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logging.LogE(err, "Unable to create local CA certificate")
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(caPrivKey)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs8,
	})
	return caCert, caPrivKey, caPEM, caPrivKeyPEM
}

func generateSerialNumber() (*big.Int, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	return serialNumber, nil
}

func X509Cert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, dnsNames []string) (*bytes.Buffer, *bytes.Buffer) {
	serialNumber, err := generateSerialNumber()
	if err != nil {
		logging.LogE(err, "Unable to generate a serial number for X509 certificate")
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		DNSNames:     dnsNames,
		Subject: pkix.Name{
			CommonName:   "Certificate",
			Organization: []string{"Red Hat, INC."},
		},
		NotBefore:   time.Now().AddDate(-1, 0, 0),
		NotAfter:    time.Now().AddDate(validity, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		logging.LogE(err, "Unable to generate a private key for X509 certificate")
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logging.LogE(err, "Unable to create X509 certificate")
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(certPrivKey)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs8,
	})

	return certPEM, certPrivKeyPEM
}
