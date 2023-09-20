// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

// Package gerrit provides gerrit utilities
package gerrit

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "embed"

	apiroutev1 "github.com/openshift/api/route/v1"
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
)

const managesfResourcesIdent string = "managesf-resources"
const gerritHTTPDPort = 8080
const gerritHTTPPort = 80
const gerritHTTPDPortName = "gerrit-httpd"
const gerritSSHDPort = 29418
const gerritSSHDPortName = "gerrit-sshd"
const gerritSiteMountPath = "/gerrit"
const gerritIdent = "gerrit"
const gerritImage = "quay.io/software-factory/gerrit:3.6.4-8"

//go:embed static/entrypoint.sh
var entrypoint string

//go:embed static/post-init.sh
var postInitScript string

//go:embed static/msf-entrypoint.sh
var managesfEntrypoint string

//go:embed static/init.sh
var gerritInitScript string

//go:embed static/config.py.tmpl
var managesfConf string

//go:embed static/create-repo.sh
var CreateRepoScript string

//go:embed static/create-ci-user.sh
var CreateCIUserScript string

type GerritCMDContext struct {
	env  *utils.ENV
	fqdn string
}

var ns = "sf"

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
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &secret)
	if err != nil && errors.IsNotFound(err) {
		secret = secretGen(name, g.env.Ns)
		err = g.env.Cli.Create(g.env.Ctx, &secret)
		notifByError(err, "secret", name)
	}
	return secret
}

func (g *GerritCMDContext) ensureService(name string, service apiv1.Service) {
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &apiv1.Service{})
	if err != nil && errors.IsNotFound(err) {
		err = g.env.Cli.Create(g.env.Ctx, &service)
		notifByError(err, "service", name)
	}
}

func (g *GerritCMDContext) ensureJob(name string, job batchv1.Job) {
	var curJob batchv1.Job
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &curJob)
	if err != nil && errors.IsNotFound(err) {
		err = g.env.Cli.Create(g.env.Ctx, &job)
		notifByError(err, "job", name)
	}
	for i := 0; i < 60; i++ {
		if curJob.Status.Succeeded >= 1 {
			return
		}
		time.Sleep(2 * time.Second)
		if err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &curJob); err != nil {
			panic(err)
		}
	}
	fmt.Println("failed to wait for batch job")
	os.Exit(1)
}

func (g *GerritCMDContext) ensureRoute(name string, route apiroutev1.Route) {
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &apiroutev1.Route{})
	if err != nil && errors.IsNotFound(err) {
		err = g.env.Cli.Create(g.env.Ctx, &route)
		notifByError(err, "route", name)
	}
}

func (g *GerritCMDContext) ensureCM(name string, data map[string]string) {
	cmName := name + "-config-map"
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: cmName, Namespace: g.env.Ns}, &apiv1.ConfigMap{})
	if err != nil && errors.IsNotFound(err) {
		cm := apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: g.env.Ns},
			Data:       data,
		}
		err = g.env.Cli.Create(g.env.Ctx, &cm)
		notifByError(err, "cm", name)
	}
}

func GerritHttpdService(ns string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gerritHTTPDPortName,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       gerritHTTPDPortName,
					Protocol:   apiv1.ProtocolTCP,
					Port:       gerritHTTPDPort,
					TargetPort: intstr.FromString(gerritHTTPDPortName),
				},
				{
					Name:       gerritHTTPDPortName + "-internal-http",
					Protocol:   apiv1.ProtocolTCP,
					Port:       gerritHTTPPort,
					TargetPort: intstr.FromString(gerritHTTPDPortName),
				},
			},
			Selector: map[string]string{
				"app": "sf",
				"run": gerritIdent,
			},
		}}
}

func GerritSshdService(ns string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gerritSSHDPortName,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     gerritSSHDPortName,
					Protocol: apiv1.ProtocolTCP,
					Port:     gerritSSHDPort,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": gerritIdent,
			},
		}}
}

func GenerateManageSFConfig(gerritadminpassword string, fqdn string) string {

	// Structure for config.py file template
	type ConfigPy struct {
		Fqdn                string
		GerritAdminPassword string
	}

	// Initializing Template Structure
	configpy := ConfigPy{
		fqdn,
		gerritadminpassword,
	}

	template, err := controllers.ParseString(managesfConf, configpy)
	if err != nil {
		panic("Template parsing failed")
	}

	return template
}

var ManageSFVolumes = []apiv1.Volume{
	controllers.MKVolumeCM(managesfResourcesIdent+"-config-vol",
		managesfResourcesIdent+"-config-map"),
	{
		Name: managesfResourcesIdent + "-tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: managesfResourcesIdent + "-tooling-config-map",
				},
				DefaultMode: &controllers.Execmod,
			},
		},
	},
}

func SetGerritSTSContainer(sts *appsv1.StatefulSet, volumeMounts []apiv1.VolumeMount, fqdn string) {
	sts.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", entrypoint}
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		controllers.MKContainerPort(gerritHTTPDPort, gerritHTTPDPortName),
		controllers.MKContainerPort(gerritSSHDPort, gerritSSHDPortName),
	}
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		controllers.MKEnvVar("HOME", "/gerrit"),
		controllers.MKEnvVar("FQDN", fqdn),
		controllers.MKSecretEnvVar("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = controllers.MkReadinessCMDProbe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].StartupProbe = controllers.MkStartupCMDProbe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = controllers.MkLivenessCMDProbe([]string{"bash", "/gerrit/ready.sh"})
}

func SetGerritMSFRContainer(sts *appsv1.StatefulSet, fqdn string) {
	container := controllers.MkContainer(managesfResourcesIdent, controllers.BusyboxImage)
	container.Command = []string{"sh", "-c", managesfEntrypoint}
	container.Env = []apiv1.EnvVar{
		controllers.MKEnvVar("HOME", "/tmp"),
		controllers.MKEnvVar("FQDN", fqdn),
		// managesf-resources need an admin ssh access to the local Gerrit
		controllers.MKSecretEnvVar("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      managesfResourcesIdent + "-config-vol",
			MountPath: "/etc/managesf",
		},
		{
			Name:      managesfResourcesIdent + "-tooling-vol",
			MountPath: "/usr/share/managesf",
		},
	}
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, container)
}

func SetGerritSTSVolumes(sts *appsv1.StatefulSet) {
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
		ManageSFVolumes...,
	)
}

func GerritPostInitContainer(jobName string, fqdn string) apiv1.Container {
	env := []apiv1.EnvVar{
		controllers.MKEnvVar("HOME", "/tmp"),
		controllers.MKEnvVar("FQDN", fqdn),
		controllers.MKSecretEnvVar("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
		controllers.MKSecretEnvVar("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
		controllers.MKSecretEnvVar("ZUUL_SSH_PUB_KEY", "zuul-ssh-key", "pub"),
		controllers.MKSecretEnvVar("ZUUL_HTTP_PASSWORD", "zuul-gerrit-api-key", "zuul-gerrit-api-key"),
	}

	container := controllers.MkContainer(fmt.Sprintf("%s-container", jobName), controllers.BusyboxImage)
	container.Command = []string{"sh", "-c", postInitScript}
	container.Env = env
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      managesfResourcesIdent + "-config-vol",
			MountPath: "/etc/managesf",
		},
		{
			Name:      managesfResourcesIdent + "-tooling-vol",
			MountPath: "/usr/share/managesf",
		},
	}
	return container
}

func GerritInitContainers(volumeMounts []apiv1.VolumeMount, fqdn string) apiv1.Container {
	container := controllers.MkContainer("gerrit-init", gerritImage)
	container.Command = []string{"sh", "-c", gerritInitScript}
	container.Env = []apiv1.EnvVar{
		controllers.MKSecretEnvVar("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
		controllers.MKEnvVar("FQDN", fqdn),
	}
	container.VolumeMounts = volumeMounts
	return container
}

func (g *GerritCMDContext) ensureGerritPostInitJob() {
	jobName := "post-init"
	job := controllers.MkJob(
		jobName, g.env.Ns,
		GerritPostInitContainer(jobName, g.fqdn),
	)
	job.Spec.Template.Spec.Volumes = ManageSFVolumes
	g.ensureJob(jobName, job)
}

func (g *GerritCMDContext) getSTS(name string) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	err := g.env.Cli.Get(g.env.Ctx, client.ObjectKey{Name: name, Namespace: g.env.Ns}, &sts)
	return sts, err
}

func (g *GerritCMDContext) isSTSReady(name string) bool {
	sts, _ := g.getSTS(name)
	return controllers.IsStatefulSetRolloutDone(&sts)
}

func (g *GerritCMDContext) ensureGerritSTS() {
	name := gerritIdent
	_, err := g.getSTS(name)
	if err != nil && errors.IsNotFound(err) {
		container := controllers.MkContainer(name, gerritImage)
		storageConfig := controllers.BaseGetStorageConfOrDefault(v1.StorageSpec{}, "")
		pvc := controllers.MkPVC(name, g.env.Ns, storageConfig, apiv1.ReadWriteOnce)
		sts := controllers.MkStatefulset(
			name, g.env.Ns, 1, name, container, pvc)
		volumeMounts := []apiv1.VolumeMount{
			{
				Name:      name,
				MountPath: gerritSiteMountPath,
			},
		}
		SetGerritSTSContainer(&sts, volumeMounts, g.fqdn)
		sts.Spec.Template.Spec.InitContainers = []apiv1.Container{
			GerritInitContainers(volumeMounts, g.fqdn),
		}

		SetGerritMSFRContainer(&sts, g.fqdn)

		SetGerritSTSVolumes(&sts)

		err = g.env.Cli.Create(g.env.Ctx, &sts)
		notifByError(err, "sts", name)
	}
}

func (g *GerritCMDContext) ensureGerritIngresses() {
	name := "gerrit"
	route := controllers.MkHTTSRoute(name, g.env.Ns, name,
		gerritHTTPDPortName, "/", gerritHTTPDPort, map[string]string{}, g.fqdn, nil)
	g.ensureRoute(name, route)
}

func EnsureGerrit(env *utils.ENV, fqdn string) {
	// Gerrit namespace creation
	err := env.Cli.Get(env.Ctx, client.ObjectKey{Name: env.Ns}, &apiv1.Namespace{})
	if err != nil && errors.IsNotFound(err) {
		nsR := apiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: env.Ns},
		}
		err = env.Cli.Create(env.Ctx, &nsR)
		if err != nil {
			fmt.Println("failed to create the namespace", env.Ns)
			os.Exit(1)
		} else {
			fmt.Println("created namespace", env.Ns)
		}
	}

	g := GerritCMDContext{
		env:  env,
		fqdn: fqdn,
	}

	// Ensure the admin SSH key pair secret
	g.ensureSecret("admin-ssh-key", controllers.CreateSSHKeySecret)

	// Ensure the zuul SSH key pair secret
	g.ensureSecret("zuul-ssh-key", controllers.CreateSSHKeySecret)

	// Ensure the admin API key secret
	adminAPIKeyName := "gerrit-admin-api-key"
	adminAPIKeySecret := g.ensureSecret(adminAPIKeyName, createAPIKeySecret)
	adminAPIKey, _ := controllers.GetValueFromKeySecret(adminAPIKeySecret, adminAPIKeyName)

	// Ensure the zuul API key secret
	g.ensureSecret("zuul-gerrit-api-key", createAPIKeySecret)

	// Ensure httpd Service
	g.ensureService(gerritHTTPDPortName, GerritHttpdService(ns))

	// Ensure sshd Service
	g.ensureService(gerritSSHDPortName, GerritSshdService(ns))

	// Ensure configMaps for managesf-resources
	cmData := make(map[string]string)
	cmData["config.py"] = GenerateManageSFConfig(string(adminAPIKey), fqdn)
	g.ensureCM(managesfResourcesIdent, cmData)
	toolingData := make(map[string]string)
	toolingData["create-repo.sh"] = CreateRepoScript
	toolingData["create-ci-user.sh"] = CreateCIUserScript
	g.ensureCM(managesfResourcesIdent+"-tooling", toolingData)

	// Ensure gerrit statefulset
	g.ensureGerritSTS()

	// Wait for Gerrit statefullSet ready
	for !g.isSTSReady(gerritIdent) {
		fmt.Println("Wait for gerrit sts to be ready ...")
		time.Sleep(10 * time.Second)
	}

	// Start Post Init Job
	g.ensureGerritPostInitJob()

	// Ensure the Ingress route
	g.ensureGerritIngresses()
}

func WipeGerrit(env *utils.ENV) {
	cl := env.Cli
	ctx := env.Ctx
	ns := env.Ns
	// Delete secrets
	cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "admin-ssh-key", Namespace: ns}})
	cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zuul-ssh-key", Namespace: ns}})
	cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gerrit-admin-api-key", Namespace: ns}})
	cl.Delete(ctx, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zuul-gerrit-api-key", Namespace: ns}})

	// Delete services
	cl.Delete(ctx,
		&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: gerritHTTPDPortName, Namespace: ns}})
	cl.Delete(ctx,
		&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: gerritSSHDPortName, Namespace: ns}})

	// Delete Gerrit STS and the associated Statefulset
	cl.Delete(ctx,
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: gerritIdent, Namespace: ns}})
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
		&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: managesfResourcesIdent + "-config-map", Namespace: ns}})
	cl.Delete(ctx,
		&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: managesfResourcesIdent + "-tooling-config-map", Namespace: ns}})

	// Delete Gerrit route
	cl.Delete(ctx,
		&apiroutev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "gerrit", Namespace: ns}})
}

var GerritCmd = &cobra.Command{
	Use:   "gerrit",
	Short: "Deploy a demo Gerrit instance to hack on sf-operator",
	Run: func(cmd *cobra.Command, args []string) {
		deploy, _ := cmd.Flags().GetBool("deploy")
		wipe, _ := cmd.Flags().GetBool("wipe")
		fqdn, _ := cmd.Flags().GetString("fqdn")

		if !(deploy || wipe) {
			println("Select one of deploy or wipe option")
			os.Exit(1)
		}

		// Get the kube client
		cl := utils.CreateKubernetesClientOrDie("")
		ctx := context.Background()
		env := utils.ENV{
			Cli: cl,
			Ns:  ns,
			Ctx: ctx,
		}
		if deploy {
			fmt.Println("Ensure Gerrit deployed in namespace", ns)
			EnsureGerrit(&env, fqdn)
			fmt.Printf("Gerrit is available at https://gerrit.%s\n", fqdn)
		}

		if wipe {
			fmt.Println("Wipe Gerrit from namespace", ns)

			WipeGerrit(&env)
		}

	},
}

func init() {
	GerritCmd.Flags().BoolP("deploy", "", false, "Deploy Gerrit")
	GerritCmd.Flags().BoolP("wipe", "", false, "Wipe Gerrit deployment")
	GerritCmd.PersistentFlags().StringP("fqdn", "f", "sftests.com", "The FQDN of gerrit (gerrit.<FQDN>)")
}
