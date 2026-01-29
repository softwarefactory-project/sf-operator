// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package controllers

import (
	"fmt"
	"os"
	"path/filepath"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

func (r *SFKubeContext) copySecrets(eCR sfv1.SoftwareFactory, controlEnv *SFKubeContext) error {
	secrets := []string{"ca-cert", "zookeeper-client-tls", "zuul-ssh-key"}

	// Collect zuul connections secret
	for _, conn := range eCR.Spec.Zuul.GerritConns {
		if conn.Sshkey != "" {
			secrets = append(secrets, conn.Sshkey)
		}
	}
	for _, conn := range eCR.Spec.Zuul.GitHubConns {
		if conn.Secrets != "" {
			secrets = append(secrets, conn.Secrets)
		}
	}
	for _, conn := range eCR.Spec.Zuul.PagureConns {
		if conn.Secrets != "" {
			secrets = append(secrets, conn.Secrets)
		}
	}
	for _, conn := range eCR.Spec.Zuul.GitLabConns {
		if conn.Secrets != "" {
			secrets = append(secrets, conn.Secrets)
		}
	}

	// Copy the secrets
	for _, secret := range secrets {
		var sec apiv1.Secret
		if !controlEnv.GetM(secret, &sec) {
			return fmt.Errorf("failed to read secret %s", secret)
		}
		sec.SetNamespace(r.Ns)
		r.EnsureSecret(&sec)
	}

	return nil
}

func (r *SFKubeContext) setupFingerLB() {
	svc := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "zuul-executor-lb", Namespace: r.Ns},
		Spec: apiv1.ServiceSpec{
			Type: "LoadBalancer",
			Selector: map[string]string{
				"app": "sf",
				"run": "zuul-executor",
			},
			Ports: []apiv1.ServicePort{
				{
					Name:       "zuul-executorf-7900",
					Port:       7900,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(7900),
				},
			},
		},
	}
	r.EnsureService(&svc)
}

func (r *SFKubeContext) setupRemoteExecutorConfig(crPath string, eCR sfv1.SoftwareFactory) error {
	if _, err := os.Stat(crPath); err != nil {
		return fmt.Errorf("missing control plane resource %s", crPath)
	}

	// Load the control plane resource
	var controlCR sfv1.SoftwareFactory
	controlCR, err := ReadSFYAML(crPath)
	if err != nil {
		return fmt.Errorf("couldn't load sfv1.SoftwareFactory from %s", crPath)
	}

	if controlCR.Spec.FQDN != eCR.Spec.FQDN {
		return fmt.Errorf("control plane %s fqdn '%s' doesn't match remote executor '%s'", crPath, controlCR.Spec.FQDN, eCR.Spec.FQDN)
	}

	// Setup kubeconfig
	cConfig := filepath.Dir(crPath) + "/kubeconfig"
	controlEnv, err := MkSFKubeContext(cConfig, "", "")
	if err != nil {
		return err
	}

	if err := r.copySecrets(eCR, &controlEnv); err != nil {
		return err
	}
	r.setupFingerLB()
	return nil
}
