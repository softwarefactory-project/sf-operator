// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

type SFController struct {
	SFKubeContext
	cr            sfv1.SoftwareFactory
	configBaseURL string
	needOpendev   bool
}

func messageGenerator(isReady bool, goodmsg string, badmsg string) string {
	if isReady {
		return color.GreenString(goodmsg)
	}
	return color.RedString(badmsg)
}

func messageInfo(services map[string]bool) string {
	msg := ""
	servicesSorted := []string{}
	for servicename := range services {
		servicesSorted = append(servicesSorted, servicename)
	}

	sort.Strings(servicesSorted)

	for _, servicename := range servicesSorted {
		statusmsg := messageGenerator(services[servicename], "OK\n", "Waiting ...\n")
		msg = msg + fmt.Sprintf("\t - %s: %s", color.CyanString(servicename), statusmsg)
	}

	if msg != "" {
		msg = "\n" + msg
	}

	return msg
}

func isOperatorReady(services map[string]bool) bool {
	for _, value := range services {
		if !value {
			return false
		}
	}
	return true
}

// cleanup ensures removal of legacy resources
func (r *SFController) cleanup() {

	logging.LogI("Nothing to clean up.")

	// sanity check: if zookeeper certs are missing, the services must be terminated to avoid them from not responding to sigterm.
	// First, ZK is flooded with `io.netty.handler.codec.DecoderException: javax.net.ssl.SSLHandshakeException: Insufficient buffer remaining for AEAD cipher fragment (2). Needs to be more than tag size (16)`
	// Then, python-kazoo is stuck and zuul services are not responding to sigterm
	zkTLS := corev1.Secret{}
	if r.cr.Spec.Zuul.Executor.Standalone == nil && !r.GetOrDie("zookeeper-client-tls", &zkTLS) {
		r.nukeZKClients()
	}

	r.DeleteSecret("ca-cert")
}

// Manually kill all the ZK process in last resort
func (r *SFKubeContext) nukeZKClients() {
	podslist, _ := r.ClientSet.CoreV1().Pods(r.Ns).List(r.Ctx, metav1.ListOptions{})
	for _, pod := range podslist.Items {
		if strings.HasPrefix(pod.Name, "zuul-") || strings.HasPrefix(pod.Name, "nodepool-") {
			// Get the service name from the first container
			cName := pod.Spec.Containers[0].Name
			logging.LogW("Killing ZooKeeper client: " + pod.Name + " " + cName)

			// Ensure the process are killed
			r.PodExec(pod.Name, cName, []string{"kill", "-9", "1"})
			if cName == "zuul-web" || cName == "zuul-weeder" || cName == "nodepool-launcher" {
				r.DeleteR(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: cName, Namespace: r.Ns}})
			} else {
				r.DeleteR(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: cName, Namespace: r.Ns}})
			}
		}
	}
	// Delete zookeeper at the end
	r.DeleteR(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "zookeeper", Namespace: r.Ns}})
}

func (r *SFController) validateZuulConnectionsSecrets() error {
	// Validate github secrets
	for _, connection := range r.cr.Spec.Zuul.GitHubConns {
		secret, err := r.GetSecret(connection.Secrets)
		if err != nil {
			return errors.New("Missing github secret: " + connection.Secrets)
		}
		if connection.AppID > 0 && len(secret.Data["app_key"]) == 0 {
			return errors.New("Missing github app_key field in: " + connection.Secrets)
		}
	}

	// Validate gerrit secrets
	for _, conn := range r.cr.Spec.Zuul.GerritConns {
		if conn.Sshkey != "" {
			_, err := r.GetSecretDataFromKey(conn.Sshkey, "priv")
			if err != nil {
				return errors.New("Missing gerrit secret: " + conn.Sshkey)
			}
		}
	}
	return nil
}

func ensureTrailingSlash(url string) string {
	if len(url) > 0 && url[len(url)-1:] != "/" {
		return url + "/"
	}
	return url
}

func resolveConfigBaseURL(cr sfv1.SoftwareFactory) string {
	name := cr.Spec.ConfigRepositoryLocation.ZuulConnectionName
	url := ""
	for _, conn := range cr.Spec.Zuul.GerritConns {
		if conn.Name == name {
			if conn.Puburl != "" {
				url = conn.Puburl
			} else {
				url = fmt.Sprintf("https://%s/", conn.Hostname)
			}
			return ensureTrailingSlash(url)
		}
	}
	for _, conn := range cr.Spec.Zuul.GitHubConns {
		if conn.Name == name {
			if conn.Server == "" {
				url = "https://github.com/"
			} else {
				url = fmt.Sprintf("https://%s/", conn.Server)
			}
			return ensureTrailingSlash(url)
		}
	}
	for _, conn := range cr.Spec.Zuul.GitLabConns {
		if conn.Name == name {
			if conn.BaseURL != "" {
				url = conn.BaseURL
			} else {
				url = fmt.Sprintf("https://%s/", conn.Server)
			}
			return ensureTrailingSlash(url)
		}
	}
	for _, conn := range cr.Spec.Zuul.GitConns {
		if conn.Name == name {
			return ensureTrailingSlash(conn.Baseurl)
		}
	}
	for _, conn := range cr.Spec.Zuul.PagureConns {
		if conn.Name == name {
			if conn.BaseURL != "" {
				url = conn.BaseURL
			} else {
				url = fmt.Sprintf("https://%s/", conn.Server)
			}
			return ensureTrailingSlash(url)
		}
	}
	return ""
}

func GetUserDefinedConnections(zuul *sfv1.ZuulSpec) ([]string, error) {
	var conns []string
	for _, conn := range zuul.GerritConns {
		if conn.Name == "opendev.org" && conn.Hostname != "review.opendev.org" {
			return conns, errors.New("opendev.org gerrit connection must be for review.opendev.org")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.GitHubConns {
		if conn.Name == "opendev.org" {
			return conns, errors.New("opendev.org must be a gerrit or git connection")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.GitLabConns {
		if conn.Name == "opendev.org" {
			return conns, errors.New("opendev.org must be a gerrit or git connection")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.GitConns {
		if conn.Name == "opendev.org" && conn.Baseurl != "https://opendev.org" {
			return conns, errors.New("opendev.org git connection must be for https://opendev.org")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.PagureConns {
		if conn.Name == "opendev.org" {
			return conns, errors.New("opendev.org must be a gerrit or git connection")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.ElasticSearchConns {
		if conn.Name == "opendev.org" {
			return conns, errors.New("opendev.org must be a gerrit or git connection")
		}
		conns = append(conns, conn.Name)
	}
	for _, conn := range zuul.SMTPConns {
		if conn.Name == "opendev.org" {
			return conns, errors.New("opendev.org must be a gerrit or git connection")
		}
		conns = append(conns, conn.Name)
	}
	return conns, nil
}

func (r *SFController) IsCodesearchEnabled() bool {
	return r.cr.Spec.Codesearch.Enabled == nil || *r.cr.Spec.Codesearch.Enabled
}

func (r *SFController) EnsureToolingVolume() {
	schedulerToolingData := make(map[string]string)
	schedulerToolingData["init-container.sh"] = zuulSchedulerInitContainerScript
	schedulerToolingData["generate-zuul-tenant-yaml.sh"] = zuulGenerateTenantConfig
	schedulerToolingData["reconnect-zk.py"] = zuulReconnectZK
	schedulerToolingData["rotate-keystore.py"] = zuulRotateKeystore
	schedulerToolingData["fetch-config-repo.sh"] = fetchConfigRepoScript
	schedulerToolingData["hound-search-init.sh"] = houndSearchInit
	schedulerToolingData["hound-search-config.sh"] = houndSearchConfig
	schedulerToolingData["hound-search-render.py"] = houndSearchRender
	schedulerToolingData["zuul-change-dump.py"], _ = utils.ParseString(zuulChangeDump, struct {
		ZuulWebURL string
	}{ZuulWebURL: "https://" + r.cr.Spec.FQDN + "/zuul"})

	r.EnsureConfigMap("zuul-scheduler-tooling", schedulerToolingData)
}

func (r *SFController) deployStandaloneExectorStep(services map[string]bool) map[string]bool {
	services["Zuul"] = false

	// Notes - required resources
	// Secret: ca-cert, zookeeper-client-tls, zuul-ssh-key, zuul-keystore-password
	// Zuul' connections secrets

	// Validate the Secrets are available
	for _, sn := range []string{ZuulKeystorePasswordName, "zuul-ssh-key", "zookeeper-client-tls"} {
		_, err := r.GetSecret(sn)
		if err != nil {
			logging.LogE(err, "Unable to find the Secret named "+sn)
			return services
		}
	}

	// Setup zuul.conf Secret
	cfg := r.EnsureZuulConfigSecret(true)
	if cfg == nil {
		return services
	}

	// Install the Service Resource
	r.EnsureZuulExecutorService()

	// Run the StatefullSet deployment
	services["Zuul"] = r.EnsureZuulExecutor(cfg)

	return services
}

func (r *SFController) deployZKAndZuulAndNodepool(services map[string]bool) map[string]bool {
	// 1. Ensure Zookeeper is reconciled
	// ---------------------------------
	// The Zookeeper service is needed by Zuul and Nodepool to synchronize
	services["Zookeeper"] = r.DeployZookeeper()
	if !services["Zookeeper"] {
		logging.LogI("Waiting for Zookeeper service to be ready ...")
		return services
	}

	// 2. Handle Zuul and Nodepool deployment if Zookeeper is up and running
	// ---------------------------------------------------------------------
	if !services["GitServer"] || !services["MariaDB"] {
		logging.LogI("Waiting for GitServer and MariaDB services to be ready ...")
		return services
	}
	logging.LogI("Deploying Zuul and Nodepool ...")
	nodepool := r.DeployNodepool()
	services["NodePoolLauncher"] = nodepool[LauncherIdent]
	services["NodePoolBuilder"] = nodepool[BuilderIdent]
	zuulComponentsStatus := r.DeployZuul()
	services["Zuul"] = zuulComponentsStatus["Zuul"]
	if !services["Zuul"] {
		for cmp := range maps.Keys(zuulComponentsStatus) {
			services[cmp] = zuulComponentsStatus[cmp]
		}
	}
	return services
}

func (r *SFController) deploySFStep(services map[string]bool) map[string]bool {
	// 1. Ensure some content resources
	// --------------------------------
	// Ensure SF Admin ssh key pair
	r.DeployZuulSecrets()
	// Setup custom tools used by zuul and code-search
	r.EnsureToolingVolume()

	// 2. Deploy backing and companion services
	// ----------------------------------------
	// The git server service is needed to store system jobs
	services["GitServer"] = r.DeployGitServer()
	// The MariaDB service is needed by Zuul to store build results metadata
	services["MariaDB"] = r.DeployMariadb()
	// The Logserver service is needed by Zuul to store build artifacts
	services["Logserver"] = r.DeployLogserver()
	// The gateway is on redirect incoming HTTP request to backing services
	services["Gateway"] = r.DeployHTTPDGateway()
	// The Hound service provides a codesearch service
	if r.IsCodesearchEnabled() {
		services["HoundSearch"] = r.DeployHoundSearch()
	} else {
		r.TerminateHoundSearch()
	}
	// The Logjuicer is a log analysis service suitable for Zuul
	// TODO: make this configurable
	services["LogJuicer"] = r.EnsureLogJuicer()

	// 3. Deploy Zuul, Nodepool and Zookeeper
	// --------------------------------------
	services = r.deployZKAndZuulAndNodepool(services)

	// 4. Wait for Zuul and LogServer to be up
	// ---------------------------------------
	if !services["Zuul"] || !services["Logserver"] {
		// Force Config status to false to force the main loop to call this function
		services["Config"] = false
		return services
	}

	// 5. Zuul and the LogServer are up and running, then we can ensure that the config jobs are setup
	// -----------------------------------------------------------------------------------------------
	services["Config"] = r.SetupConfigJob()
	if services["Config"] {
		conds.RefreshCondition(&r.cr.Status.Conditions, "ConfigReady", metav1.ConditionTrue, "Ready", "Config is ready")
	}

	return services
}

func (r *SFController) Step() sfv1.SoftwareFactoryStatus {

	r.cleanup()

	if err := r.validateZuulConnectionsSecrets(); err != nil {
		logging.LogE(err, "Validation of Zuul connections secrets failed")
		// TODO: add error as a new status conditions
		status := r.cr.Status.DeepCopy()
		status.Ready = false
		return *status
	}

	services := map[string]bool{}

	if r.cr.Spec.Zuul.Executor.Standalone != nil {
		services = r.deployStandaloneExectorStep(services)
	} else {
		services = r.deploySFStep(services)
	}

	logging.LogI(messageInfo(services))

	isReady := isOperatorReady(services)

	return sfv1.SoftwareFactoryStatus{
		Ready:              isReady,
		ObservedGeneration: r.cr.Generation,
		ReconciledBy:       conds.GetOperatorConditionName(),
		Conditions:         r.cr.Status.Conditions,
	}
}

func (r *SFController) DebugService(debugService string) {
	fmt.Printf("Debugging service: %#v\n", debugService)
	if debugService == "zuul-executor" {
		r.DebugStatefulSet(debugService)
	} else {
		panic("Unknown service")
	}
}

func HasDuplicate(conns []string) string {
	for i, conn := range conns {
		if slices.Contains(conns[i+1:], conn) {
			return conn
		}
	}
	return ""
}

func MkSFController(r SFKubeContext, cr sfv1.SoftwareFactory) SFController {
	conns, err := GetUserDefinedConnections(&cr.Spec.Zuul)
	if err != nil {
		ctrl.Log.Error(err, "Invalid Zuul connections")
		os.Exit(1)
	}
	if dup := HasDuplicate(conns); dup != "" {
		fmt.Fprintf(os.Stderr, "Duplicate zuul connection: %s", dup)
		os.Exit(1)
	}
	if slices.Contains(conns, "git-server") {
		fmt.Fprintf(os.Stderr, "The git-server connection name is reserved, please rename it")
		os.Exit(1)
	}
	return SFController{
		SFKubeContext: r,
		cr:            cr,
		configBaseURL: resolveConfigBaseURL(cr),
		needOpendev:   !slices.Contains(conns, "opendev.org"),
	}
}

var controllerCMName = "sf-standalone-owner"

func (r *SFKubeContext) GetStandaloneOwner() bool {
	var cm corev1.ConfigMap
	if r.GetOrDie(controllerCMName, &cm) {
		r.Owner = &cm
		return true
	} else {
		// Note that Owner is an interface, and we can't assign nil here.
		return false
	}
}

func (r *SFKubeContext) EnsureStandaloneOwner(spec sfv1.SoftwareFactorySpec) {
	// Create a fake resource that simulate the Resource Owner.
	// A deletion to that resource Owner will cascade delete owned resources
	if !r.GetStandaloneOwner() {
		controllerCM := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      controllerCMName,
				Namespace: r.Ns,
			}}
		marshaledSpec, _ := yaml.Marshal(spec)
		controllerCM.Data = map[string]string{
			"spec": string(marshaledSpec),
		}
		// We can't use CreateR here because it requires an Owner.
		if err := r.Client.Create(r.Ctx, &controllerCM); err != nil {
			ctrl.Log.Error(err, "Unable to create configMap", "name", controllerCMName)
			os.Exit(1)
		}
		r.Owner = &controllerCM
	}
}

func (r *SFKubeContext) StandaloneReconcile(sf sfv1.SoftwareFactory) error {
	d, _ := time.ParseDuration("5s")
	maxAttempt := 60
	log := log.FromContext(r.Ctx)
	controllerAnnotations := map[string]string{
		"sf-operator-version": utils.GetVersion(),
		"last-reconcile":      strconv.FormatInt(time.Now().Unix(), 10),
	}
	r.EnsureStandaloneOwner(sf.Spec)
	sfCtrl := MkSFController(*r, sf)
	attempt := 0

	for {
		status := sfCtrl.Step()
		if r.DryRun {
			log.Info("[Dry Run] Standalone reconcile done")
			return nil
		}
		attempt += 1
		if attempt == maxAttempt {
			return errors.New("unable to reconcile after max attempts")
		}
		if status.Ready {
			log.Info("Updating controller configmap ...")
			marshaledSpec, _ := yaml.Marshal(sf.Spec)
			var controllerCM corev1.ConfigMap
			if r.GetOrDie(controllerCMName, &controllerCM) {
				controllerCM.Data = map[string]string{
					"spec": string(marshaledSpec),
				}
				controllerCM.ObjectMeta.Annotations = controllerAnnotations
				if err := r.Client.Update(r.Ctx, &controllerCM); err != nil {
					log.Error(err, "Unable to update configMap", "name", controllerCMName)
					return err
				}
				log.Info("Standalone reconcile done.")
			} else {
				log.Error(errors.New(controllerCMName+" not found"), "Controller configmap not found")
			}
			return nil
		}
		log.Info("[attempt #" + strconv.Itoa(attempt) + "] Waiting 5s for the next reconcile call ...")
		time.Sleep(d)
	}
}
