// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the config job configuration.

package controllers

import (
	_ "embed"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed static/sf_operator/secret.py
var pymodSecret string

//go:embed static/sf_operator/main.py
var pymodMain string

// SetupBaseSecrets returns true when the Job that set the zuul secret in the system-config repository is done
func (r *SFController) SetupBaseSecrets(internalTenantSecretsVersion string) bool {

	serviceAccountName := "config-updater"
	serviceAccount := apiv1.ServiceAccount{}
	if !r.GetM(serviceAccountName, &serviceAccount) {
		serviceAccount.SetNamespace(r.ns)
		serviceAccount.Name = serviceAccountName
		r.CreateR(&serviceAccount)
	}

	roleAnnotations := map[string]string{
		"serial": "3",
	}

	roleName := "config-updater-role"
	roleRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets"},
			Verbs:     []string{"get", "list"},
		},
	}

	currentRole := rbacv1.Role{}
	if !r.GetM(roleName, &currentRole) {
		currentRole.SetNamespace(r.ns)
		currentRole.Name = roleName
		currentRole.Annotations = roleAnnotations
		currentRole.Rules = roleRules
		r.CreateR(&currentRole)
	} else {
		if !utils.MapEquals(&currentRole.Annotations, &roleAnnotations) {
			currentRole.Rules = roleRules
			currentRole.Annotations = roleAnnotations
			if !r.UpdateR(&currentRole) {
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
	secretName := "config-update-secrets"
	if !r.GetM(secretName, &secret) {
		utils.LogI("Creating the config-update service account secret")
		secret = apiv1.Secret{
			Type: "kubernetes.io/service-account-token",
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": "config-updater"},
				Namespace: r.ns,
			},
		}
		r.CreateR(&secret)
	}

	var job batchv1.Job
	// We need to run a new job whenever the version of the secrets changed
	// thus we include the version of the secrets
	jobName := "config-base-secret-" + internalTenantSecretsVersion
	found := r.GetM(jobName, &job)

	extraCmdVars := []apiv1.EnvVar{
		base.MkEnvVar("HOME", "/tmp"),
		base.MkSecretEnvVar("SERVICE_ACCOUNT_TOKEN", secretName, "token"),
		base.MkSecretEnvVar("ZUUL_LOGSERVER_PRIVATE_KEY", "zuul-ssh-key", "priv"),
	}

	if r.cr.Spec.ConfigRepositoryLocation.ClusterAPIURL != "" {
		extraCmdVars = append(extraCmdVars, []apiv1.EnvVar{
			base.MkEnvVar("KUBERNETES_PUBLIC_API_URL", r.cr.Spec.ConfigRepositoryLocation.ClusterAPIURL),
		}...)
	}

	if r.cr.Spec.ConfigRepositoryLocation.LogserverHost != "" {
		extraCmdVars = append(extraCmdVars, []apiv1.EnvVar{
			base.MkEnvVar("ZUUL_LOGSERVER_HOST", r.cr.Spec.ConfigRepositoryLocation.LogserverHost),
		}...)
	}

	if !found {
		utils.LogI("Creating base secret job")
		r.CreateR(r.RunCommand(jobName, []string{"config-create-zuul-secrets"}, extraCmdVars))
		return false
	} else if job.Status.Succeeded >= 1 {
		return true
	} else {
		utils.LogI("Waiting for base secret job result")
		return false
	}
}

func (r *SFController) RunCommand(name string, args []string, extraVars []apiv1.EnvVar) *batchv1.Job {
	jobContainer := base.MkContainer("sf-operator", base.BusyboxImage())
	jobContainer.Command = append([]string{"python3", "/sf_operator/main.py"}, args...)
	jobContainer.Env = append([]apiv1.EnvVar{
		base.MkEnvVar("PYTHONPATH", "/"),
		base.MkEnvVar("FQDN", r.cr.Spec.FQDN),
	}, extraVars...)
	jobContainer.VolumeMounts = []apiv1.VolumeMount{
		{Name: "pymod-sf-operator", MountPath: "/sf_operator"},
	}
	job := base.MkJob(name, r.ns, jobContainer, r.cr.Spec.ExtraLabels)
	job.Spec.Template.Spec.Volumes = []apiv1.Volume{
		base.MkVolumeCM("pymod-sf-operator", "pymod-sf-operator-config-map"),
	}
	return &job
}

func (r *SFController) InstallTooling() {
	r.EnsureConfigMap("pymod-sf-operator", map[string]string{
		"secret.py": pymodSecret,
		"main.py":   pymodMain,
	})
}

func (r *SFController) SetupConfigJob() bool {

	// Get the resource version of the keystore password
	zkp := apiv1.Secret{}
	r.GetM("zuul-keystore-password", &zkp)

	// This ensure we trigger the base secret creation job when the setting change
	extraSettingsChecksum := "ns"
	if r.cr.Spec.ConfigRepositoryLocation.ClusterAPIURL != "" || r.cr.Spec.ConfigRepositoryLocation.LogserverHost != "" {
		extraSettingsChecksum = utils.Checksum([]byte(
			r.cr.Spec.ConfigRepositoryLocation.ClusterAPIURL + r.cr.Spec.ConfigRepositoryLocation.LogserverHost))[0:5]
	}

	var (
		// We use the CM to store versions that can trigger internal tenant secrets update
		// or zuul internal tenant reconfigure
		cmName                       = "zs-internal-tenant-reconfigure"
		zsInternalTenantReconfigure  apiv1.ConfigMap
		configHash                   = utils.Checksum([]byte(r.MkPreInitScript()))
		internalTenantSecretsVersion = "1" + "-" + zkp.ResourceVersion + "-" + extraSettingsChecksum
		needReconfigureTenant        = false
		needCMUpdate                 = false
	)

	// Ensure that toolings are available in the ConfigMap
	r.InstallTooling()

	// Get the internal tenant version CM and evaluate if we need to trigger actions
	if !r.GetM(cmName, &zsInternalTenantReconfigure) {
		needReconfigureTenant = true
	} else {
		if configHash != zsInternalTenantReconfigure.Data["internal-tenant-config-hash"] ||
			internalTenantSecretsVersion != zsInternalTenantReconfigure.Data["internal-tenant-secrets-version"] {
			needReconfigureTenant = true
			needCMUpdate = true
		}
	}

	// We ensure that base secrets are set in the system-config repository
	baseSecretsInstalled := r.SetupBaseSecrets(internalTenantSecretsVersion)

	if baseSecretsInstalled {
		// We run zuul tenant-reconfigure for the 'internal' tenant, when:
		// - the configMap does not exists (or)
		// - tenant config changed
		// - tenant secrets version changed
		// This ensures that the zuul-scheduler loaded the provisionned Zuul config
		// for the 'internal' tenant
		if needReconfigureTenant {
			utils.LogI("Running tenant-reconfigure for the 'internal' tenant")
			if r.runZuulInternalTenantReconfigure() {
				// Create an empty ConfigMap to keep note the reconfigure has been already done
				zsInternalTenantReconfigure.ObjectMeta = metav1.ObjectMeta{
					Name:      cmName,
					Namespace: r.ns,
				}
				zsInternalTenantReconfigure.Data = map[string]string{
					"internal-tenant-config-hash":     configHash,
					"internal-tenant-secrets-version": internalTenantSecretsVersion,
				}
				if needCMUpdate {
					r.UpdateR(&zsInternalTenantReconfigure)
				} else {
					r.CreateR(&zsInternalTenantReconfigure)
				}
			}
			return false
		} else {
			return true
		}
	}
	return false
}
