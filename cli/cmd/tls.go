/*
Copyright © 2023 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

/*
"tls" subcommand configures TLS certificates for a SF instance.
*/

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	sf "github.com/softwarefactory-project/sf-operator/controllers"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ensureTLSSecret(env *cliutils.ENV, CAContents []byte,
	CertificateContents []byte, KeyContents []byte) {
	var secret apiv1.Secret
	secretName := sf.CustomSSLSecretName
	data := map[string][]byte{
		"CA":  CAContents,
		"crt": CertificateContents,
		"key": KeyContents,
	}
	if !cliutils.GetMOrDie(env, secretName, &secret) {
		// Create the secret as it does not exists
		secret := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: env.Ns,
			},
			Data: data,
		}
		cliutils.CreateROrDie(env, &secret)
	} else {
		// Update the secret data
		secret.Data = data
		cliutils.UpdateROrDie(env, &secret)
	}
}

func verifyCertificates(CAContents []byte, CertificateContents []byte,
	KeyContents []byte, serverName string) error {
	// Verify if provided cert is correct
	decodedCACert, _ := pem.Decode(CAContents)
	if decodedCACert == nil {
		ctrl.Log.Error(errors.New("no PEM data found"),
			"Error while PEM-decoding the Certificate Authority's certificate")
		os.Exit(1)
	}
	caCert, err := x509.ParseCertificate(decodedCACert.Bytes)
	if err != nil {
		ctrl.Log.Error(err, "Error parsing the Certificate Authority's certificate")
		os.Exit(1)
	}

	decodedClientCert, _ := pem.Decode(CertificateContents)
	clientCert, err := x509.ParseCertificate(decodedClientCert.Bytes)
	if err != nil {
		ctrl.Log.Error(err, "Error parsing the certificate")
		os.Exit(1)
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: roots,
		DNSName:       serverName,
	}

	_, err = clientCert.Verify(opts)
	return err
}

func configureTLS(ns string, kubeContext string, CAPath string, CertificatePath string, KeyPath string, fqdn string) {
	var err error
	var CAContents, CertificateContents, KeyContents []byte
	if CAContents, err = cliutils.GetFileContent(CAPath); err != nil {
		ctrl.Log.Error(err, "Error opening "+CAPath)
		os.Exit(1)
	}
	if CertificateContents, err = cliutils.GetFileContent(CertificatePath); err != nil {
		ctrl.Log.Error(err, "Error opening "+CertificatePath)
		os.Exit(1)
	}
	if KeyContents, err = cliutils.GetFileContent(KeyPath); err != nil {
		ctrl.Log.Error(err, "Error opening "+KeyPath)
		os.Exit(1)
	}
	if CAContents == nil || CertificateContents == nil || KeyContents == nil {
		ctrl.Log.Error(errors.New("empty file"), "At least one of the provided files has no contents")
		os.Exit(1)
	}
	if err = verifyCertificates(CAContents, CertificateContents, KeyContents, fqdn); err != nil {
		ctrl.Log.Error(err, "Certificates verification failed")
		os.Exit(1)
	}
	env := cliutils.ENV{
		Cli: cliutils.CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	ensureTLSSecret(&env, CAContents, CertificateContents, KeyContents)
}

func TLSConfigureCmd(kmd *cobra.Command, args []string) {
	cliCtx, err := cliutils.GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	fqdn := cliCtx.FQDN
	CAPath, _ := kmd.Flags().GetString("CA")
	CertificatePath, _ := kmd.Flags().GetString("cert")
	KeyPath, _ := kmd.Flags().GetString("key")
	configureTLS(ns, kubeContext, CAPath, CertificatePath, KeyPath, fqdn)
}
