// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/go-logr/logr"

	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

// SoftwareFactoryReconciler reconciles a SoftwareFactory object
type SoftwareFactoryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
	Oneshot    bool
}

//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/finalizers,verbs=update

type SFController struct {
	*SoftwareFactoryReconciler
	cr  *sfv1.SoftwareFactory
	ns  string
	log logr.Logger
	ctx context.Context
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

func (r *SFController) Step() sfv1.SoftwareFactoryStatus {
	sf := r.cr

	services := map[string]bool{}

	jaegerStatus := true
	if sf.Spec.Telemetry.Enabled {
		jaegerStatus = r.DeployJaeger()
		services["Jaeger"] = jaegerStatus
	}

	r.EnsureCA()

	storage_class := create_storageclass("standard")
	r.GetOrCreate(&storage_class)

	// Ensure SF Admin ssh key pair
	r.EnsureSSHKey("admin-ssh-key")

	// The git server service is needed to store system jobs (config-check and config-update)
	services["GitServer"] = r.DeployGitServer()

	services["MariaDB"] = r.DeployMariadb()

	services["Zookeeper"] = r.DeployZookeeper()

	if services["MariaDB"] {
		// Keycloak is enable if Gerrit or Zuul are Enabled
		keycloakEnabled := r.IsKeycloakEnabled()
		if keycloakEnabled {
			services["Keycloak"] = r.DeployKeycloak()
		}
	}

	if services["MariaDB"] && services["Zookeeper"] && services["GitServer"] {
		services["Zuul"] = r.DeployZuul()
	}

	if services["Zookeeper"] {
		services["NodePool"] = r.DeployNodepool()
	}

	services["Gerrit"] = r.DeployGerrit()

	if services["MariaDB"] && services["Zookeeper"] && services["Zuul"] && services["GitServer"] && sf.Spec.Zuul.Enabled {
		services["Config"] = r.SetupConfigJob()
	}

	// managesfEnabled is enabled if Gerrit is Enabled
	managesfEnabled := r.cr.Spec.Gerrit.Enabled
	if managesfEnabled {
		services["ManageSF"] = r.DeployManagesf()
	}

	if sf.Spec.Gerrit.Enabled && services["Gerrit"] {
		services["ConfigRepo"] = r.SetupConfigRepo(sf.Spec.Gerrit.Enabled)
	}

	if sf.Spec.Postfix.Enabled {
		services["Postfix"] = r.DeployPostfix()
	}

	if IsToDeployGateway(r) {
		services["Gateway"] = r.DeployGateway()
	}

	r.log.V(1).Info(messageInfo(r, services))

	ready := isOperatorReady(services)

	if ready {
		r.PodExec("zuul-scheduler-0", "scheduler-sidecar", []string{"generate-zuul-tenant-yaml.sh"})
		r.PodExec("zuul-scheduler-0", "zuul-scheduler", []string{"zuul-scheduler", "full-reconfigure"})
		r.SetupIngress()
	}

	return sfv1.SoftwareFactoryStatus{
		Ready: ready,
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

// Run reconcille loop manually
func (r *SoftwareFactoryReconciler) Standalone(ctx context.Context, ns string, sf sfv1.SoftwareFactory, debugService string) {
	log := log.FromContext(ctx)
	// Ensure the CR name is created
	sf.SetNamespace(ns)
	r.Create(ctx, &sf)
	// Then get's its metadata
	var current_sf sfv1.SoftwareFactory
	if err := r.Get(ctx, client.ObjectKey{
		Name:      sf.GetName(),
		Namespace: ns,
	}, &current_sf); err != nil {
		panic(err.Error())
	}
	// And update the provided metadata so that the reference works out
	sf.ObjectMeta = current_sf.ObjectMeta
	sfc := &SFController{
		SoftwareFactoryReconciler: r,
		cr:                        &sf,
		ns:                        ns,
		log:                       log,
		ctx:                       ctx,
	}

	if debugService != "" {
		sfc.DebugService(debugService)
		os.Exit(0)
	}

	// Manually loop until the step function produces a ready status
	for !sf.Status.Ready {
		sf.Status = sfc.Step()
		fmt.Printf("Step result: %#v\n", sf.Status)
		if sf.Status.Ready {
			break
		}
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SoftwareFactoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var sf sfv1.SoftwareFactory
	if err := r.Get(ctx, req.NamespacedName, &sf); err != nil {
		log.Error(err, "unable to fetch SoftwareFactory resource")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var sfc = &SFController{
		SoftwareFactoryReconciler: r,
		cr:                        &sf,
		ns:                        req.NamespacedName.Namespace,
		log:                       log,
		ctx:                       ctx,
	}

	sf.Status = sfc.Step()

	if err := r.Status().Update(ctx, &sf); err != nil {
		log.Error(err, "unable to update Software Factory status")
		return ctrl.Result{}, err
	}
	if !sf.Status.Ready {
		log.V(1).Info("Reconcile running...")
		delay, _ := time.ParseDuration("20s")
		return ctrl.Result{RequeueAfter: delay}, nil
	} else {
		log.V(1).Info("Reconcile completed!", "sf", sf)
		if r.Oneshot {
			os.Exit(0)
		}
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SoftwareFactoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.SoftwareFactory{}).
		Complete(r)
}
