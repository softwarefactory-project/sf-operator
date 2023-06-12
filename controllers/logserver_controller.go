// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main Reconcile loop.

package controllers

import (
	"context"
	_ "embed"
	"strconv"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/finalizers,verbs=update

const LOGSERVER_IDENT = "logserver"
const LOGSERVER_HTTPD_PORT = 8080
const LOGSERVER_HTTPD_PORT_NAME = "logserver-httpd"
const LOGSERVER_IMAGE = "registry.access.redhat.com/rhscl/httpd-24-rhel7:latest"

const LOGSERVER_SSHD_PORT = 2222
const LOGSERVER_SSHD_PORT_NAME = "logserver-sshd"

const LOGSERVER_SSHD_IMAGE = "quay.io/software-factory/sshd:0.1-2"

const CONTAINER_HTTP_BASE_DIR = "/opt/rh/httpd24/root"

const LOGSERVER_DATA = "/var/www"

//go:embed static/logserver/run.sh
var logserver_run string

const PURGELOG_IDENT = "purgelogs"
const PURGELOG_IMAGE = "quay.io/software-factory/purgelogs:0.2.1-3"
const PURGELOG_LOGS_DIR = "/home/logs"

//go:embed static/logserver/logserver.conf.tmpl
var logserverconf string

type LogServerReconciler struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
}

type LogServerController struct {
	SFUtilContext
	cr sfv1.LogServer
}

func isLogserverReady(logserver sfv1.LogServer) bool {
	return logserver.Status.ObservedGeneration == logserver.Generation && logserver.Status.Ready
}

func (r *LogServerController) DeployLogserver() sfv1.LogServerStatus {

	r.EnsureSSHKey(LOGSERVER_IDENT + "-keys")

	cm_data := make(map[string]string)
	cm_data["logserver.conf"], _ = parse_string(logserverconf, struct {
		ServerPort    int
		ServerRoot    string
		LogserverRoot string
	}{
		ServerPort:    LOGSERVER_HTTPD_PORT,
		ServerRoot:    CONTAINER_HTTP_BASE_DIR,
		LogserverRoot: LOGSERVER_DATA,
	})
	cm_data["index.html"] = ""
	cm_data["run.sh"] = logserver_run

	r.EnsureConfigMap(LOGSERVER_IDENT, cm_data)

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      LOGSERVER_IDENT + "-config-vol",
			MountPath: "/etc/httpd/conf.d/logserver.conf",
			ReadOnly:  true,
			SubPath:   "logserver.conf",
		},
		{
			Name:      LOGSERVER_IDENT + "-config-vol",
			MountPath: CONTAINER_HTTP_BASE_DIR + LOGSERVER_DATA + "/index.html",
			ReadOnly:  true,
			SubPath:   "index.html",
		},
		{
			Name:      LOGSERVER_IDENT,
			MountPath: CONTAINER_HTTP_BASE_DIR + LOGSERVER_DATA + "/logs",
		},
	}

	// Create the deployment
	dep := r.create_deployment(LOGSERVER_IDENT, LOGSERVER_IMAGE)

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	data_pvc := r.create_pvc(LOGSERVER_IDENT, r.baseGetStorageConfOrDefault(
		r.cr.Spec.Settings.Storage, r.cr.Spec.StorageClassName))
	r.GetOrCreate(&data_pvc)
	var mod int32 = 256 // decimal for 0400 octal
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		create_volume_cm(LOGSERVER_IDENT+"-config-vol", LOGSERVER_IDENT+"-config-map"),
		{
			Name: LOGSERVER_IDENT + "-keys",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  LOGSERVER_IDENT + "-keys",
					DefaultMode: &mod,
				},
			},
		},
		{
			Name: LOGSERVER_IDENT,
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: LOGSERVER_IDENT,
				},
			},
		},
	}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		create_container_port(LOGSERVER_HTTPD_PORT, LOGSERVER_HTTPD_PORT_NAME),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", LOGSERVER_HTTPD_PORT)

	// Create services exposed by logserver
	service_ports := []int32{LOGSERVER_HTTPD_PORT}
	httpd_service := r.create_service(
		LOGSERVER_HTTPD_PORT_NAME, LOGSERVER_IDENT, service_ports, LOGSERVER_HTTPD_PORT_NAME)
	r.GetOrCreate(&httpd_service)

	// Side Car Container
	volumeMounts_sidecar := []apiv1.VolumeMount{
		{
			Name:      LOGSERVER_IDENT,
			MountPath: "/home/data/rsync",
		},
		{
			Name:      LOGSERVER_IDENT + "-keys",
			MountPath: "/var/ssh-keys",
			ReadOnly:  true,
		},
		{
			Name:      LOGSERVER_IDENT + "-config-vol",
			MountPath: "/conf",
		},
	}

	ports_sidecar := []apiv1.ContainerPort{
		create_container_port(LOGSERVER_SSHD_PORT, LOGSERVER_SSHD_PORT_NAME),
	}

	env_sidecar := []apiv1.EnvVar{
		create_env("AUTHORIZED_KEY", r.cr.Spec.AuthorizedSSHKey),
	}

	// Setup the sidecar container for sshd
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, apiv1.Container{
		Name:            LOGSERVER_SSHD_PORT_NAME,
		Image:           LOGSERVER_SSHD_IMAGE,
		Command:         []string{"bash", "/conf/run.sh"},
		VolumeMounts:    volumeMounts_sidecar,
		Env:             env_sidecar,
		Ports:           ports_sidecar,
		SecurityContext: create_security_context(false),
	})

	// Add PurgeLog container
	loopdelay := 3600
	if r.cr.Spec.Settings.LoopDelay > 0 {
		loopdelay = r.cr.Spec.Settings.LoopDelay
	}

	retentiondays := 60
	if r.cr.Spec.Settings.RetentionDays > 0 {
		retentiondays = r.cr.Spec.Settings.RetentionDays
	}

	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, apiv1.Container{
		Name:  PURGELOG_IDENT,
		Image: PURGELOG_IMAGE,
		Command: []string{
			"/usr/local/bin/purgelogs",
			"--retention-days",
			strconv.Itoa(retentiondays),
			"--loop", strconv.Itoa(loopdelay),
			"--log-path-dir",
			PURGELOG_LOGS_DIR,
			"--debug"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      LOGSERVER_IDENT,
				MountPath: PURGELOG_LOGS_DIR,
			},
		},
		SecurityContext: create_security_context(false),
	})

	r.GetOrCreate(&dep)

	sshd_service_ports := []int32{LOGSERVER_SSHD_PORT}
	sshd_service := r.create_service(LOGSERVER_SSHD_PORT_NAME, LOGSERVER_IDENT, sshd_service_ports, LOGSERVER_SSHD_PORT_NAME)
	r.GetOrCreate(&sshd_service)

	pvc_readiness := r.reconcile_expand_pvc(LOGSERVER_IDENT, r.cr.Spec.Settings.Storage)

	return sfv1.LogServerStatus{
		Ready:              r.IsDeploymentReady(&dep) && pvc_readiness,
		ObservedGeneration: r.cr.Generation,
		ReconciledBy:       getOperatorConditionName(),
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *LogServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(1).Info("Logserver CR - Entering reconcile loop")

	var cr sfv1.LogServer

	if err := r.Client.Get(ctx, req.NamespacedName, &cr); err != nil && errors.IsNotFound(err) {
		log.Error(err, "unable to fetch LogServer resource")
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
		owner:      &cr,
	}

	var controller = LogServerController{
		SFUtilContext: *utils,
		cr:            cr,
	}

	cr.Status = controller.DeployLogserver()

	if err := r.Client.Status().Update(ctx, &cr); err != nil {
		log.Error(err, "unable to update LogServer status")
		return ctrl.Result{}, err
	}
	if !cr.Status.Ready {
		log.V(1).Info("Logserver CR - Reconcile running...")
		delay, _ := time.ParseDuration("20s")
		return ctrl.Result{RequeueAfter: delay}, nil
	} else {
		log.V(1).Info("Logserver CR - Reconcile completed!")
		controller.setupLogserverIngress()
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.LogServer{}).
		Complete(r)
}

func (r *LogServerController) setupLogserverIngress() {
	r.ensureHTTPSRoute(
		r.cr.Name+"-logserver", LOGSERVER_IDENT,
		LOGSERVER_HTTPD_PORT_NAME, "/", LOGSERVER_HTTPD_PORT, map[string]string{}, r.cr.Spec.FQDN)
}
