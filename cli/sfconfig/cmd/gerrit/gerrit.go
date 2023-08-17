// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gerrit configuration.

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

const MANAGESF_RESOURCES_IDENT string = "managesf-resources"
const GERRIT_HTTPD_PORT = 8080
const GERRIT_HTTPD_HTTP_PORT = 80
const GERRIT_HTTPD_PORT_NAME = "gerrit-httpd"
const GERRIT_SSHD_PORT = 29418
const GERRIT_SSHD_PORT_NAME = "gerrit-sshd"
const GERRIT_SITE_MOUNT_PATH = "/gerrit"
const GERRIT_IDENT = "gerrit"
const GERRIT_IMAGE = "quay.io/software-factory/gerrit:3.6.4-8"

//go:embed static/entrypoint.sh
var entrypoint string

//go:embed static/post-init.sh
var postInitScript string

//go:embed static/msf-entrypoint.sh
var managesf_entrypoint string

//go:embed static/init.sh
var gerritInitScript string

//go:embed static/config.py.tmpl
var managesf_conf string

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
			Name:      GERRIT_HTTPD_PORT_NAME,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       GERRIT_HTTPD_PORT_NAME,
					Protocol:   apiv1.ProtocolTCP,
					Port:       GERRIT_HTTPD_PORT,
					TargetPort: intstr.FromString(GERRIT_HTTPD_PORT_NAME),
				},
				{
					Name:       GERRIT_HTTPD_PORT_NAME + "-internal-http",
					Protocol:   apiv1.ProtocolTCP,
					Port:       GERRIT_HTTPD_HTTP_PORT,
					TargetPort: intstr.FromString(GERRIT_HTTPD_PORT_NAME),
				},
			},
			Selector: map[string]string{
				"app": "sf",
				"run": GERRIT_IDENT,
			},
		}}
}

func GerritSshdService(ns string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GERRIT_SSHD_PORT_NAME,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     GERRIT_SSHD_PORT_NAME,
					Protocol: apiv1.ProtocolTCP,
					Port:     GERRIT_SSHD_PORT,
				},
			},
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "sf",
				"run": GERRIT_IDENT,
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

	template, err := controllers.Parse_string(managesf_conf, configpy)
	if err != nil {
		panic("Template parsing failed")
	}

	return template
}

var ManageSFVolumes = []apiv1.Volume{
	controllers.Create_volume_cm(MANAGESF_RESOURCES_IDENT+"-config-vol",
		MANAGESF_RESOURCES_IDENT+"-config-map"),
	{
		Name: MANAGESF_RESOURCES_IDENT + "-tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: MANAGESF_RESOURCES_IDENT + "-tooling-config-map",
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
		controllers.Create_container_port(GERRIT_HTTPD_PORT, GERRIT_HTTPD_PORT_NAME),
		controllers.Create_container_port(GERRIT_SSHD_PORT, GERRIT_SSHD_PORT_NAME),
	}
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		controllers.Create_env("HOME", "/gerrit"),
		controllers.Create_env("FQDN", fqdn),
		controllers.Create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = controllers.Create_readiness_cmd_probe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].StartupProbe = controllers.Create_startup_cmd_probe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = controllers.Create_liveness_cmd_probe([]string{"bash", "/gerrit/ready.sh"})
}

func SetGerritMSFRContainer(sts *appsv1.StatefulSet, fqdn string) {
	container := controllers.MkContainer(MANAGESF_RESOURCES_IDENT, controllers.BUSYBOX_IMAGE)
	container.Command = []string{"sh", "-c", managesf_entrypoint}
	container.Env = []apiv1.EnvVar{
		controllers.Create_env("HOME", "/tmp"),
		controllers.Create_env("FQDN", fqdn),
		// managesf-resources need an admin ssh access to the local Gerrit
		controllers.Create_secret_env("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-config-vol",
			MountPath: "/etc/managesf",
		},
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-tooling-vol",
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

func GerritPostInitContainer(job_name string, fqdn string) apiv1.Container {
	env := []apiv1.EnvVar{
		controllers.Create_env("HOME", "/tmp"),
		controllers.Create_env("FQDN", fqdn),
		controllers.Create_secret_env("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
		controllers.Create_secret_env("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
		controllers.Create_secret_env("ZUUL_SSH_PUB_KEY", "zuul-ssh-key", "pub"),
		controllers.Create_secret_env("ZUUL_HTTP_PASSWORD", "zuul-gerrit-api-key", "zuul-gerrit-api-key"),
	}

	container := controllers.MkContainer(fmt.Sprintf("%s-container", job_name), controllers.BUSYBOX_IMAGE)
	container.Command = []string{"sh", "-c", postInitScript}
	container.Env = env
	container.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-config-vol",
			MountPath: "/etc/managesf",
		},
		{
			Name:      MANAGESF_RESOURCES_IDENT + "-tooling-vol",
			MountPath: "/usr/share/managesf",
		},
	}
	return container
}

func GerritInitContainers(volumeMounts []apiv1.VolumeMount, fqdn string) apiv1.Container {
	container := controllers.MkContainer("gerrit-init", GERRIT_IMAGE)
	container.Command = []string{"sh", "-c", gerritInitScript}
	container.Env = []apiv1.EnvVar{
		controllers.Create_secret_env("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
		controllers.Create_env("FQDN", fqdn),
	}
	container.VolumeMounts = volumeMounts
	return container
}

func (g *GerritCMDContext) ensureGerritPostInitJob() {
	job_name := "post-init"
	job := controllers.MkJob(
		job_name, g.env.Ns,
		GerritPostInitContainer(job_name, g.fqdn),
	)
	job.Spec.Template.Spec.Volumes = ManageSFVolumes
	g.ensureJob(job_name, job)
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
	name := GERRIT_IDENT
	_, err := g.getSTS(name)
	if err != nil && errors.IsNotFound(err) {
		container := controllers.MkContainer(name, GERRIT_IMAGE)
		storage_config := controllers.BaseGetStorageConfOrDefault(v1.StorageSpec{}, "")
		pvc := controllers.MkPVC(name, g.env.Ns, storage_config)
		sts := controllers.MkStatefulset(
			name, g.env.Ns, 1, name, container, pvc)
		volumeMounts := []apiv1.VolumeMount{
			{
				Name:      name,
				MountPath: GERRIT_SITE_MOUNT_PATH,
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
		GERRIT_HTTPD_PORT_NAME, "/", GERRIT_HTTPD_PORT, map[string]string{}, g.fqdn)
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
	adminApiKeyName := "gerrit-admin-api-key"
	adminApiKeySecret := g.ensureSecret(adminApiKeyName, createAPIKeySecret)
	adminApiKey, _ := controllers.GetValueFromKeySecret(adminApiKeySecret, adminApiKeyName)

	// Ensure the zuul API key secret
	g.ensureSecret("zuul-gerrit-api-key", createAPIKeySecret)

	// Ensure httpd Service
	g.ensureService(GERRIT_HTTPD_PORT_NAME, GerritHttpdService(ns))

	// Ensure sshd Service
	g.ensureService(GERRIT_SSHD_PORT_NAME, GerritSshdService(ns))

	// Ensure configMaps for managesf-resources
	cm_data := make(map[string]string)
	cm_data["config.py"] = GenerateManageSFConfig(string(adminApiKey), fqdn)
	g.ensureCM(MANAGESF_RESOURCES_IDENT, cm_data)
	tooling_data := make(map[string]string)
	tooling_data["create-repo.sh"] = CreateRepoScript
	tooling_data["create-ci-user.sh"] = CreateCIUserScript
	g.ensureCM(MANAGESF_RESOURCES_IDENT+"-tooling", tooling_data)

	// Ensure gerrit statefulset
	g.ensureGerritSTS()

	// Wait for Gerrit statefullSet ready
	for !g.isSTSReady(GERRIT_IDENT) {
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
		&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: GERRIT_HTTPD_PORT_NAME, Namespace: ns}})
	cl.Delete(ctx,
		&apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: GERRIT_SSHD_PORT_NAME, Namespace: ns}})

	// Delete Gerrit STS and the associated Statefulset
	cl.Delete(ctx,
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: GERRIT_IDENT, Namespace: ns}})
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
		&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: MANAGESF_RESOURCES_IDENT + "-config-map", Namespace: ns}})
	cl.Delete(ctx,
		&apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: MANAGESF_RESOURCES_IDENT + "-tooling-config-map", Namespace: ns}})

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
