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

	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/finalizers,verbs=update

const logserverIdent = "logserver"
const httpdPort = 8080
const httpdPortName = "logserver-httpd"
const image = "registry.access.redhat.com/rhscl/httpd-24-rhel7:latest"

//go:embed static/logserver/logserver-entrypoint.sh
var logserverEntrypoint string

const sshdPort = 2222
const sshdPortName = "logserver-sshd"

const sshdImage = "quay.io/software-factory/sshd:0.1-2"

const httpdBaseDir = "/opt/rh/httpd24/root"

const httpdData = "/var/www"

//go:embed static/logserver/run.sh
var logserverRun string

const purgelogIdent = "purgelogs"
const purgeLogsImage = "quay.io/software-factory/purgelogs:0.2.3-2"
const purgelogsLogsDir = "/home/logs"

//go:embed static/logserver/logserver.conf.tmpl
var logserverConf string

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

func getLogserverSettingsOrDefault(settings sfv1.LogServerSpecSettings) (int, int) {
	loopdelay := 3600
	if settings.LoopDelay > 0 {
		loopdelay = settings.LoopDelay
	}

	retentiondays := 60
	if settings.RetentionDays > 0 {
		retentiondays = settings.RetentionDays
	}
	return loopdelay, retentiondays
}

func (r *LogServerController) ensureLogserverPodMonitor() bool {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "sf",
			"run": logserverIdent,
		},
	}
	nePort := GetNodeexporterPortName(logserverIdent)
	desiredLsPodmonitor := r.mkPodMonitor(logserverIdent+"-monitor", nePort, selector)
	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version": "1",
	}
	desiredLsPodmonitor.ObjectMeta.Annotations = annotations
	currentLspm := monitoringv1.PodMonitor{}
	if !r.GetM(desiredLsPodmonitor.Name, &currentLspm) {
		r.CreateR(&desiredLsPodmonitor)
		return false
	} else {
		if !mapEquals(&currentLspm.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Logserver PodMonitor configuration changed, updating...")
			currentLspm.Spec = desiredLsPodmonitor.Spec
			currentLspm.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentLspm)
			return false
		}
	}
	return true
}

// Create some default, interesting alerts
func (r *LogServerController) ensureLogserverPromRule() bool {
	diskFullLabels := map[string]string{
		"lasttime": "{{ $value | humanizeTimestamp }}",
		"severity": "critical",
	}
	diskFullAnnotations := map[string]string{
		"description": "Log server only has {{ $value | humanize1024 }} free disk available.",
		"summary":     "Log server out of disk",
	}
	diskFull3daysAnnotations := map[string]string{
		"description": "Log server only has at most three days' worth ({{ $value | humanize1024 }}) of free disk available.",
		"summary":     "Log server running out of disk",
	}
	diskFull := mkPrometheusAlertRule(
		"OutOfDiskNow",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 10) and "+
				"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 20 * 1024 ^ 3)"),
		"30m",
		diskFullLabels,
		diskFullAnnotations,
	)
	diskFullIn3days := mkPrometheusAlertRule(
		"OutOfDiskInThreeDays",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 50) and "+
				"(predict_linear(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"}[1d], 3 * 24 * 3600) < 0) and "+
				"(node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} <= 1e+11)"),
		"12h",
		map[string]string{},
		diskFull3daysAnnotations,
	)
	lsDiskRuleGroup := mkPrometheusRuleGroup(
		"disk.rules",
		[]monitoringv1.Rule{diskFull, diskFullIn3days})
	desiredLsPromRule := r.mkPrometheusRuleCR(logserverIdent + ".rules")
	desiredLsPromRule.Spec.Groups = append(desiredLsPromRule.Spec.Groups, lsDiskRuleGroup)

	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version": "1",
	}
	desiredLsPromRule.ObjectMeta.Annotations = annotations
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredLsPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredLsPromRule)
		return false
	} else {
		if !mapEquals(&currentPromRule.ObjectMeta.Annotations, &annotations) {
			r.log.V(1).Info("Logserver default Prometheus rules changed, updating...")
			currentPromRule.Spec = desiredLsPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

func (r *LogServerController) DeployLogserver() sfv1.LogServerStatus {
	log := log.FromContext(r.ctx)

	r.EnsureSSHKey(logserverIdent + "-keys")

	cmData := make(map[string]string)
	cmData["logserver.conf"], _ = ParseString(logserverConf, struct {
		ServerRoot    string
		LogserverRoot string
	}{
		ServerRoot:    httpdBaseDir,
		LogserverRoot: httpdData,
	})
	cmData["index.html"] = ""
	cmData["run.sh"] = logserverRun

	lgEntryScriptName := logserverIdent + "-entrypoint.sh"
	cmData[lgEntryScriptName] = logserverEntrypoint

	r.EnsureConfigMap(logserverIdent, cmData)

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/etc/httpd/conf.d/logserver.conf",
			ReadOnly:  true,
			SubPath:   "logserver.conf",
		},
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: httpdBaseDir + httpdData + "/index.html",
			ReadOnly:  true,
			SubPath:   "index.html",
		},
		{
			Name:      logserverIdent,
			MountPath: httpdBaseDir + httpdData + "/logs",
		},
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/usr/bin/" + lgEntryScriptName,
			SubPath:   lgEntryScriptName,
		},
	}

	// Create the deployment
	dep := r.mkDeployment(logserverIdent, image)

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	// NOTE: Currently we are providing "ReadWriteOnce" access mode
	// for the Logserver, due "ReadWriteMany" requires special storage backend.
	dataPvc := r.MkPVC(logserverIdent, BaseGetStorageConfOrDefault(
		r.cr.Spec.Settings.Storage, r.cr.Spec.StorageClassName), apiv1.ReadWriteOnce)
	r.GetOrCreate(&dataPvc)
	var mod int32 = 256 // decimal for 0400 octal
	dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
		{
			Name: logserverIdent + "-config-vol",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: logserverIdent + "-config-map",
					},
					DefaultMode: &Execmod,
				},
			},
		},
		{
			Name: logserverIdent + "-keys",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  logserverIdent + "-keys",
					DefaultMode: &mod,
				},
			},
		},
		{
			Name: logserverIdent,
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: logserverIdent,
				},
			},
		},
	}

	dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
		MKContainerPort(httpdPort, httpdPortName),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = mkReadinessHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].StartupProbe = mkStartupHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].LivenessProbe = mkLiveHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].Command = []string{
		"/usr/bin/" + lgEntryScriptName,
	}

	// Create services exposed by logserver
	servicePorts := []int32{httpdPort}
	httpdService := r.mkService(
		httpdPortName, logserverIdent, servicePorts, httpdPortName)
	r.GetOrCreate(&httpdService)

	// Side Car Container
	volumeMountsSidecar := []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: "/home/data/rsync",
		},
		{
			Name:      logserverIdent + "-keys",
			MountPath: "/var/ssh-keys",
			ReadOnly:  true,
		},
		{
			Name:      logserverIdent + "-config-vol",
			MountPath: "/conf",
		},
	}

	portsSidecar := []apiv1.ContainerPort{
		MKContainerPort(sshdPort, sshdPortName),
	}

	envSidecar := []apiv1.EnvVar{
		MKEnvVar("AUTHORIZED_KEY", r.cr.Spec.AuthorizedSSHKey),
	}

	// Setup the sidecar container for sshd
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, apiv1.Container{
		Name:            sshdPortName,
		Image:           sshdImage,
		Command:         []string{"bash", "/conf/run.sh"},
		VolumeMounts:    volumeMountsSidecar,
		Env:             envSidecar,
		Ports:           portsSidecar,
		SecurityContext: mkSecurityContext(false),
	})

	loopdelay, retentiondays := getLogserverSettingsOrDefault(r.cr.Spec.Settings)

	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, apiv1.Container{
		Name:  purgelogIdent,
		Image: purgeLogsImage,
		Command: []string{
			"/usr/local/bin/purgelogs",
			"--retention-days",
			strconv.Itoa(retentiondays),
			"--loop", strconv.Itoa(loopdelay),
			"--log-path-dir",
			purgelogsLogsDir,
			"--debug"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      logserverIdent,
				MountPath: purgelogsLogsDir,
			},
		},
		SecurityContext: mkSecurityContext(false),
	})

	// add volumestats exporter
	volumeMountsStatsExporter := []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: "/home/data/rsync",
		},
	}

	statsExporter := createNodeExporterSideCarContainer(logserverIdent, volumeMountsStatsExporter)
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, statsExporter)

	// Increase serial each time you need to enforce a deployment change/pod restart between operator versions
	annotations := map[string]string{
		"fqdn":           r.cr.Spec.FQDN,
		"serial":         "2",
		"httpd-conf":     checksum([]byte(logserverConf)),
		"purgeLogConfig": "retentionDays:" + strconv.Itoa(retentiondays) + " loopDelay:" + strconv.Itoa(loopdelay),
	}

	// do we have an existing deployment?
	currentDep := v1.Deployment{}
	deploymentUpdated := true
	if r.GetM(dep.GetName(), &currentDep) {
		// Are annotations in sync?
		if !mapEquals(&currentDep.Spec.Template.ObjectMeta.Annotations, &annotations) {
			currentDep.Spec.Template.Spec = dep.Spec.Template.Spec
			currentDep.Spec.Template.ObjectMeta.Annotations = annotations
			log.V(1).Info("Logserver pod restarting to apply changes ...")
			deploymentUpdated = r.UpdateR(&currentDep)
		}

	} else {
		dep.Spec.Template.ObjectMeta.Annotations = annotations
		r.log.V(1).Info("Creating object", "name", dep.GetName())
		r.CreateR(&dep)
	}

	sshdServicePorts := []int32{sshdPort}
	sshdService := r.mkService(sshdPortName, logserverIdent, sshdServicePorts, sshdPortName)
	r.GetOrCreate(&sshdService)

	r.getOrCreateNodeExporterSideCarService(logserverIdent)

	pvcReadiness := r.reconcileExpandPVC(logserverIdent, r.cr.Spec.Settings.Storage)

	// refresh current deployment
	r.GetM(dep.GetName(), &currentDep)

	routeReady := r.ensureHTTPSRoute(
		r.cr.Name+"-logserver", logserverIdent,
		httpdPortName, "/", httpdPort, map[string]string{}, r.cr.Spec.FQDN, r.cr.Spec.LetsEncrypt)

	// TODO(mhu) We may want to open an ingress to port 9100 for an external prometheus instance.
	// TODO(mhu) we may want to include monitoring objects' status in readiness computation
	r.ensureLogserverPodMonitor()
	r.ensureLogserverPromRule()

	isDeploymentReady := r.IsDeploymentReady(&currentDep)
	updateConditions(&r.cr.Status.Conditions, logserverIdent, isDeploymentReady)

	return sfv1.LogServerStatus{
		Ready:              deploymentUpdated && isDeploymentReady && pvcReadiness && routeReady,
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

	// Setup LetsEncrypt Issuer if needed
	if cr.Spec.LetsEncrypt != nil {
		controller.ensureLetsEncryptIssuer(*cr.Spec.LetsEncrypt)
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
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.LogServer{}).
		Owns(&apiv1.Secret{}).
		Owns(&certv1.Certificate{}).
		Complete(r)
}
