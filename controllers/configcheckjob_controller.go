// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

// ConfigCheckJobReconciler reconciles a ConfigCheckJob object
type ConfigCheckJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const ZUUL_SERVICE_JOB_CONTAINER_NAME string = "zuul-config-check"

func getCheckContainerNames() []string {
	return []string{
		ZUUL_SERVICE_JOB_CONTAINER_NAME,
		ZUUL_SERVICE_JOB_CONTAINER_NAME + "-init",
	}
}

func getPodMatchLabels(name string) map[string]string {
	return map[string]string{
		"app":  "sf",
		"job":  "config-check",
		"name": name,
	}
}

func (r *ConfigCheckJobReconciler) getJobPodID(ctx context.Context, name string) string {
	var podList apiv1.PodList
	matchLabels := getPodMatchLabels(name)
	labels := labels.SelectorFromSet(labels.Set(matchLabels))
	labelSelectors := client.MatchingLabelsSelector{Selector: labels}
	if err := r.Client.List(ctx, &podList, labelSelectors); err == nil {
		for _, pod := range podList.Items {
			// return first pod found
			return pod.ObjectMeta.Name
		}
	}
	return ""
}

func getPodFailurePolicy() batchv1.PodFailurePolicy {
	// Pod will only fail on errors unrelated to tenant-conf-check's exit code: quota errors, provisioning, etc
	var rules []batchv1.PodFailurePolicyRule
	for _, name := range getCheckContainerNames() {
		rule := batchv1.PodFailurePolicyRule{
			Action: batchv1.PodFailurePolicyActionFailJob,
			OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: &name,
				Operator:      batchv1.PodFailurePolicyOnExitCodesOpNotIn,
				Values: []int32{
					0,
				},
			},
		}
		rules = append(rules, rule)
	}
	podFailurePolicy := batchv1.PodFailurePolicy{
		Rules: rules,
	}
	return podFailurePolicy
}

func (r *ConfigCheckJobReconciler) getConfigLocationSpec(ctx context.Context) sfv1.ConfigLocationSpec {

	var sfList sfv1.SoftwareFactoryList
	if err := r.Client.List(ctx, &sfList); err != nil {
		// If there is no SF why are we even doing this?
		panic(err)
	}
	// TODO we need to support multiple SF instances in the namespace maybe?
	var sf = sfList.Items[0]
	return sf.Spec.ConfigLocation
}

func getZuulConfigInitEnvVars(cl sfv1.ConfigLocationSpec) []apiv1.EnvVar {

	return []apiv1.EnvVar{
		Create_env("CONFIG_REPO_NAME", cl.Name),
		Create_env("CONFIG_REPO_SET", "TRUE"),
		Create_env("CONFIG_REPO_CONNECTION_NAME", cl.ZuulConnectionName),
		Create_env("HOME", "/var/lib/zuul"),
	}
}

func (r *ConfigCheckJobReconciler) addZuulConfigCheckContainers(ctx context.Context, req ctrl.Request, cr sfv1.ConfigCheckJob, jobSpec batchv1.JobSpec) batchv1.JobSpec {
	tenantConfig := ""
	if cr.Spec.ZuulTenantConfig != "" {
		tenantConfig = cr.Spec.ZuulTenantConfig
	}

	// Create a ConfigMap holding the tenant config to test.
	// TODO configMaps have a capacity limit of 1Mi due to etcd.
	// Our largest zuul tenant config file in prod is about 200Ki.
	// We may want to find a workaround for this size limit.
	zuul_config_cm_name := cr.Name + "-zuul-cfg"
	cm := apiv1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: zuul_config_cm_name, Namespace: req.Namespace}, &cm)
	if err != nil && errors.IsNotFound(err) {
		cm := apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: zuul_config_cm_name, Namespace: req.Namespace},
			Data: map[string]string{
				"main.yaml": tenantConfig,
			},
		}
		ctrl.SetControllerReference(&cr, &cm, r.Scheme)
		r.Client.Create(ctx, &cm)
	}

	// Inject generate-tenant-config.sh into the Init container.
	// TODO this is created by the zuul controller, maybe this could be refactored
	zuul_tooling_volume := apiv1.Volume{
		Name: "tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "zuul-scheduler-tooling-config-map",
				},
				DefaultMode: &Execmod,
			},
		},
	}

	// Use memory storage to hold the concatenated tenant config.
	zuul_main_config_dir := apiv1.Volume{
		Name: "zuul-main-config",
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{
				Medium: apiv1.StorageMediumMemory,
			},
		},
	}

	cl := r.getConfigLocationSpec(ctx)

	// This container is used to concatenate the internal tenant config with the "public" config.
	zuulInitContainer := apiv1.Container{
		Name:    ZUUL_SERVICE_JOB_CONTAINER_NAME + "-init",
		Image:   BUSYBOX_IMAGE,
		Command: []string{"/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "zuul-main-config",
				MountPath: "/var/lib/zuul",
			},
			{
				Name:      zuul_config_cm_name,
				MountPath: "/var/lib/zuul/" + cl.Name + "/zuul",
			},
			{
				Name:      "tooling-vol",
				SubPath:   "generate-zuul-tenant-yaml.sh",
				MountPath: "/usr/local/bin/generate-zuul-tenant-yaml.sh"},
		},
		Env: getZuulConfigInitEnvVars(cl),
	}
	// The actual tenant config check
	zuulContainer := apiv1.Container{
		Name:    ZUUL_SERVICE_JOB_CONTAINER_NAME,
		Image:   Zuul_Image("zuul-scheduler"),
		Command: []string{"zuul-admin", "tenant-conf-check"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "zuul-config",
				MountPath: "/etc/zuul",
				ReadOnly:  true,
			},
			{
				Name:      "zuul-main-config",
				MountPath: "/var/lib/zuul",
			},
		},
	}

	zuulVolumes := []apiv1.Volume{
		// TODO Check if volume exists, if there is no SF there is no point in creating this CR
		create_volume_secret("zuul-config"),
		Create_volume_cm(zuul_config_cm_name, zuul_config_cm_name),
		zuul_main_config_dir,
		zuul_tooling_volume,
	}

	jobSpec.Template.Spec.InitContainers = append(jobSpec.Template.Spec.InitContainers, zuulInitContainer)
	jobSpec.Template.Spec.Containers = append(jobSpec.Template.Spec.Containers, zuulContainer)
	jobSpec.Template.Spec.Volumes = append(jobSpec.Template.Spec.Volumes, zuulVolumes...)

	return jobSpec
}

//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=configcheckjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=configcheckjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sf.softwarefactory-project.io,resources=configcheckjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ConfigCheckJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(1).Info("Reconciling ConfigCheckJob")

	cr := sfv1.ConfigCheckJob{}

	if err := r.Client.Get(ctx, req.NamespacedName, &cr); err != nil && errors.IsNotFound(err) {
		log.Error(err, "Unable to fetch ConfigCheckJob resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ttl := cr.Spec.TTL

	// How many times the pod should restart in case of failure as defined above
	var backoffLimit int32 = 5

	podFailurePolicy := getPodFailurePolicy()

	specJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: req.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoffLimit,
			PodFailurePolicy:        &podFailurePolicy,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getPodMatchLabels(cr.Name),
				},
				Spec: apiv1.PodSpec{
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}

	specJob.Spec = r.addZuulConfigCheckContainers(ctx, req, cr, specJob.Spec)

	// This annotation is used to keep track of the creation of the underlying batch job.
	// If we don't do that, the controller would recreate a new batch job indefinitely as
	// soon as the previous one gets deleted by reaching its TTL.
	init_annotations := map[string]string{
		"initialized": "true",
	}

	currentJob := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      cr.Name,
		Namespace: req.Namespace,
	}, currentJob); err != nil {
		// Only create the batch job if it was not created before, ie the annotation isn't there
		if !map_equals(&cr.ObjectMeta.Annotations, &init_annotations) {
			ctrl.SetControllerReference(&cr, &specJob, r.Scheme)
			if er := r.Client.Create(ctx, &specJob); er != nil {
				panic(er.Error())
			} else {
				log.V(1).Info("ConfigCheckJob created")
				cr.ObjectMeta.Annotations = init_annotations
				if err := r.Client.Update(ctx, &cr); err != nil {
					log.Error(err, "unable to update ConfigCheckJob")
					return ctrl.Result{}, err
				}
			}
		}
	}

	// refresh currentJob object
	if ref_err := r.Get(ctx, client.ObjectKey{
		Name:      cr.Name,
		Namespace: req.Namespace,
	}, currentJob); ref_err == nil {
		cr.Status = sfv1.ConfigCheckJobStatus{
			Ready:   *currentJob.Spec.Completions > 0,
			Outcome: "PENDING",
		}

		if !currentJob.Status.StartTime.IsZero() {
			cr.Status.StartTime = *currentJob.Status.StartTime
		}
		// CompletionTime is only set in case of success ¯\_(°_^)_/¯
		if !currentJob.Status.CompletionTime.IsZero() {
			cr.Status.CompletionTime = *currentJob.Status.CompletionTime
		} else {
			if currentJob.Status.Failed == 1 && cr.Status.CompletionTime.IsZero() {
				cr.Status.CompletionTime = metav1.Now()
			}
		}
		if !cr.Status.CompletionTime.IsZero() {
			cr.Status.PodID = r.getJobPodID(ctx, cr.Name)
		}
		if currentJob.Status.Failed > 0 {
			cr.Status.Outcome = "FAILURE"
		}
		if currentJob.Status.Succeeded > 0 {
			cr.Status.Outcome = "SUCCESS"
		}

		if err := r.Client.Status().Update(ctx, &cr); err != nil {
			log.Error(err, "unable to update ConfigCheckJob status")
			return ctrl.Result{}, err
		}

	}

	ttlDuration := time.Duration(ttl) * time.Second

	if !cr.Status.Ready {
		log.V(1).Info("ConfigCheckJob " + cr.Name + " is still running ...")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	// TODO The TTL countdown should probably start with StartTime,
	// as CompletionTime might never be set
	if !cr.Status.CompletionTime.IsZero() {
		now := metav1.Now()
		maxLife := cr.Status.CompletionTime.Add(ttlDuration)
		if maxLife.Before(now.Time) {
			if del_err := r.Client.Delete(ctx, &cr); del_err != nil {
				log.V(1).Info("Deletion of "+cr.Name+" failed", del_err)
				return ctrl.Result{}, del_err
			} else {
				log.V(1).Info("ConfigCheckJob " + cr.Name + " deleted")
			}
		} else {
			log.V(1).Info("Waiting for scheduled deletion ...")
			return ctrl.Result{RequeueAfter: ttlDuration}, nil
		}
	} else {
		// Waiting for a completion time
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	log.V(1).Info("End of reconcile loop")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigCheckJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfv1.ConfigCheckJob{}).
		Complete(r)
}
