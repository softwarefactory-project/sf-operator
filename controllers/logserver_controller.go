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

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/conds"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
)

//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=logservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

const logserverIdent = "logserver"
const httpdPort = 8080
const httpdPortName = "logserver-httpd"

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
	nePort := sfmonitoring.GetTruncatedPortName(logserverIdent, sfmonitoring.NodeExporterPortNameSuffix)
	desiredLsPodmonitor := sfmonitoring.MkPodMonitor(logserverIdent+"-monitor", r.ns, []string{nePort}, selector)
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
		if !utils.MapEquals(&currentLspm.ObjectMeta.Annotations, &annotations) {
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
	diskFullAnnotations := map[string]string{
		"description": "Log server only has {{ $value | humanize1024 }} free disk available.",
		"summary":     "Log server out of disk",
	}
	diskFull3daysAnnotations := map[string]string{
		"description": "Log server only has at most three days' worth ({{ $value | humanize1024 }}) of free disk available.",
		"summary":     "Log server running out of disk",
	}
	diskFull := sfmonitoring.MkPrometheusAlertRule(
		"OutOfDiskNow",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 10) and "+
				"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 20 * 1024 ^ 3)"),
		"30m",
		sfmonitoring.CriticalSeverityLabel,
		diskFullAnnotations,
	)
	diskFullIn3days := sfmonitoring.MkPrometheusAlertRule(
		"OutOfDiskInThreeDays",
		intstr.FromString(
			"(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} * 100 /"+
				" node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} < 50) and "+
				"(predict_linear(node_filesystem_avail_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"}[1d], 3 * 24 * 3600) < 0) and "+
				"(node_filesystem_size_bytes{job=\""+r.ns+"/"+logserverIdent+"-monitor\"} <= 1e+11)"),
		"12h",
		sfmonitoring.WarningSeverityLabel,
		diskFull3daysAnnotations,
	)
	lsDiskRuleGroup := sfmonitoring.MkPrometheusRuleGroup(
		"disk_default.rules",
		[]monitoringv1.Rule{diskFull, diskFullIn3days})
	desiredLsPromRule := sfmonitoring.MkPrometheusRuleCR(logserverIdent+"-default.rules", r.ns)
	desiredLsPromRule.Spec.Groups = append(desiredLsPromRule.Spec.Groups, lsDiskRuleGroup)

	var checksumable string
	for _, group := range desiredLsPromRule.Spec.Groups {
		for _, rule := range group.Rules {
			checksumable += sfmonitoring.MkAlertRuleChecksumString(rule)
		}
	}

	// add annotations so we can handle lifecycle
	annotations := map[string]string{
		"version":       "2",
		"rulesChecksum": utils.Checksum([]byte(checksumable)),
	}
	// delete badly named, previous rule - TODO remove this after next release
	badPromRule := monitoringv1.PrometheusRule{}
	if r.GetM(logserverIdent+".rules", &badPromRule) {
		r.DeleteR(&badPromRule)
	}

	desiredLsPromRule.ObjectMeta.Annotations = annotations
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredLsPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredLsPromRule)
		return false
	} else {
		if !utils.MapEquals(&currentPromRule.ObjectMeta.Annotations, &annotations) {
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

	r.EnsureSSHKeySecret(logserverIdent + "-keys")

	cmData := make(map[string]string)
	cmData["logserver.conf"], _ = utils.ParseString(logserverConf, struct {
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
	dep := base.MkDeployment(logserverIdent, r.ns, HTTPDImage)

	// Setup the main container
	dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	// NOTE: Currently we are providing "ReadWriteOnce" access mode
	// for the Logserver, due "ReadWriteMany" requires special storage backend.
	dataPvc := base.MkPVC(logserverIdent, r.ns, BaseGetStorageConfOrDefault(
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
					DefaultMode: &utils.Execmod,
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
		base.MkContainerPort(httpdPort, httpdPortName),
	}

	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = base.MkReadinessHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].StartupProbe = base.MkStartupHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].LivenessProbe = base.MkLiveHTTPProbe("/", httpdPort)
	dep.Spec.Template.Spec.Containers[0].Command = []string{
		"/usr/bin/" + lgEntryScriptName,
	}

	// Create services exposed by logserver
	servicePorts := []int32{httpdPort}
	httpdService := base.MkService(
		httpdPortName, r.ns, logserverIdent, servicePorts, httpdPortName)
	r.GetOrCreate(&httpdService)

	// Setup the sidecar container for sshd
	sshdContainer := base.MkContainer(sshdPortName, sshdImage)
	sshdContainer.Command = []string{"bash", "/conf/run.sh"}
	sshdContainer.Env = []apiv1.EnvVar{
		base.MkEnvVar("AUTHORIZED_KEY", r.cr.Spec.AuthorizedSSHKey),
	}
	sshdContainer.VolumeMounts = []apiv1.VolumeMount{
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
	sshdContainer.Ports = []apiv1.ContainerPort{
		base.MkContainerPort(sshdPort, sshdPortName),
	}

	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, sshdContainer)

	loopdelay, retentiondays := getLogserverSettingsOrDefault(r.cr.Spec.Settings)

	purgelogsContainer := base.MkContainer(purgelogIdent, purgeLogsImage)
	purgelogsContainer.Command = []string{
		"/usr/local/bin/purgelogs",
		"--retention-days",
		strconv.Itoa(retentiondays),
		"--loop", strconv.Itoa(loopdelay),
		"--log-path-dir",
		purgelogsLogsDir,
		"--debug"}
	purgelogsContainer.VolumeMounts = []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: purgelogsLogsDir,
		},
	}

	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, purgelogsContainer)

	// add volumestats exporter
	volumeMountsStatsExporter := []apiv1.VolumeMount{
		{
			Name:      logserverIdent,
			MountPath: "/home/data/rsync",
		},
	}

	statsExporter := sfmonitoring.MkNodeExporterSideCarContainer(logserverIdent, volumeMountsStatsExporter)
	dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, statsExporter)

	// Increase serial each time you need to enforce a deployment change/pod restart between operator versions
	annotations := map[string]string{
		"fqdn":           r.cr.Spec.FQDN,
		"serial":         "3",
		"httpd-conf":     utils.Checksum([]byte(logserverConf)),
		"purgeLogConfig": "retentionDays:" + strconv.Itoa(retentiondays) + " loopDelay:" + strconv.Itoa(loopdelay),
	}

	// do we have an existing deployment?
	currentDep := v1.Deployment{}
	deploymentUpdated := true
	if r.GetM(dep.GetName(), &currentDep) {
		// Are annotations in sync?
		if !utils.MapEquals(&currentDep.Spec.Template.ObjectMeta.Annotations, &annotations) {
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
	sshdService := base.MkService(sshdPortName, r.ns, logserverIdent, sshdServicePorts, sshdPortName)
	r.GetOrCreate(&sshdService)

	nodeExporterSidecarService := sfmonitoring.MkNodeExporterSideCarService(logserverIdent, r.ns)
	r.GetOrCreate(&nodeExporterSidecarService)

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
	conds.UpdateConditions(&r.cr.Status.Conditions, logserverIdent, isDeploymentReady)

	return sfv1.LogServerStatus{
		Ready:              deploymentUpdated && isDeploymentReady && pvcReadiness && routeReady,
		ObservedGeneration: r.cr.Generation,
		ReconciledBy:       conds.GetOperatorConditionName(),
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
