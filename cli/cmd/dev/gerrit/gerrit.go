/*
Copyright © 2024 Red Hat

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

// Package gerrit provides gerrit related functions for the CLI
package gerrit

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "embed"

	apiroutev1 "github.com/openshift/api/route/v1"
	v1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	cutils "github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
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
	env  *cliutils.ENV
	fqdn string
}

func createAPIKeySecret(name string, ns string) apiv1.Secret {
	return base.MkSecretFromFunc(name, ns, cutils.NewUUIDString)
}

func (g *GerritCMDContext) ensureSecretOrDie(
	name string, secretGenerator func(string, string) apiv1.Secret) apiv1.Secret {
	secret := apiv1.Secret{}

	if !cliutils.GetMOrDie(g.env, name, &secret) {
		secret = secretGenerator(name, g.env.Ns)
		cliutils.CreateROrDie(g.env, &secret)
	}
	return secret
}

func (g *GerritCMDContext) ensureServiceOrDie(service apiv1.Service) {
	var serv apiv1.Service
	if !cliutils.GetMOrDie(g.env, service.Name, &serv) {
		cliutils.CreateROrDie(g.env, &service)
	}
}

func (g *GerritCMDContext) ensureJobCompletedOrDie(job batchv1.Job) {
	var curJob batchv1.Job
	if !cliutils.GetMOrDie(g.env, job.Name, &curJob) {
		cliutils.CreateROrDie(g.env, &job)
	}
	for i := 0; i < 60; i++ {
		if curJob.Status.Succeeded >= 1 {
			return
		}
		time.Sleep(2 * time.Second)
		// refresh curJob
		cliutils.GetMOrDie(g.env, job.Name, &curJob)
	}
	ctrl.Log.Error(errors.New("timeout reached"), "Error waiting for job '"+job.Name+"' to complete")
	os.Exit(1)
}

func (g *GerritCMDContext) ensureRouteOrDie(route apiroutev1.Route) {
	var rte apiroutev1.Route
	if !cliutils.GetMOrDie(g.env, route.Name, &rte) {
		cliutils.CreateROrDie(g.env, &route)
	}
}

func (g *GerritCMDContext) ensureConfigMapOrDie(name string, data map[string]string) {
	cmName := name + "-config-map"
	var cm apiv1.ConfigMap
	if !cliutils.GetMOrDie(g.env, cmName, &cm) {
		ctrl.Log.Info(name)
		cm = apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: g.env.Ns},
			Data:       data,
		}
		cliutils.CreateROrDie(g.env, &cm)
	}
}

func gerritHttpdService(ns string) apiv1.Service {
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

func gerritSshdService(ns string) apiv1.Service {
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
			Type: apiv1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app": "sf",
				"run": gerritIdent,
			},
		}}
}

func generateManageSFConfig(adminPassword string, fqdn string) string {

	// Structure for config.py file template
	type ConfigPy struct {
		Fqdn                string
		GerritAdminPassword string
	}

	// Initializing Template Structure
	configpy := ConfigPy{
		fqdn,
		adminPassword,
	}

	template, err := cutils.ParseString(managesfConf, configpy)
	if err != nil {
		ctrl.Log.Error(err, "Failure applying manageSF configuration template")
		os.Exit(1)
	}

	return template
}

var ManageSFVolumes = []apiv1.Volume{
	base.MkVolumeCM(managesfResourcesIdent+"-config-vol",
		managesfResourcesIdent+"-config-map"),
	{
		Name: managesfResourcesIdent + "-tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: managesfResourcesIdent + "-tooling-config-map",
				},
				DefaultMode: &cutils.Execmod,
			},
		},
	},
}

func configureGerritContainer(sts *appsv1.StatefulSet, volumeMounts []apiv1.VolumeMount, fqdn string, hostAliases []v1.HostAlias) {
	sts.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", entrypoint}
	sts.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	sts.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		base.MkContainerPort(gerritHTTPDPort, gerritHTTPDPortName),
		base.MkContainerPort(gerritSSHDPort, gerritSSHDPortName),
	}
	sts.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/gerrit"),
		base.MkEnvVar("FQDN", fqdn),
		base.MkEnvVar("JVM_XMS", "128m"),
		base.MkEnvVar("JVM_XMX", "512m"),
		base.MkSecretEnvVar("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
	}
	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessCMDProbe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupCMDProbe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLivenessCMDProbe([]string{"bash", "/gerrit/ready.sh"})
	sts.Spec.Template.Spec.HostAliases = base.CreateHostAliases(hostAliases)
}

func addManageSFContainer(sts *appsv1.StatefulSet, fqdn string) {
	container := base.MkContainer(managesfResourcesIdent, base.BusyboxImage())
	container.Command = []string{"sh", "-c", managesfEntrypoint}
	container.Env = []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/tmp"),
		base.MkEnvVar("FQDN", fqdn),
		// managesf-resources need an admin ssh access to the local Gerrit
		base.MkSecretEnvVar("SF_ADMIN_SSH", "admin-ssh-key", "priv"),
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

func addManageSFVolumes(sts *appsv1.StatefulSet) {
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
		ManageSFVolumes...,
	)
}

func createPostInitContainer(jobName string, fqdn string) apiv1.Container {
	env := []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/tmp"),
		base.MkEnvVar("FQDN", fqdn),
		base.MkSecretEnvVar("GERRIT_ADMIN_SSH", "admin-ssh-key", "priv"),
		base.MkSecretEnvVar("GERRIT_ADMIN_API_KEY", "gerrit-admin-api-key", "gerrit-admin-api-key"),
		base.MkSecretEnvVar("ZUUL_SSH_PUB_KEY", "zuul-ssh-key", "pub"),
		base.MkSecretEnvVar("ZUUL_HTTP_PASSWORD", "zuul-gerrit-api-key", "zuul-gerrit-api-key"),
	}

	container := base.MkContainer(fmt.Sprintf("%s-container", jobName), base.BusyboxImage())
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

func createInitContainers(volumeMounts []apiv1.VolumeMount, fqdn string) []apiv1.Container {
	container := base.MkContainer("gerrit-init", gerritImage)
	container.Command = []string{"sh", "-c", gerritInitScript}
	container.Env = []apiv1.EnvVar{
		base.MkSecretEnvVar("GERRIT_ADMIN_SSH_PUB", "admin-ssh-key", "pub"),
		base.MkEnvVar("FQDN", fqdn),
		base.MkEnvVar("JVM_XMS", "256m"),
		base.MkEnvVar("JVM_XMX", "512m"),
	}
	container.VolumeMounts = volumeMounts
	base.SetContainerLimits(
		&container,
		resource.MustParse("512Mi"),
		resource.MustParse("768Mi"),
		resource.MustParse("100m"),
		resource.MustParse("1000m"))
	return []apiv1.Container{
		container,
	}
}

func (g *GerritCMDContext) ensureGerritPostInitJobOrDie() {
	jobName := "post-init"
	job := base.MkJob(
		jobName, g.env.Ns,
		createPostInitContainer(jobName, g.fqdn),
		map[string]string{},
	)
	job.Spec.Template.Spec.Volumes = ManageSFVolumes
	g.ensureJobCompletedOrDie(job)
}

func (g *GerritCMDContext) getStatefulSetOrDie(name string) (bool, appsv1.StatefulSet) {
	sts := appsv1.StatefulSet{}
	b := cliutils.GetMOrDie(g.env, name, &sts)
	return b, sts
}

func (g *GerritCMDContext) isStatefulSetReady(name string) bool {
	b, sts := g.getStatefulSetOrDie(name)
	return b && base.IsStatefulSetRolloutDone(&sts)
}

func (g *GerritCMDContext) ensureStatefulSetOrDie(hostAliases []v1.HostAlias) {
	name := gerritIdent
	b, _ := g.getStatefulSetOrDie(name)
	if !b {
		container := base.MkContainer(name, gerritImage)
		base.SetContainerLimits(
			&container,
			resource.MustParse("512Mi"),
			resource.MustParse("768Mi"),
			resource.MustParse("100m"),
			resource.MustParse("1000m"))
		storageConfig := controllers.BaseGetStorageConfOrDefault(v1.StorageSpec{}, v1.StorageDefaultSpec{})
		pvc := base.MkPVC(name, g.env.Ns, storageConfig, apiv1.ReadWriteOnce)
		sts := base.MkStatefulset(
			name, g.env.Ns, 1, name, container, pvc, map[string]string{})
		volumeMounts := []apiv1.VolumeMount{
			{
				Name:      name,
				MountPath: gerritSiteMountPath,
			},
		}
		configureGerritContainer(&sts, volumeMounts, g.fqdn, hostAliases)
		sts.Spec.Template.Spec.InitContainers = createInitContainers(volumeMounts, g.fqdn)

		addManageSFContainer(&sts, g.fqdn)
		addManageSFVolumes(&sts)

		cliutils.CreateROrDie(g.env, &sts)
	}
}

func (g *GerritCMDContext) ensureGerritIngressesOrDie() {
	name := "gerrit"
	route := cliutils.MkHTTPSRoute(name, g.env.Ns, name+"."+g.fqdn,
		gerritHTTPDPortName, "/", gerritHTTPDPort, map[string]string{})
	g.ensureRouteOrDie(route)
}

func ensureNamespaceOrDie(env *cliutils.ENV) {
	var namespace apiv1.Namespace
	err := env.Cli.Get(env.Ctx, client.ObjectKey{Name: env.Ns}, &namespace)
	if err != nil && apierrors.IsNotFound(err) {
		nsR := apiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: env.Ns},
		}
		err = env.Cli.Create(env.Ctx, &nsR)
		if err != nil {
			ctrl.Log.Error(err, "Failed to create namespace '"+env.Ns+"'")
			os.Exit(1)
		} else {
			ctrl.Log.Info("created namespace '" + env.Ns + "'")
		}
	}
}

func EnsureGerrit(env *cliutils.ENV, fqdn string, hostAliases []v1.HostAlias) {
	ensureNamespaceOrDie(env)

	g := GerritCMDContext{
		env:  env,
		fqdn: fqdn,
	}

	// Ensure the admin SSH key pair secret
	g.ensureSecretOrDie("admin-ssh-key", base.MkSSHKeySecret)

	// Ensure the zuul SSH key pair secret
	g.ensureSecretOrDie("zuul-ssh-key", base.MkSSHKeySecret)

	// Ensure the admin API key secret
	adminAPIKeyName := "gerrit-admin-api-key"
	adminAPIKeySecret := g.ensureSecretOrDie(adminAPIKeyName, createAPIKeySecret)
	adminAPIKey, _ := controllers.GetValueFromKeySecret(adminAPIKeySecret, adminAPIKeyName)

	// Ensure the zuul API key secret
	g.ensureSecretOrDie("zuul-gerrit-api-key", createAPIKeySecret)

	// Ensure httpd Service
	g.ensureServiceOrDie(gerritHttpdService(g.env.Ns))

	// Ensure sshd Service
	g.ensureServiceOrDie(gerritSshdService(g.env.Ns))

	// Ensure configMaps for managesf-resources
	cmData := make(map[string]string)
	cmData["config.py"] = generateManageSFConfig(string(adminAPIKey), fqdn)
	g.ensureConfigMapOrDie(managesfResourcesIdent, cmData)
	toolingData := make(map[string]string)
	toolingData["create-repo.sh"] = CreateRepoScript
	toolingData["create-ci-user.sh"] = CreateCIUserScript
	g.ensureConfigMapOrDie(managesfResourcesIdent+"-tooling", toolingData)

	// Ensure gerrit statefulset
	g.ensureStatefulSetOrDie(hostAliases)

	// Wait for Gerrit statefulset to be ready
	for !g.isStatefulSetReady(gerritIdent) {
		ctrl.Log.Info("Waiting 10s for gerrit statefulset to be ready...")
		time.Sleep(10 * time.Second)
	}

	// Start Post Init Job
	g.ensureGerritPostInitJobOrDie()

	// Ensure the Ingress route
	g.ensureGerritIngressesOrDie()
}

func WipeGerrit(env *cliutils.ENV, rmData bool) {
	ns := env.Ns
	// Delete route
	cliutils.DeleteOrDie(env, &apiroutev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "gerrit", Namespace: ns}})
	// Delete secrets
	for _, secret := range []string{
		"admin-ssh-key",
		"zuul-ssh-key",
		"gerrit-admin-api-key",
		"zuul-gerrit-api-key",
	} {
		cliutils.DeleteOrDie(env, &apiv1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: ns}})
	}
	// Delete services
	for _, srv := range []string{
		gerritHTTPDPortName,
		gerritSSHDPortName,
	} {
		cliutils.DeleteOrDie(env, &apiv1.Service{ObjectMeta: metav1.ObjectMeta{Name: srv, Namespace: ns}})
	}
	// Delete config maps
	for _, cm := range []string{
		managesfResourcesIdent + "-config-map",
		managesfResourcesIdent + "-tooling-config-map",
	} {
		cliutils.DeleteOrDie(env, &apiv1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cm, Namespace: ns}})
	}
	// Delete statefulset
	cliutils.DeleteOrDie(env, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: gerritIdent, Namespace: ns}})

	// Delete post init job
	backgroundDeletion := metav1.DeletePropagationBackground
	cliutils.DeleteOrDie(env,
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "post-init", Namespace: ns}},
		&client.DeleteOptions{
			PropagationPolicy: &backgroundDeletion,
		})

	// Delete persistent volume for full wipe
	if rmData {
		cliutils.DeleteOrDie(env, &apiv1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "gerrit-gerrit-0", Namespace: ns}})
	}
}

func GetAdminRepoURL(env *cliutils.ENV, fqdn string, repoName string) string {
	var (
		gerritAPIKey apiv1.Secret
	)
	if !cliutils.GetMOrDie(env, "gerrit-admin-api-key", &gerritAPIKey) {
		ctrl.Log.Error(errors.New("secret 'gerrit-admin-api-key' does not exist"), "Cannot clone repo as admin")
		os.Exit(1)
	}
	apiKey := string(gerritAPIKey.Data["gerrit-admin-api-key"])
	ctrl.Log.V(5).Info("API Key: " + apiKey)
	repoURL := fmt.Sprintf("https://admin:%s@gerrit.%s/a/%s", apiKey, fqdn, repoName)
	return repoURL
}

func CloneAsAdmin(env *cliutils.ENV, fqdn string, repoName string, dest string, verify bool) {
	var (
		output string
	)
	repoURL := GetAdminRepoURL(env, fqdn, repoName)
	if _, err := os.Stat(filepath.Join(dest, ".git")); os.IsNotExist(err) {
		ctrl.Log.Info("Cloning repo " + repoURL + " in " + dest)
		args := []string{}
		if !verify {
			args = append(args, "-c", "http.sslVerify=false")
		}
		args = append(args, "clone", repoURL, dest)
		output = cliutils.RunCmdOrDie("git", args...)
		ctrl.Log.V(5).Info("captured output:\n" + output)
		output = cliutils.RunCmdOrDie("git", "-C", dest, "remote", "add", "gerrit", repoURL)
		ctrl.Log.V(5).Info("captured output:\n" + output)
	} else {
		ctrl.Log.Info("Repository exists. Resetting remotes...")
		for _, o := range []string{
			cliutils.RunCmdOrDie("git", "-C", dest, "remote", "set-url", "origin", repoURL),
			cliutils.RunCmdOrDie("git", "-C", dest, "remote", "set-url", "gerrit", repoURL),
			cliutils.RunCmdOrDie("git", "-C", dest, "fetch", "origin"),
		} {
			if o != "" {
				ctrl.Log.V(5).Info("captured output:\n" + o)
			}
		}
	}
	ctrl.Log.Info("Configuring local repository for commits...")
	for _, _args := range [][]string{
		{
			"-C", dest, "config", "user.email", "admin@" + fqdn,
		},
		{
			"-C", dest, "config", "user.name", "admin",
		},
		{
			"-C", dest, "reset", "--hard", "origin/master",
		},
	} {
		output = cliutils.RunCmdOrDie("git", _args...)
		if output != "" {
			ctrl.Log.V(5).Info("captured output:\n" + output)
		}
	}
	if !verify {
		output = cliutils.RunCmdOrDie("git",
			"-C", dest, "config", "http.sslverify", "false")
		if output != "" {
			ctrl.Log.V(5).Info("captured output:\n" + output)
		}
	}
}

func EnsureGerritAccess(fqdn string) {
	attempt := 1
	maxTries := 10
	delay := 6 * time.Second
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	var (
		resp      *http.Response
		err       error
		bodyBytes []byte
	)
	for {
		if attempt > maxTries {
			endpointError := errors.New("endpoint failure")
			ctrl.Log.Error(endpointError, "Could not reach gerrit after "+strconv.Itoa(maxTries)+" tries")
			defer resp.Body.Close()
			bodyBytes, err = io.ReadAll(resp.Body)
			if err != nil {
				ctrl.Log.Error(err, "Error reading Gerrit response")
			} else {
				ctrl.Log.Error(endpointError, fmt.Sprintf("Last status:%d - Last response body:\"%s\"", resp.StatusCode, string(bodyBytes)))
			}
			os.Exit(1)
		}
		url := fmt.Sprintf("https://gerrit.%s/projects/", fqdn)
		ctrl.Log.Info(fmt.Sprintf("Querying Gerrit projects endpoint... [attempt %d/%d]", attempt, maxTries))
		resp, err := client.Get(url)
		if err != nil {
			ctrl.Log.Error(err, "Redirect failure or HTTP protocol error")
			os.Exit(1)
		}
		if resp.StatusCode < 400 {
			ctrl.Log.Info("Gerrit is up and available")
			break
		}
		attempt += 1
		time.Sleep(delay)
	}
}
