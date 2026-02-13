package controllers

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"
	cliapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
)

const OpenshiftPodsKubeContext = "openshiftpods"

func (r *SFKubeContext) EnsureSARole(saName string) error {
	var role rbacv1.Role
	var roleBinding rbacv1.RoleBinding

	roleName := saName + "-role"
	if !r.GetOrDie(roleName, &role) {
		role.Name = roleName
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/exec", "pods/portforward", "services", "persistentvolumeclaims", "configmaps", "secrets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		}
		r.CreateROrDie(&role)
	}

	rbName := saName + "-rb"
	if !r.GetOrDie(rbName, &roleBinding) {
		roleBinding.Name = rbName
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: saName,
			},
		}
		roleBinding.RoleRef.Kind = "Role"
		roleBinding.RoleRef.Name = roleName
		roleBinding.RoleRef.APIGroup = "rbac.authorization.k8s.io"
		r.CreateROrDie(&roleBinding)
	}
	return nil
}

func (r *SFKubeContext) EnsureSAToken(saName string) string {
	var secret apiv1.Secret
	tokenName := saName + "-token"
	if !r.GetOrDie(tokenName, &secret) {
		secret.Name = tokenName
		secret.ObjectMeta.Annotations = map[string]string{
			"kubernetes.io/service-account.name": saName,
		}
		secret.Type = "kubernetes.io/service-account-token"
		r.CreateROrDie(&secret)
	}
	var token []byte
	for retry := 1; retry < 20; retry++ {
		token = secret.Data["token"]
		if token != nil {
			break
		}
		time.Sleep(time.Second)
		r.GetOrDie(tokenName, &secret)
	}
	if token == nil {
		ctrl.Log.Error(errors.New("query timeout"), "Error getting nodepool service account token")
		os.Exit(1)
	}
	return string(token)
}

func (r *SFKubeContext) CreateKubeConfigOrDie(saName string, token string) cliapi.Config {
	currentConfig := r.RESTConfig
	if strings.HasPrefix(currentConfig.Host, "https://localhost") || strings.HasPrefix(currentConfig.Host, "https://127.") {
		ctrl.Log.Error(
			errors.New("invalid config host address"),
			"The server address of the context used by nodepool cannot be \"localhost\" and must be resolvable from nodepool's pod.",
		)
		os.Exit(1)
	}
	return cliapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*cliapi.Cluster{
			"OpenshiftPodsCluster": {
				// APIPath is selected by client-go for individual API requests. A
				// kubeconfig server must contain only the cluster's base URL.
				Server:                   currentConfig.Host + currentConfig.APIPath,
				CertificateAuthorityData: currentConfig.TLSClientConfig.CAData,
			},
		},
		Contexts: map[string]*cliapi.Context{
			OpenshiftPodsKubeContext: {
				Cluster:   "OpenshiftPodsCluster",
				Namespace: r.Ns,
				AuthInfo:  saName,
			},
		},
		CurrentContext: OpenshiftPodsKubeContext,
		AuthInfos: map[string]*cliapi.AuthInfo{
			saName: {
				Token: token,
			},
		},
	}
}

func (r *SFKubeContext) ensureKubeProvidersSecrets(kubeconfig []byte) {
	var secret apiv1.Secret
	if !r.GetOrDie(NodepoolProvidersSecretsName, &secret) {
		// Initialize the secret data
		secret.Name = NodepoolProvidersSecretsName
		secret.Data = make(map[string][]byte)
		if kubeconfig != nil {
			secret.Data["kube.config"] = kubeconfig
		}
		r.CreateROrDie(&secret)
	} else {
		// Handle secret update
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		needUpdate := false
		if kubeconfig != nil {
			if !bytes.Equal(secret.Data["kube.config"], kubeconfig) {
				ctrl.Log.Info("Updating the kube config ...")
				secret.Data["kube.config"] = kubeconfig
				needUpdate = true
			}
		} else {
			if _, ok := secret.Data["kube.config"]; ok {
				ctrl.Log.Info("Removing the kube config ...")
				delete(secret.Data, "kube.config")
				needUpdate = true
			}
		}
		if needUpdate {
			r.UpdateROrDie(&secret)
		} else {
			ctrl.Log.Info("Secret \"" + NodepoolProvidersSecretsName + "\" already up to date, doing nothing")
		}
	}
}

func (r *SFKubeContext) SetupK8SProvider(providerEnv *SFKubeContext) error {
	providerEnv.EnsureNamespaceOrDie(providerEnv.Ns)
	providerEnv.EnsureServiceAccountOrDie("zuul-launcher-sa")
	providerEnv.EnsureSARole("zuul-launcher-sa")
	token := providerEnv.EnsureSAToken("zuul-launcher-sa")
	config := providerEnv.CreateKubeConfigOrDie("zuul-launcher-sa", token)
	kconfig, err := clientcmd.Write(config)
	if err != nil {
		ctrl.Log.Error(err, "Could not serialize nodepool's kubeconfig")
		return err
	}
	r.ensureKubeProvidersSecrets(kconfig)
	return nil
}
