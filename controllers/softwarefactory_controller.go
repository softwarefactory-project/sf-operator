// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gopkg.in/yaml.v3"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/strings/slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/cert"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	discovery "k8s.io/client-go/discovery"
)

type SoftwareFactoryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
	CancelFunc context.CancelFunc
	Completed  bool
	DryRun     bool
}

// Run `make manifests` to apply rbac change
//
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=jobs;pods;pods/exec;services;statefulsets;deployments;configmaps;secrets;persistentvolumeclaims;serviceaccounts;roles;rolebindings;storageclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=jobs/status;pods/status;services/status;statefulsets/status;deployments/status;configmaps/status;secrets/status;persistentvolumeclaims/status;serviceaccounts/status;roles/status,verbs=get
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

type SFController struct {
	SFKubeContext
	cr            sfv1.SoftwareFactory
	ZkChanged     bool
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
	if r.cr.Spec.Zuul.Executor.Standalone == nil && !r.GetM("zookeeper-client-tls", &zkTLS) {
		r.nukeZKClients()
	}
}

// Manually kill all the ZK process in last resort
func (r *SFController) nukeZKClients() {
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
	for _, sn := range []string{
		ZuulKeystorePasswordName, cert.LocalCACertSecretName,
		"zuul-ssh-key", "zookeeper-client-tls"} {
		_, err := r.GetSecret(sn)
		if err != nil {
			logging.LogE(err, "Unable to find the Secret named "+sn)
			return services
		}
	}

	// Setup zuul.conf Secret
	cfg := r.EnsureZuulConfigSecret(true, true)
	if cfg == nil {
		return services
	}

	// Install the Service Resource
	r.EnsureZuulExecutorService()

	// Run the StatefullSet deployment
	services["Zuul"] = r.EnsureZuulExecutor(cfg)

	return services
}

func (r *SFController) setupMonitoring() ([]string, []string) {
	monitoredPorts := []string{}
	selectorRunList := []string{}
	monitoredPorts = append(monitoredPorts,
		sfmonitoring.GetTruncatedPortName(GitServerIdent, sfmonitoring.NodeExporterPortNameSuffix),
		sfmonitoring.GetTruncatedPortName(MariaDBIdent, sfmonitoring.NodeExporterPortNameSuffix),
		sfmonitoring.GetTruncatedPortName(ZookeeperIdent, sfmonitoring.NodeExporterPortNameSuffix),
		sfmonitoring.GetTruncatedPortName(BuilderIdent, sfmonitoring.NodeExporterPortNameSuffix),
		NodepoolStatsdExporterPortName,
		sfmonitoring.GetTruncatedPortName("zuul-scheduler", sfmonitoring.NodeExporterPortNameSuffix),
		sfmonitoring.GetTruncatedPortName("zuul-merger", sfmonitoring.NodeExporterPortNameSuffix),
		sfmonitoring.GetTruncatedPortName("zuul-web", sfmonitoring.NodeExporterPortNameSuffix),
		ZuulPrometheusPortName,
		ZuulStatsdExporterPortName,
	)

	selectorRunList = append(selectorRunList, LauncherIdent, BuilderIdent, "zuul-scheduler", "zuul-merger", "zuul-web", GitServerIdent, MariaDBIdent, ZookeeperIdent)

	if r.IsExecutorEnabled() {
		monitoredPorts = append(monitoredPorts,
			sfmonitoring.GetTruncatedPortName("zuul-executor", sfmonitoring.NodeExporterPortNameSuffix))
		selectorRunList = append(selectorRunList, "zuul-executor")
	}

	return monitoredPorts, selectorRunList
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
	// Setup the Certificate Authority for Zookeeper/Zuul/Nodepool usage
	r.EnsureZookeeperCertificates(ZookeeperIdent, ZookeeperReplicas)
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

	// TODO? we could add this to the readiness computation.
	if !r.cr.Spec.PrometheusMonitorsDisabled && isReady {
		DURuleGroups := []monitoringv1.RuleGroup{
			sfmonitoring.MkDiskUsageRuleGroup(r.Ns, "sf"),
		}
		monitoredPorts, selectorRunList := r.setupMonitoring()
		podMonitorSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "sf",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "run",
					Operator: metav1.LabelSelectorOpIn,
					Values:   selectorRunList,
				},
			},
		}
		r.EnsureSFPodMonitor(monitoredPorts, podMonitorSelector)
		r.EnsureDiskUsagePromRule(DURuleGroups)
	}

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

type K8sDist int

const (
	Kubernetes K8sDist = iota
	Openshift
)

func KubernetesDistribution(kubeConfig *rest.Config) K8sDist {

	// Get Config
	if kubeConfig == nil {
		kubeConfig = ctrl.GetConfigOrDie()
	}

	// Create a DiscoveryClient for a given config
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(kubeConfig)

	// Get Api Resources Groups
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "command was not able to find the cluster server groups.\nCheck if the provided kubeconfig file is right.")
		os.Exit(1)
	}

	// Iterate list for config.openshift.io
	apiGroups := apiList.Groups
	for _, element := range apiGroups {
		if element.Name == "route.openshift.io" {
			return Openshift
		}
	}
	return Kubernetes
}

func CheckOpenShift(kubeConfig *rest.Config) bool {

	// Check if environment variable exists
	env := os.Getenv("OPENSHIFT_USER")

	if env != "" {
		openshiftUser, err := strconv.ParseBool(env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "The OPENSHIFT_USER environment variable must be set to true/false, it was set to '%s'\n", env)
			os.Exit(1)
		}
		return openshiftUser
	}

	// Discovering Kubernetes Distribution
	logging.LogI("OPENSHIFT_USER environment variable is not set, discovering Kubernetes Distribution\n")

	var flavour = KubernetesDistribution(kubeConfig)
	switch flavour {
	case Openshift:
		logging.LogI("Kubernetes Distribution found: Openshift\n")
		return true
	default:
		logging.LogI("Kubernetes Distribution found: Kubernetes\n")
		return false
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
		ZkChanged:     false,
		configBaseURL: resolveConfigBaseURL(cr),
		needOpendev:   !slices.Contains(conns, "opendev.org"),
	}
}

func (r *SoftwareFactoryReconciler) mkSFController(
	ctx context.Context, ns string, owner client.Object, cr sfv1.SoftwareFactory,
	standalone bool) SFController {
	clientSet, err := kubernetes.NewForConfig(r.RESTConfig)
	if err != nil {
		ctrl.Log.Error(err, "Invalid client")
		os.Exit(1)
	}
	return MkSFController(
		SFKubeContext{
			Client:     r.Client,
			Scheme:     r.Scheme,
			RESTClient: r.RESTClient,
			RESTConfig: r.RESTConfig,
			ClientSet:  clientSet,
			Ns:         ns,
			Ctx:        ctx,
			Owner:      owner,
			Standalone: standalone,
			DryRun:     r.DryRun,
			// cluster settings
			IsOpenShift:  CheckOpenShift(r.RESTConfig),
			hasProcMount: os.Getenv("HAS_PROC_MOUNT") == "true",
		},
		cr,
	)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SoftwareFactoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if r.Completed {
		// Special case for OneShot mode where we want to prevent re-entering the Step function
		// and get such error: panic: client rate limiter Wait returned an error: context canceled
		return ctrl.Result{}, nil
	}
	log := log.FromContext(ctx)

	log.V(1).Info("SoftwareFactory CR - Entering reconcile loop")

	var sf sfv1.SoftwareFactory
	if err := r.Get(ctx, req.NamespacedName, &sf); err != nil {
		log.Error(err, "unable to fetch SoftwareFactory resource")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sfCtrl := r.mkSFController(ctx, req.Namespace, &sf, sf, false)
	sf.Status = sfCtrl.Step()

	if err := r.Status().Update(ctx, &sf); err != nil {
		log.Error(err, "unable to update Software Factory status")
		return ctrl.Result{}, err
	}
	if !sf.Status.Ready {
		log.V(1).Info("SoftwareFactory CR - Reconcile running...")
		delay, _ := time.ParseDuration("20s")
		return ctrl.Result{RequeueAfter: delay}, nil
	} else {
		log.V(1).Info("SoftwareFactory CR - Reconcile completed!")
		if r.CancelFunc != nil {
			log.V(1).Info("Exiting!")
			r.CancelFunc()
			r.Completed = true
		}
		return ctrl.Result{}, nil
	}

}

var controllerCMName = "sf-standalone-owner"

func EnsureStandaloneOwner(ctx context.Context, cl client.Client, ns string, spec sfv1.SoftwareFactorySpec) (corev1.ConfigMap, error) {
	// Create a fake resource that simulate the Resource Owner.
	// A deletion to that resource Owner will cascade delete owned resources
	controllerCM := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerCMName,
			Namespace: ns,
		}}
	err := cl.Get(
		ctx, client.ObjectKey{Name: controllerCMName, Namespace: ns}, &controllerCM)
	if err != nil && k8s_errors.IsNotFound(err) {
		marshaledSpec, _ := yaml.Marshal(spec)
		controllerCM.Data = map[string]string{
			"spec": string(marshaledSpec),
		}
		logging.LogI("Creating ConfigMap, name: " + controllerCMName)
		// Create the fake controller configMap
		if err := cl.Create(ctx, &controllerCM); err != nil {
			log := log.FromContext(ctx)
			log.Error(err, "Unable to create configMap", "name", controllerCMName)
			return controllerCM, err
		}
	}
	return controllerCM, nil
}

func (r *SFKubeContext) StandaloneReconcile(sf sfv1.SoftwareFactory) error {
	d, _ := time.ParseDuration("5s")
	maxAttempt := 60
	log := log.FromContext(r.Ctx)
	controllerAnnotations := map[string]string{
		"sf-operator-version": utils.GetVersion(),
		"last-reconcile":      strconv.FormatInt(time.Now().Unix(), 10),
	}
	controllerCM, err := EnsureStandaloneOwner(r.Ctx, r.Client, r.Ns, sf.Spec)
	if err != nil {
		return err
	}
	sfCtrl := MkSFController(*r, sf)
	sfCtrl.Owner = &controllerCM
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
			if err := r.Client.Get(
				r.Ctx, client.ObjectKey{Name: controllerCMName, Namespace: r.Ns}, &controllerCM); err == nil {
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
				log.Error(err, "Controller configmap not found")
			}
			return nil
		}
		log.Info("[attempt #" + strconv.Itoa(attempt) + "] Waiting 5s for the next reconcile call ...")
		time.Sleep(d)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SoftwareFactoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mkReconcileRequest := func(softwareFactory sfv1.SoftwareFactory, a client.Object) []reconcile.Request {
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name:      softwareFactory.Name,
				Namespace: a.GetNamespace(),
			}}}

	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.SoftwareFactory{}).
		// Watch only specific Secrets resources
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				softwareFactories := sfv1.SoftwareFactoryList{}
				r.Client.List(ctx, &softwareFactories, &client.ListOptions{
					Namespace: a.GetNamespace(),
				})
				if len(softwareFactories.Items) > 0 {
					// We take the first one of the list
					// sf-operator only manages one SoftwareFactory instance by namespace
					softwareFactory := softwareFactories.Items[0]
					req := mkReconcileRequest(softwareFactory, a)
					switch updatedResourceName := a.GetName(); updatedResourceName {
					case CorporateCACerts:
						return req
					default:
						// All others ConfigMap must not trigger reconcile
						return []reconcile.Request{}
					}
				}
				return []reconcile.Request{}
			}),
		).
		// Watch only specific Secrets resources
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				softwareFactories := sfv1.SoftwareFactoryList{}
				r.Client.List(ctx, &softwareFactories, &client.ListOptions{
					Namespace: a.GetNamespace(),
				})
				if len(softwareFactories.Items) > 0 {
					// We take the first one of the list
					// sf-operator only manages one SoftwareFactory instance by namespace
					softwareFactory := softwareFactories.Items[0]
					req := mkReconcileRequest(softwareFactory, a)
					switch updatedResourceName := a.GetName(); updatedResourceName {
					case NodepoolProvidersSecretsName:
						return req
					default:
						// Discover secrets for GitHub, GitLab and Pagure connections
						otherSecretNames := []string{}
						otherSecretNames = append(otherSecretNames, sfv1.GetGitHubConnectionsSecretName(&softwareFactory.Spec.Zuul)...)
						otherSecretNames = append(otherSecretNames, sfv1.GetGitLabConnectionsSecretName(&softwareFactory.Spec.Zuul)...)
						otherSecretNames = append(otherSecretNames, sfv1.GetPagureConnectionsSecretName(&softwareFactory.Spec.Zuul)...)
						if slices.Contains(otherSecretNames, a.GetName()) {
							return req
						}
						// All others secrets must not trigger reconcile
						return []reconcile.Request{}
					}
				}
				return []reconcile.Request{}
			}),
		).
		Complete(r)
}
