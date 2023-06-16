// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	apiroutev1 "github.com/openshift/api/route/v1"
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type GerritCMDContext struct {
	cl  client.Client
	ns  string
	ctx context.Context
}

var fqdn = "sf.dev"

func notifByError(err error, oType string, name string) {
	if err != nil {
		fmt.Println("failed to create", oType, name, err)
		os.Exit(1)
	} else {
		fmt.Println("created", oType, name)
	}
}

func createAPIKeySecret(name string, ns string) apiv1.Secret {
	return controllers.CreateSecretFromFunc(name, ns, controllers.NewUUIDString)
}

func (g *GerritCMDContext) ensureSecret(
	name string, secretGen func(string, string) apiv1.Secret) apiv1.Secret {
	secret := apiv1.Secret{}
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: name, Namespace: g.ns}, &secret)
	if err != nil && errors.IsNotFound(err) {
		secret = secretGen(name, g.ns)
		err = g.cl.Create(g.ctx, &secret)
		notifByError(err, "secret", name)
	}
	return secret
}

func (g *GerritCMDContext) ensureService(name string, service apiv1.Service) {
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: name, Namespace: g.ns}, &apiv1.Service{})
	if err != nil && errors.IsNotFound(err) {
		err = g.cl.Create(g.ctx, &service)
		notifByError(err, "service", name)
	}
}

func (g *GerritCMDContext) ensureJob(name string, job batchv1.Job) {
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: name, Namespace: g.ns}, &batchv1.Job{})
	if err != nil && errors.IsNotFound(err) {
		err = g.cl.Create(g.ctx, &job)
		notifByError(err, "job", name)
	}
}

func (g *GerritCMDContext) ensureRoute(name string, route apiroutev1.Route) {
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: name, Namespace: g.ns}, &apiroutev1.Route{})
	if err != nil && errors.IsNotFound(err) {
		err = g.cl.Create(g.ctx, &route)
		notifByError(err, "route", name)
	}
}

func (g *GerritCMDContext) ensureCM(name string, data map[string]string) {
	cmName := name + "-config-map"
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: cmName, Namespace: g.ns}, &apiv1.ConfigMap{})
	if err != nil && errors.IsNotFound(err) {
		cm := apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: g.ns},
			Data:       data,
		}
		err = g.cl.Create(g.ctx, &cm)
		notifByError(err, "cm", name)
	}
}

func (g *GerritCMDContext) ensureGerritPostInitJob() {
	job_name := "post-init"
	job := controllers.MkJob(
		job_name, g.ns,
		controllers.GerritPostInitContainer(job_name, fqdn),
	)
	job.Spec.Template.Spec.Volumes = controllers.ManageSFVolumes
	g.ensureJob(job_name, job)
}

func (g *GerritCMDContext) getSTS(name string) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	err := g.cl.Get(g.ctx, client.ObjectKey{Name: name, Namespace: g.ns}, &sts)
	return sts, err
}

func (g *GerritCMDContext) isSTSReady(name string) bool {
	sts, _ := g.getSTS(name)
	return controllers.IsStatefulSetRolloutDone(&sts)
}

func (g *GerritCMDContext) ensureGerritSTS() {
	name := controllers.GERRIT_IDENT
	_, err := g.getSTS(name)
	if err != nil && errors.IsNotFound(err) {
		container := controllers.MkContainer(name, controllers.GERRIT_IMAGE)
		storage_config := controllers.BaseGetStorageConfOrDefault(v1.StorageSpec{}, "")
		pvc := controllers.MkPVC(name, g.ns, storage_config)
		sts := controllers.MkStatefulset(
			name, g.ns, 1, name, container, pvc)
		volumeMounts := []apiv1.VolumeMount{
			{
				Name:      name,
				MountPath: controllers.GERRIT_SITE_MOUNT_PATH,
			},
		}
		controllers.SetGerritSTSContainer(&sts, volumeMounts, fqdn)
		sts.Spec.Template.Spec.InitContainers = []apiv1.Container{
			controllers.GerritInitContainers(volumeMounts, fqdn),
		}

		controllers.SetGerritMSFRContainer(&sts, fqdn)

		controllers.SetGerritSTSVolumes(&sts)

		err = g.cl.Create(g.ctx, &sts)
		notifByError(err, "sts", name)
	}
}

func (g *GerritCMDContext) ensureGerritIngresses() {
	name := "gerrit"
	route := controllers.MkHTTSRoute(name, g.ns, name,
		controllers.GERRIT_HTTPD_PORT_NAME, "/", controllers.GERRIT_HTTPD_PORT, map[string]string{}, fqdn)
	g.ensureRoute(name, route)
}

var gerritCmd = &cobra.Command{
	Use:   "gerrit",
	Short: "Deploy a demo Gerrit instance to hack on sf-operator",
	Run: func(cmd *cobra.Command, args []string) {
		deploy, _ := cmd.Flags().GetBool("deploy")
		wipe, _ := cmd.Flags().GetBool("wipe")

		if !(deploy || wipe) {
			println("Select one of deploy or wipe option")
			os.Exit(1)
		}

		// Get the kube client
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(apiroutev1.AddToScheme(scheme))
		cl, err := client.New(config.GetConfigOrDie(), client.Options{
			Scheme: scheme,
		})
		if err != nil {
			fmt.Println("failed to create client")
			os.Exit(1)
		}

		ctx := context.Background()
		ns := "gerrit"

		if deploy {
			fmt.Println("Ensure Gerrit deployed in namespace", ns)

			// Gerrit namespace creation
			err = cl.Get(ctx, client.ObjectKey{Name: ns}, &apiv1.Namespace{})
			if err != nil && errors.IsNotFound(err) {
				nsR := apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: ns},
				}
				err = cl.Create(context.Background(), &nsR)
				if err != nil {
					fmt.Println("failed to create the namespace", ns)
					os.Exit(1)
				} else {
					fmt.Println("created namespace", ns)
				}
			}

			g := GerritCMDContext{
				cl:  cl,
				ns:  ns,
				ctx: ctx,
			}

			// Ensure the admin SSH key pair secret
			g.ensureSecret("admin-ssh-key", controllers.CreateSSHKeySecret)

			// Ensure the zuul SSH key pair secret
			g.ensureSecret("zuul-ssh-key", controllers.CreateSSHKeySecret)

			// Ensure the admin API key secret
			adminApiKeyName := "gerrit-admin-api-key"
			adminApiKeySecret := g.ensureSecret(adminApiKeyName, createAPIKeySecret)
			adminApiKey, _ := controllers.GetValueFromKeySecret(adminApiKeySecret, adminApiKeyName)

			// Ensure the zuul API key secret
			g.ensureSecret("zuul-gerrit-api-key", createAPIKeySecret)

			// Ensure httpd Service
			g.ensureService(controllers.GERRIT_HTTPD_PORT_NAME, controllers.GerritHttpdService(ns))

			// Ensure sshd Service
			g.ensureService(controllers.GERRIT_SSHD_PORT_NAME, controllers.GerritSshdService(ns))

			// Ensure configMaps for managesf-resources
			cm_data := make(map[string]string)
			cm_data["config.py"] = controllers.GenerateManageSFConfig(string(adminApiKey), fqdn)
			g.ensureCM(controllers.MANAGESF_RESOURCES_IDENT, cm_data)
			tooling_data := make(map[string]string)
			tooling_data["create-repo.sh"] = controllers.CreateRepoScript
			tooling_data["create-ci-user.sh"] = controllers.CreateCIUserScript
			g.ensureCM(controllers.MANAGESF_RESOURCES_IDENT+"-tooling", tooling_data)

			// Ensure gerrit statefulset
			g.ensureGerritSTS()

			// Wait for Gerrit statefullSet ready
			for !g.isSTSReady(controllers.GERRIT_IDENT) {
				fmt.Println("Wait for gerrit sts to be ready ...")
				time.Sleep(10 * time.Second)
			}

			// Start Post Init Job
			g.ensureGerritPostInitJob()

			// Ensure the Ingress route
			g.ensureGerritIngresses()

			fmt.Printf("Gerrit is available at https://gerrit.%s\n", fqdn)

		}

		if wipe {
			fmt.Println("Wipe Gerrit from namespace", ns)

			// Delete secrets
			cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "admin-ssh-key", Namespace: ns}})
			cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zuul-ssh-key", Namespace: ns}})
			cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gerrit-admin-api-key", Namespace: ns}})
			cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zuul-gerrit-api-key", Namespace: ns}})

			// Delete services
			cl.Delete(ctx,
				&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: controllers.GERRIT_HTTPD_PORT_NAME, Namespace: ns}})
			cl.Delete(ctx,
				&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: controllers.GERRIT_SSHD_PORT_NAME, Namespace: ns}})

			// Delete Gerrit STS and the associated Statefulset
			cl.Delete(ctx,
				&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: controllers.GERRIT_IDENT, Namespace: ns}})
			cl.Delete(ctx,
				&apiv1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "gerrit-gerrit-0", Namespace: ns}})

			// Delete post init job
			backgroundDeletion := metav1.DeletePropagationBackground
			cl.Delete(ctx,
				&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "post-init", Namespace: ns}},
				&client.DeleteOptions{
					PropagationPolicy: &backgroundDeletion,
				},
			)

			// Delete managesf-resources ConfigMap
			cl.Delete(ctx,
				&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: controllers.MANAGESF_RESOURCES_IDENT + "-config-map", Namespace: ns}})
			cl.Delete(ctx,
				&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: controllers.MANAGESF_RESOURCES_IDENT + "-tooling-config-map", Namespace: ns}})

			// Delete Gerrit route
			cl.Delete(ctx,
				&apiroutev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "gerrit", Namespace: ns}})
		}

	},
}

func init() {
	rootCmd.AddCommand(gerritCmd)
	gerritCmd.Flags().BoolP("deploy", "", false, "Deploy Gerrit")
	gerritCmd.Flags().BoolP("wipe", "", false, "Wipe Gerrit deployment")
}
