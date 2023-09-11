// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/sf_operator/secret.py
var pymod_secret string

//go:embed static/sf_operator/main.py
var pymod_main string

// Returns true when the Job that set the zuul secret in the system-config repository is done
func (r *SFController) SetupBaseSecrets() bool {

	serviceAccountName := "config-updater"
	service_account := apiv1.ServiceAccount{}
	if !r.GetM(serviceAccountName, &service_account) {
		service_account.SetNamespace(r.ns)
		service_account.Name = serviceAccountName
		r.CreateR(&service_account)
	}

	roleAnnotations := map[string]string{
		"serial": "1",
	}

	roleName := "config-updater-role"
	roleRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "pods/exec", "services", "persistentvolumeclaims", "configmaps", "secrets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
	}

	current_role := rbacv1.Role{}
	if !r.GetM(roleName, &current_role) {
		current_role.SetNamespace(r.ns)
		current_role.Name = roleName
		current_role.Annotations = roleAnnotations
		current_role.Rules = roleRules
		r.CreateR(&current_role)
	} else {
		if !map_equals(&current_role.Annotations, &roleAnnotations) {
			current_role.Rules = roleRules
			current_role.Annotations = roleAnnotations
			if !r.UpdateR(&current_role) {
				return false
			}
		}
	}

	roleBindingName := serviceAccountName
	rb := rbacv1.RoleBinding{}
	if !r.GetM(roleBindingName, &rb) {
		rb.SetNamespace(r.ns)
		rb.Name = roleBindingName
		rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: serviceAccountName}}
		rb.RoleRef.Kind = "Role"
		rb.RoleRef.Name = roleName
		rb.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		r.CreateR(&rb)
	}

	// Create a long lived service account token for the use within the
	// config-update process
	// See: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account
	var secret apiv1.Secret
	secret_name := "config-update-secrets"
	if !r.GetM(secret_name, &secret) {
		r.log.V(1).Info("Creating the config-update service account secret")
		secret = apiv1.Secret{
			Type: "kubernetes.io/service-account-token",
			ObjectMeta: metav1.ObjectMeta{
				Name: secret_name,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": "config-updater"},
				Namespace: r.ns,
			},
		}
		r.CreateR(&secret)
	}

	var job batchv1.Job
	job_name := "config-base-secret"
	found := r.GetM(job_name, &job)

	extra_cmd_vars := []apiv1.EnvVar{
		Create_env("HOME", "/tmp"),
		Create_secret_env("SERVICE_ACCOUNT_TOKEN", secret_name, "token"),
		Create_secret_env("ZUUL_LOGSERVER_PRIVATE_KEY", "zuul-ssh-key", "priv"),
	}

	if !found {
		r.log.V(1).Info("Creating base secret job")
		r.CreateR(r.RunCommand(job_name, []string{"config-create-zuul-secrets"}, extra_cmd_vars))
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		r.log.V(1).Info("Waiting for base secret job result")
		return false
	}
}

func (r *SFController) RunCommand(name string, args []string, extra_vars []apiv1.EnvVar) *batchv1.Job {
	job := r.create_job(
		name,
		apiv1.Container{
			Name:    "sf-operator",
			Image:   BUSYBOX_IMAGE,
			Command: append([]string{"python3", "/sf_operator/main.py"}, args...),
			Env: append([]apiv1.EnvVar{
				Create_env("PYTHONPATH", "/"),
				Create_env("FQDN", r.cr.Spec.FQDN),
			}, extra_vars...),
			VolumeMounts: []apiv1.VolumeMount{
				{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
			},
			SecurityContext: create_security_context(false),
		},
	)
	job.Spec.Template.Spec.Volumes = []apiv1.Volume{
		Create_volume_cm("pymod-sf-operator", "pymod-sf-operator-config-map"),
	}
	return &job
}

func (r *SFController) InstallTooling() {
	r.EnsureConfigMap("pymod-sf-operator", map[string]string{
		"secret.py": pymod_secret,
		"main.py":   pymod_main,
	})
}

func (r *SFController) SetupConfigJob() bool {
	r.InstallTooling()
	// We ensure that base secrets are set in the system-config repository
	if r.SetupBaseSecrets() {
		// We run zuul tenant-reconfigure for the 'internal' tenant, when:
		// - the configMap does not exists (or)
		// - tenant config changed
		// This ensures that the zuul-scheduler loaded the provisionned Zuul config
		// for the 'internal' tenant
		var zs_internal_tenant_reconfigure apiv1.ConfigMap
		var cm_name = "zs-internal-tenant-reconfigure"
		var needReconfigureTenant = false
		var configHash = checksum([]byte(preInitScriptTemplate))
		if !r.GetM(cm_name, &zs_internal_tenant_reconfigure) {
			needReconfigureTenant = true
		} else {
			if configHash != zs_internal_tenant_reconfigure.Data["internal-tenant-config-hash"] {
				needReconfigureTenant = true
			}
		}
		if needReconfigureTenant {
			r.log.Info("Running tenant-reconfigure for the 'internal' tenant")
			if r.runZuulInternalTenantReconfigure() {
				// Create an empty ConfigMap to keep note the reconfigure has been already done
				zs_internal_tenant_reconfigure.ObjectMeta = metav1.ObjectMeta{
					Name:      cm_name,
					Namespace: r.ns,
				}
				zs_internal_tenant_reconfigure.Data = map[string]string{
					"internal-tenant-config-hash": configHash,
				}
				r.CreateR(&zs_internal_tenant_reconfigure)
			}
			return false
		} else {
			return true
		}
	}
	return false
}
