// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	"fmt"
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

func (r *SFController) Step() sfv1.SoftwareFactoryStatus {
	sf := r.cr

	jaegerStatus := true
	if sf.Spec.Telemetry.Enabled {
		jaegerStatus = r.DeployJaeger(sf.Spec.Telemetry.Enabled)
	}

	// Keycloak is enabled if gerrit is enabled
	keycloakEnabled := sf.Spec.Gerrit.Enabled

	r.EnsureCA()

	storage_class := create_storageclass("standard")
	r.GetOrCreate(&storage_class)

	// Ensure SF Admin ssh key pair
	r.EnsureSSHKey("admin-ssh-key")

	zkStatus := r.DeployZK(sf.Spec.Zuul.Enabled)
	// The git server service is needed to store system jobs (config-check and config-update)
	gitServerStatus := r.DeployGitServer(sf.Spec.Zuul.Enabled)

	mariadbEnabled := sf.Spec.Etherpad.Enabled || sf.Spec.Lodgeit.Enabled || sf.Spec.Zuul.Enabled || keycloakEnabled
	mariadbStatus := r.DeployMariadb(mariadbEnabled)

	mosquitto_enabled := keycloakEnabled
	mosquittoStatus := r.DeployMosquitto(mosquitto_enabled)

	etherpadStatus := true
	lodgeitStatus := true
	zuulStatus := true
	nodepoolStatus := true
	keycloakStatus := false
	opensearchStatus := true
	opensearchdashboardsStatus := true
	managesfStatus := true
	houndStatus := true
	gerritbotStatus := true
	if mariadbStatus {
		etherpadStatus = r.DeployEtherpad(sf.Spec.Etherpad.Enabled)
		lodgeitStatus = r.DeployLodgeit(sf.Spec.Lodgeit.Enabled)
		keycloakStatus = r.DeployKeycloak(
			keycloakEnabled, sf.Spec.Gerrit.Enabled, sf.Spec.Zuul.Enabled, sf.Spec.OpensearchDashboards.Enabled)
	}

	if mariadbStatus && zkStatus && gitServerStatus {
		zuulStatus = r.DeployZuul(sf.Spec.Zuul, sf.Spec.Gerrit.Enabled, sf.Spec.Telemetry.Enabled)
	}
	if zkStatus {
		nodepoolStatus = r.DeployNodepool(sf.Spec.Zuul.Enabled)
	}

	gerritStatus := r.DeployGerrit(sf.Spec.Gerrit, sf.Spec.Zuul.Enabled, sf.Spec.ConfigLocations.ConfigRepo == "")

	if opensearchStatus {
		opensearchStatus = r.DeployOpensearch(sf.Spec.Opensearch.Enabled)
	}

	if opensearchdashboardsStatus {
		opensearchdashboardsStatus = r.DeployOpensearchDashboards(sf.Spec.OpensearchDashboards.Enabled, keycloakStatus)
	}

	murmurStatus := r.DeployMurmur(sf.Spec.Murmur)

	configStatus := true
	if mariadbStatus && zkStatus && zuulStatus && gitServerStatus && sf.Spec.Zuul.Enabled {
		configStatus = r.SetupConfigJob()
	}

	managesfStatus = r.DeployManagesf(managesfStatus, sf.Spec.Gerrit.Enabled, sf.Spec.Zuul.Enabled)
	// Handle populate of the config repository
	var config_repo_url string
	var config_repo_user string
	if sf.Spec.ConfigLocations.ConfigRepo == "" && sf.Spec.Gerrit.Enabled {
		config_repo_url = "gerrit-sshd:29418/config"
		config_repo_user = "admin"
	} else if sf.Spec.ConfigLocations.ConfigRepo != "" {
		var user string
		if sf.Spec.ConfigLocations.User != "" {
			user = sf.Spec.ConfigLocations.User
		} else {
			user = "git"
		}
		config_repo_url = sf.Spec.ConfigLocations.ConfigRepo
		config_repo_user = user
	} else {
		// TODO: uncomment the panic once the config repo is actually working
		// panic("ConfigRepo settings not supported !")
	}

	configRepoStatus := true
	if sf.Spec.Gerrit.Enabled && gerritStatus {
		configRepoStatus = r.SetupConfigRepo(
			config_repo_url, config_repo_user, sf.Spec.Gerrit.Enabled)
	}

	if sf.Spec.Hound.Enabled {
		houndStatus = r.DeployHound(sf.Spec.Hound.Enabled)
	}

	if sf.Spec.GerritBot.Enabled && gerritStatus {
		gerritbotStatus = r.DeployGerritBot(sf.Spec.GerritBot.Enabled)
	}

	r.log.V(1).Info("Service status:",
		"mariadbStatus", mariadbStatus,
		"zkStatus", zkStatus,
		"gitServerStatus", gitServerStatus,
		"etherpadStatus", etherpadStatus,
		"zuulStatus", zuulStatus,
		"gerritStatus", gerritStatus,
		"lodgeitStatus", lodgeitStatus,
		"opensearchStatus", opensearchStatus,
		"opensearchdashboardsStatus", opensearchdashboardsStatus,
		"keycloakStatus", keycloakStatus,
		"murmurStatus", murmurStatus,
		"mosquittoStatus", mosquittoStatus,
		"configStatus", configStatus,
		"configRepoStatus", configRepoStatus,
		"managesfStatus", managesfStatus,
		"houndStatus", houndStatus,
		"gerritbotStatus", gerritbotStatus,
	)

	ready := (mariadbStatus && etherpadStatus && zuulStatus &&
		gerritStatus && lodgeitStatus && keycloakStatus &&
		zkStatus && nodepoolStatus && opensearchStatus &&
		opensearchdashboardsStatus && configStatus && configRepoStatus &&
		murmurStatus && mosquittoStatus && jaegerStatus && managesfStatus &&
		houndStatus && gerritbotStatus)

	if ready {
		r.SetupIngress(keycloakEnabled)
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
		delay, _ := time.ParseDuration("5s")
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
