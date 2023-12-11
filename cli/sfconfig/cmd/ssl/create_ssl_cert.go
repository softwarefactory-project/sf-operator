/*
Copyright © 2023 Redhat
SPDX-License-Identifier: Apache-2.0
*/

// Package createssl functions
package createssl

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/spf13/cobra"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sf "github.com/softwarefactory-project/sf-operator/controllers"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/config"
)

func ensureSSLSecret(env *utils.ENV, serviceCAContent []byte,
	serviceCertContent []byte, serviceKeyContent []byte) {
	var secret apiv1.Secret
	secretName := sf.CustomSSLSecretName
	data := map[string][]byte{
		"CA":  serviceCAContent,
		"crt": serviceCertContent,
		"key": serviceKeyContent,
	}
	if !utils.GetM(env, secretName, &secret) {
		// Create the secret as it does not exists
		secret := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: env.Ns,
			},
			Data: data,
		}
		utils.CreateR(env, &secret)
	} else {
		// Update the secret data
		secret.Data = data
		utils.UpdateR(env, &secret)
	}
}

func verifySSLCert(serviceCAContent []byte, serviceCertContent []byte,
	serviceKeyContent []byte, serverName string) bool {
	// Verify if provided cert is correct
	decodedCACert, _ := pem.Decode(serviceCAContent)
	if decodedCACert == nil {
		panic("Failed to decode CA certificate!")
	}
	caCert, err := x509.ParseCertificate(decodedCACert.Bytes)
	if err != nil {
		panic("Failed to parse CA certificate: " + err.Error())
	}

	decodedClientCert, _ := pem.Decode(serviceCertContent)
	clientCert, err := x509.ParseCertificate(decodedClientCert.Bytes)
	if err != nil {
		panic("Failed to parse certificate: " + err.Error())
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: roots,
		DNSName:       serverName,
	}

	_, err = clientCert.Verify(opts)
	if err == nil {
		fmt.Println("Certificate is valid and signed by the local CA authority!")
		return true
	} else {
		fmt.Println("Certificate verification failed:", err)
		return false
	}

}

func CreateServiceCertSecret(sfEnv *utils.ENV, sfNamespace string,
	sfServiceCA string, sfServiceCert string,
	sfServiceKey string, serverName string,
) {
	kubernetesEnv := utils.ENV{Cli: sfEnv.Cli, Ctx: sfEnv.Ctx, Ns: sfNamespace}

	var err error
	var serviceCAContent, serviceCertContent, serviceKeyContent []byte

	if serviceCAContent, err = utils.GetFileContent(sfServiceCA); err != nil {
		panic("Unable to read " + sfServiceCA)
	}
	if serviceCertContent, err = utils.GetFileContent(sfServiceCert); err != nil {
		panic("Unable to read " + sfServiceCert)

	}
	if serviceKeyContent, err = utils.GetFileContent(sfServiceKey); err != nil {
		panic("Unable to read " + sfServiceKey)
	}

	if serviceCAContent == nil || serviceCertContent == nil ||
		serviceKeyContent == nil {
		panic("One of the provided files is empty! Can not continue")
	}
	if !verifySSLCert(serviceCAContent, serviceCertContent, serviceKeyContent, serverName) {
		panic("Provided certificates does not fit with provided address " + serverName)
	}

	ensureSSLSecret(&kubernetesEnv, serviceCAContent, serviceCertContent,
		serviceKeyContent)

}

var CreateCertificateCmd = &cobra.Command{
	Use:   "create-service-ssl-secret",
	Short: "Create secret for service SSL certificate",
	Long:  "This command adds secret with SSL certificate content",

	Run: func(cmd *cobra.Command, args []string) {
		sfNamespace, _ := cmd.Flags().GetString("sf-namespace")
		sfContext, _ := cmd.Flags().GetString("sf-context")
		sfServiceCA, _ := cmd.Flags().GetString("sf-service-ca")
		sfServiceCert, _ := cmd.Flags().GetString("sf-service-cert")
		sfServiceKey, _ := cmd.Flags().GetString("sf-service-key")
		sfEnv := utils.ENV{
			Cli: utils.CreateKubernetesClientOrDie(sfContext),
			Ctx: context.TODO(),
			Ns:  sfNamespace,
		}
		conf := config.GetSFConfigOrDie()
		CreateServiceCertSecret(&sfEnv, sfNamespace, sfServiceCA,
			sfServiceCert, sfServiceKey, conf.FQDN)

	},
}

func init() {
	CreateCertificateCmd.Flags().StringP("sf-namespace", "", "sf",
		"Name of the namespace to copy the kubeconfig, or '-' for stdout")
	CreateCertificateCmd.Flags().StringP("sf-context", "", "",
		"The kubeconfig context of the sf-namespace, use the default context by default")
	CreateCertificateCmd.Flags().StringP("sf-service-ca", "", "",
		"Path for the service CA certificate")
	CreateCertificateCmd.Flags().StringP("sf-service-cert", "", "",
		"Path for the service certificate file")
	CreateCertificateCmd.Flags().StringP("sf-service-key", "", "",
		"Path for the service private key file")
}
