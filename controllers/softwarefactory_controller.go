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

	"github.com/fatih/color"

	"k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

type SoftwareFactoryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
}

// Run `make manifests` to apply rbac change
//
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=softwarefactories/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=jobs;pods;pods/exec;services;routes;routes/custom-host;statefulsets;deployments;configmaps;secrets;persistentvolumeclaims;serviceaccounts;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=jobs/status;pods/status;services/status;routes/status;statefulsets/status;deployments/status;configmaps/status;secrets/status;persistentvolumeclaims/status;serviceaccounts/status;roles/status,verbs=get
//+kubebuilder:rbac:groups=cert-manager.io,resources=*,verbs=get;list;watch;create;update;patch;delete

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

func (r *SFController) SetupIngress() {
	r.setupGerritIngress()
	r.setupZuulIngress()
}

func (r *SFController) DeployLogserverResource() bool {

	resource := sfv1.LogServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LOGSERVER_IDENT,
			Namespace: r.ns,
		},
	}

	exists := r.GetM(LOGSERVER_IDENT, &resource)

	if exists {
		resource.Spec.Settings = r.cr.Spec.Logserver
		r.UpdateR(&resource)
	} else {
		pub_key, err := r.getSecretDataFromKey("zuul-ssh-key", "pub")
		if err != nil {
			return false
		}
		pub_key_b64 := base64.StdEncoding.EncodeToString(pub_key)
		resource.Spec = sfv1.LogServerSpec{
			FQDN:             r.cr.Spec.FQDN,
			StorageClassName: r.cr.Spec.StorageClassName,
			AuthorizedSSHKey: pub_key_b64,
			Settings:         r.cr.Spec.Logserver,
		}
		r.CreateR(&resource)
	}
	return resource.Status.Ready
}

func (r *SFController) Step() sfv1.SoftwareFactoryStatus {
	services := map[string]bool{}
	services["Zuul"] = false

	r.EnsureCA()

	// Ensure SF Admin ssh key pair
	r.EnsureSSHKey("admin-ssh-key")
	r.DeployZuulSecrets()

	// The git server service is needed to store system jobs (config-check and config-update)
	services["GitServer"] = r.DeployGitServer()

	services["MariaDB"] = r.DeployMariadb()

	services["Zookeeper"] = r.DeployZookeeper()

	services["Gerrit"] = r.DeployGerrit()

	if services["Gerrit"] {
		services["ConfigRepo"] = r.SetupConfigRepo()
	}

	if services["MariaDB"] && services["Zookeeper"] && services["GitServer"] && services["Gerrit"] && services["ConfigRepo"] {
		services["Zuul"] = r.DeployZuul()
	}

	services["Logserver"] = r.DeployLogserverResource()

	if services["Zookeeper"] {
		services["NodePool"] = r.DeployNodepool()
	}

	if services["Zuul"] {
		services["Config"] = r.SetupConfigJob()
	}

	services["ManagesfResources"] = r.DeployManagesfResources()

	r.log.V(1).Info(messageInfo(r, services))

	ready := isOperatorReady(services)

	if ready {
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SoftwareFactoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	var utils = &SFUtilContext{
		Client:     r.Client,
		Scheme:     r.Scheme,
		RESTClient: r.RESTClient,
		RESTConfig: r.RESTConfig,
		ns:         req.NamespacedName.Namespace,
		log:        log,
		ctx:        ctx,
		owner:      &sf,
	}

	var controller = SFController{
		SFUtilContext: *utils,
		cr:            sf,
	}

	sf.Status = controller.Step()

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
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SoftwareFactoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.SoftwareFactory{}).
		Complete(r)
}
