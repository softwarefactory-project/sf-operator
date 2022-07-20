// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	"os"
	"time"

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

	// Keycloak is enabled if gerrit is enabled
	keycloakEnabled := sf.Spec.Gerrit

	if sf.Spec.Zuul || sf.Spec.Opensearch {
		sfc.EnsureCA()
	}

	zkStatus := sfc.DeployZK(sf.Spec.Zuul)
	// The git server service is needed to store system jobs (config-check and config-update)
	gitServerStatus := sfc.DeployGitServer(sf.Spec.Zuul)

	// Mariadb is enabled if etherpad or lodgeit is enabled.
	mariadbEnabled := sf.Spec.Etherpad || sf.Spec.Lodgeit || sf.Spec.Zuul || keycloakEnabled
	mariadbStatus := sfc.DeployMariadb(mariadbEnabled)

	etherpadStatus := true
	lodgeitStatus := true
	zuulStatus := true
	nodepoolStatus := true
	keycloakStatus := true
	opensearchStatus := true
	if mariadbStatus {
		etherpadStatus = sfc.DeployEtherpad(sf.Spec.Etherpad)
		// lodgeitStatus = sfc.DeployLodgeit(sf.Spec.Lodgit)
		keycloakStatus = sfc.DeployKeycloak(keycloakEnabled)
	}

	if mariadbStatus && zkStatus && gitServerStatus {
		zuulStatus = sfc.DeployZuul(sf.Spec.Zuul)
	}
	if zkStatus {
		nodepoolStatus = sfc.DeployNodepool(sf.Spec.Zuul)
	}

	gerritStatus := sfc.DeployGerrit(sf.Spec.Gerrit, sf.Spec.Zuul)

	if opensearchStatus {
		opensearchStatus = sfc.DeployOpensearch(sf.Spec.Opensearch)
	}

	log.V(1).Info("Service status:",
		"mariadbStatus", mariadbStatus,
		"zkStatus", zkStatus,
		"gitServerStatus", gitServerStatus,
		"etherpadStatus", etherpadStatus,
		"zuulStatus", zuulStatus,
		"gerritStatus", gerritStatus,
		"lodgeitStatus", lodgeitStatus,
		"opensearchStatus", opensearchStatus,
		"keycloakStatus", keycloakStatus)

	sf.Status.Ready = mariadbStatus && etherpadStatus && zuulStatus && gerritStatus && lodgeitStatus && keycloakStatus && zkStatus && nodepoolStatus && opensearchStatus
	if err := r.Status().Update(ctx, &sf); err != nil {
		log.Error(err, "unable to update Software Factory status")
		return ctrl.Result{}, err
	}
	if !sf.Status.Ready {
		log.V(1).Info("Reconcile running...")
		delay, _ := time.ParseDuration("5s")
		return ctrl.Result{RequeueAfter: delay}, nil
	} else {
		sfc.SetupIngress(keycloakEnabled)
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
