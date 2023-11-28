// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"time"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/fatih/color"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"k8s.io/client-go/rest"
	"k8s.io/utils/strings/slices"

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

	apiroutev1 "github.com/openshift/api/route/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
)

type SoftwareFactoryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
	CancelFunc context.CancelFunc
	Completed  bool
}

// Run `make manifests` to apply rbac change
//
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=jobs;pods;pods/exec;services;routes;routes/custom-host;statefulsets;deployments;configmaps;secrets;persistentvolumeclaims;serviceaccounts;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=jobs/status;pods/status;services/status;routes/status;statefulsets/status;deployments/status;configmaps/status;secrets/status;persistentvolumeclaims/status;serviceaccounts/status;roles/status,verbs=get
//+kubebuilder:rbac:groups=cert-manager.io,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

type SFController struct {
	SFUtilContext
	cr sfv1.SoftwareFactory
}

func messageGenerator(isReady bool, goodmsg string, badmsg string) string {
	if isReady {
		return color.GreenString(goodmsg)
	}
	return color.RedString(badmsg)
}

func messageInfo(r *SFController, services map[string]bool) string {
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

func (r *SFController) DeployLogserverResource() bool {
	pubKey, err := r.GetSecretDataFromKey("zuul-ssh-key", "pub")
	if err != nil {
		return false
	}
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	cr := sfv1.LogServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: logserverIdent,
		},
		Spec: sfv1.LogServerSpec{
			FQDN:             r.cr.Spec.FQDN,
			LetsEncrypt:      r.cr.Spec.LetsEncrypt,
			StorageClassName: r.cr.Spec.Logserver.Storage.ClassName,
			AuthorizedSSHKey: pubKeyB64,
			Settings:         r.cr.Spec.Logserver,
		},
	}
	var logserverController = LogServerController{
		SFUtilContext: r.SFUtilContext,
		cr:            cr,
	}
	return logserverController.DeployLogserver().Ready
}

// cleanup ensures removal of legacy resources
func (r *SFController) cleanup() {
	r.DeleteR(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ns,
			Name:      BuildLogsHttpdPortName,
		},
	})

	// clean up old podmonitors if they exist. Remove after next release
	currentZKpm := monitoringv1.PodMonitor{}
	if r.GetM(ZookeeperIdent+"-monitor", &currentZKpm) {
		r.DeleteR(&currentZKpm)
	}
	currentDBpm := monitoringv1.PodMonitor{}
	if r.GetM(MariaDBIdent+"-monitor", &currentDBpm) {
		r.DeleteR(&currentDBpm)
	}
	currentGSpm := monitoringv1.PodMonitor{}
	if r.GetM(GitServerIdent+"-monitor", &currentGSpm) {
		r.DeleteR(&currentGSpm)
	}
	currentZPM := monitoringv1.PodMonitor{}
	if r.GetM("zuul-monitor", &currentZPM) {
		r.DeleteR(&currentZPM)
	}
	currentNPM := monitoringv1.PodMonitor{}
	if r.GetM("nodepool-monitor", &currentNPM) {
		r.DeleteR(&currentNPM)
	}
	// remove a legacy Route definition for Zuul
	r.DeleteR(&apiroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ns,
			Name:      r.cr.Spec.FQDN + "-zuul-red",
		},
	})
}

func (r *SFController) Step() sfv1.SoftwareFactoryStatus {

	r.cleanup()

	DURuleGroups := []monitoringv1.RuleGroup{
		sfmonitoring.MkDiskUsageRuleGroup(r.ns, "sf"),
	}
	monitoredPorts := []string{}
	selectorRunList := []string{}

	services := map[string]bool{}
	services["Zuul"] = false

	// Setup a Self-Signed certificate issuer
	r.EnsureLocalCA()

	// Setup LetsEncrypt Issuer if needed
	if r.cr.Spec.LetsEncrypt != nil {
		r.ensureLetsEncryptIssuer(*r.cr.Spec.LetsEncrypt)
	}

	// Ensure SF Admin ssh key pair
	r.DeployZuulSecrets()

	// The git server service is needed to store system jobs (config-check and config-update)
	services["GitServer"] = r.DeployGitServer()
	if services["GitServer"] {
		monitoredPorts = append(monitoredPorts, sfmonitoring.GetTruncatedPortName(GitServerIdent, sfmonitoring.NodeExporterPortNameSuffix))
		selectorRunList = append(selectorRunList, GitServerIdent)
	}

	services["MariaDB"] = r.DeployMariadb()
	if services["MariaDB"] {
		monitoredPorts = append(monitoredPorts, sfmonitoring.GetTruncatedPortName(MariaDBIdent, sfmonitoring.NodeExporterPortNameSuffix))
		selectorRunList = append(selectorRunList, MariaDBIdent)
	}

	services["Zookeeper"] = r.DeployZookeeper()
	if services["Zookeeper"] {
		monitoredPorts = append(monitoredPorts, sfmonitoring.GetTruncatedPortName(ZookeeperIdent, sfmonitoring.NodeExporterPortNameSuffix))
		selectorRunList = append(selectorRunList, ZookeeperIdent)
	}

	if services["MariaDB"] && services["Zookeeper"] && services["GitServer"] {
		services["Zuul"] = r.DeployZuul()
	}

	services["Logserver"] = r.DeployLogserverResource()

	if services["Zookeeper"] {
		nodepool := r.DeployNodepool()
		services["NodePoolLauncher"] = nodepool[LauncherIdent]
		services["NodePoolBuilder"] = nodepool[BuilderIdent]
		if services["NodePoolLauncher"] && services["NodePoolBuilder"] {
			monitoredPorts = append(
				monitoredPorts,
				sfmonitoring.GetTruncatedPortName(BuilderIdent, sfmonitoring.NodeExporterPortNameSuffix),
				NodepoolStatsdExporterPortName,
			)
			selectorRunList = append(selectorRunList, LauncherIdent, BuilderIdent)
		}
	}

	if services["Zuul"] {
		monitoredPorts = append(
			monitoredPorts,
			sfmonitoring.GetTruncatedPortName("zuul-scheduler", sfmonitoring.NodeExporterPortNameSuffix),
			sfmonitoring.GetTruncatedPortName("zuul-executor", sfmonitoring.NodeExporterPortNameSuffix),
			sfmonitoring.GetTruncatedPortName("zuul-merger", sfmonitoring.NodeExporterPortNameSuffix),
			sfmonitoring.GetTruncatedPortName("zuul-web", sfmonitoring.NodeExporterPortNameSuffix),
			ZuulPrometheusPortName,
			ZuulStatsdExporterPortName,
		)
		selectorRunList = append(selectorRunList, "zuul-scheduler", "zuul-executor", "zuul-merger", "zuul-web")
		services["Config"] = r.SetupConfigJob()
		if services["Config"] {
			conds.RefreshCondition(&r.cr.Status.Conditions, "ConfigReady", metav1.ConditionTrue, "Ready", "Config is ready")
		}
	}

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
	// TODO? we could add this to the readiness computation.
	r.EnsureSFPodMonitor(monitoredPorts, podMonitorSelector)
	r.EnsureDiskUsagePromRule(DURuleGroups)

	r.log.V(1).Info(messageInfo(r, services))

	return sfv1.SoftwareFactoryStatus{
		Ready:              isOperatorReady(services),
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

func (r *SoftwareFactoryReconciler) mkSFController(
	ctx context.Context, ns string, owner client.Object, cr sfv1.SoftwareFactory,
	standalone bool) SFController {
	return SFController{
		SFUtilContext: SFUtilContext{
			Client:     r.Client,
			Scheme:     r.Scheme,
			RESTClient: r.RESTClient,
			RESTConfig: r.RESTConfig,
			ns:         ns,
			log:        log.FromContext(ctx),
			ctx:        ctx,
			owner:      owner,
			standalone: standalone,
		},
		cr: cr,
	}
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

func (r *SoftwareFactoryReconciler) StandaloneReconcile(ctx context.Context, ns string, sf sfv1.SoftwareFactory) {
	d, _ := time.ParseDuration("5s")
	log := log.FromContext(ctx)

	// Create a fake resource that simulate the Resource Owner.
	// A deletion to that resource Owner will cascade delete owned resources
	controllerCMName := "sf-standalone-owner"
	controllerCM := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerCMName,
			Namespace: ns,
		}}
	err := r.Client.Get(
		ctx, client.ObjectKey{Name: controllerCMName, Namespace: ns}, &controllerCM)
	if err != nil && k8s_errors.IsNotFound(err) {
		controllerCM.Data = nil
		log.Info("Creating ConfigMap", "name", controllerCMName)
		// Create the fake controller configMap
		if err := r.Create(ctx, &controllerCM); err != nil {
			log.Error(err, "Unable to create configMap", "name", controllerCMName)
			return
		}
	}

	sfCtrl := r.mkSFController(ctx, ns, &controllerCM, sf, true)

	for {
		status := sfCtrl.Step()
		if status.Ready {
			break
		}
		log.Info("Waiting 5s for the next reconcile call ...")
		time.Sleep(d)
	}
	log.Info("Standalone reconcile done.")
}

// SetupWithManager sets up the controller with the Manager.
func (r *SoftwareFactoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.SoftwareFactory{}).
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
					req := []reconcile.Request{
						{NamespacedName: types.NamespacedName{
							Name:      softwareFactory.Name,
							Namespace: a.GetNamespace(),
						}}}
					switch updatedResourceName := a.GetName(); updatedResourceName {
					case NodepoolProvidersSecretsName:
						return req
					case CustomSSLSecretName:
						return req
					default:
						// Discover secrets for github and gitlab connections
						otherSecretNames := []string{}
						otherSecretNames = append(otherSecretNames, sfv1.GetGitHubConnectionsSecretName(&softwareFactory.Spec.Zuul)...)
						otherSecretNames = append(otherSecretNames, sfv1.GetGitLabConnectionsSecretName(&softwareFactory.Spec.Zuul)...)
						if slices.Contains(otherSecretNames, a.GetName()) {
							return req
						}
						// All others secrets must trigger reconcile
						return []reconcile.Request{}
					}
				}
				return []reconcile.Request{}
			}),
		).
		Owns(&certv1.Certificate{}).
		Complete(r)
}
