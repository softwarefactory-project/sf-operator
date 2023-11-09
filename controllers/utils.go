// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/cert"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	"github.com/go-logr/logr"
	apiroutev1 "github.com/openshift/api/route/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const BusyboxImage = "quay.io/software-factory/sf-op-busybox:1.5-3"

// HTTPDImage uses pinned/ubi8 based image for httpd
// https://catalog.redhat.com/software/containers/ubi8/httpd-24/6065b844aee24f523c207943?q=httpd&architecture=amd64&image=651f274c8ce9242f7bb3e011
const HTTPDImage = "registry.access.redhat.com/ubi8/httpd-24:1-284.1696531168"

type SFUtilContext struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
	ns         string
	log        logr.Logger
	ctx        context.Context
	owner      client.Object
	standalone bool
}

// --- API Interact primitive functions ---

// setOwnerReference set the Owner of a resources
// Whether we are running the controller or standalone mode the owneship must
// be managed differently
func (r *SFUtilContext) setOwnerReference(controlled metav1.Object) error {
	var err error
	if r.standalone {
		err = controllerutil.SetOwnerReference(r.owner, controlled, r.Scheme)
	} else {
		err = controllerutil.SetControllerReference(r.owner, controlled, r.Scheme)
	}
	if err != nil {
		r.log.Error(err, "Unable to set controller reference", "name", controlled.GetName())

	}
	return err
}

// GetM gets a resource, returning if it was found
func (r *SFUtilContext) GetM(name string, obj client.Object) bool {
	err := r.Client.Get(r.ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: r.ns,
		}, obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(err.Error())
	}
	return true
}

// CreateR creates a resource with the owner as the ownerReferences.
func (r *SFUtilContext) CreateR(obj client.Object) {
	r.setOwnerReference(obj)
	if err := r.Client.Create(r.ctx, obj); err != nil && !errors.IsAlreadyExists(err) {
		panic(err.Error())
	}
}

// DeleteR delete a resource.
func (r *SFUtilContext) DeleteR(obj client.Object) {
	if err := r.Client.Delete(r.ctx, obj); err != nil && !errors.IsNotFound(err) {
		panic(err.Error())
	}
}

// UpdateR updates resource with the owner as the ownerReferences.
func (r *SFUtilContext) UpdateR(obj client.Object) bool {
	r.setOwnerReference(obj)
	r.log.V(1).Info("Updating object", "name", obj.GetName())
	if err := r.Client.Update(r.ctx, obj); err != nil {
		r.log.Error(err, "Unable to update the object")
		return false
	}
	return true
}

// PatchR delete a resource.
func (r *SFUtilContext) PatchR(obj client.Object, patch client.Patch) {
	if err := r.Client.Patch(r.ctx, obj, patch); err != nil {
		panic(err.Error())
	}
}

// GetOrCreate does not change an existing object, update needs to be used manually.
// In the case the object already exists then the function return True
func (r *SFUtilContext) GetOrCreate(obj client.Object) bool {
	name := obj.GetName()

	if !r.GetM(name, obj) {
		r.log.V(1).Info("Creating object", "name", obj.GetName())
		r.CreateR(obj)
		return false
	}
	return true
}

// PodExec connects to a container's Pod and execute a command
// Stdout and Stderr is output on the caller's Stdout
// The function returns an Error for any issue
func (r *SFUtilContext) PodExec(pod string, container string, command []string) error {
	r.log.V(1).Info("Running pod execution", "pod", pod, "command", command)
	execReq := r.RESTClient.
		Post().
		Namespace(r.ns).
		Resource("pods").
		Name(pod).
		SubResource("exec").
		VersionedParams(&apiv1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(r.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", execReq.URL())
	if err != nil {
		return err
	}

	// r.log.V(1).Info("Streaming start")
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	return nil
}

// --- Ensure resources functions ---

// EnsureConfigMap ensures a config map exist
// The ConfigMap is updated if needed
func (r *SFUtilContext) EnsureConfigMap(baseName string, data map[string]string) apiv1.ConfigMap {
	name := baseName + "-config-map"
	var cm apiv1.ConfigMap
	if !r.GetM(name, &cm) {
		r.log.V(1).Info("Creating config", "name", name)
		cm = apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns},
			Data:       data,
		}
		r.CreateR(&cm)
	} else {
		if !reflect.DeepEqual(cm.Data, data) {
			r.log.V(1).Info("Updating config", "name", name)
			cm.Data = data
			r.UpdateR(&cm)
		}
	}
	return cm
}

// EnsureSecret ensures a Secret exist
// The Secret is updated if needed
func (r *SFUtilContext) EnsureSecret(secret *apiv1.Secret) {
	var current apiv1.Secret
	name := secret.GetName()
	if !r.GetM(name, &current) {
		r.log.V(1).Info("Creating secret", "name", name)
		r.CreateR(secret)
	} else {
		if !reflect.DeepEqual(current.Data, secret.Data) {
			r.log.V(1).Info("Updating secret", "name", name)
			current.Data = secret.Data
			r.UpdateR(&current)
		}
	}
}

// ensureSecretFromFunc ensure a Secret exists
// If it does not the Secret is created from the getData function
// This function does not support Secret update
// This function returns the Secret
func (r *SFUtilContext) ensureSecretFromFunc(name string, getData func() string) apiv1.Secret {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating secret", "name", name)
		secret = base.MkSecretFromFunc(name, r.ns, getData)
		r.CreateR(&secret)
	}
	return secret
}

// EnsureSecretUUID ensures a Secret caontaining an UUID
// This function does not support update
func (r *SFUtilContext) EnsureSecretUUID(name string) apiv1.Secret {
	return r.ensureSecretFromFunc(name, utils.NewUUIDString)
}

// EnsureSSHKeySecret ensures a Secret exists container an autogenerated SSH key pair
// If it does not exixtthe Secret is created
// This function does not support Secret update
func (r *SFUtilContext) EnsureSSHKeySecret(name string) {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating ssh key", "name", name)
		secret := base.MkSSHKeySecret(name, r.ns)
		r.CreateR(&secret)
	}
}

// ensureRoute ensures the Route exist
// The Route is updated if needed
// The function returns false when the resource is just created/updated
func (r *SFUtilContext) ensureRoute(route apiroutev1.Route, name string) bool {
	current := apiroutev1.Route{}
	found := r.GetM(name, &current)
	if !found {
		r.log.V(1).Info("Creating route...", "name", name)
		r.CreateR(&route)
		return false
	} else {
		// Route already exist - check if we need to update the Route
		needUpdate := false

		// First check the route annotations
		if (len(route.Annotations) == 0 && len(current.Annotations) != 0) || (len(route.Annotations) != 0 && len(current.Annotations) == 0) {
			current.Annotations = route.Annotations
			needUpdate = true
		}
		if len(route.Annotations) != 0 && len(current.Annotations) != 0 {
			if !utils.MapEquals(&route.Annotations, &current.Annotations) {
				current.Annotations = route.Annotations
				needUpdate = true
			}
		}

		// Use the String repr of the RouteSpec to compare for Spec changes
		// This comparaison mechanics may fail in case of some Route Spec default values
		// not specified in the wanted version.
		if route.Spec.String() != current.Spec.String() {
			current.Spec = route.Spec
			needUpdate = true
		}

		if needUpdate {
			r.log.V(1).Info("Updating route...", "name", name)
			r.UpdateR(&current)
			return false
		}
	}
	return true
}

// ensureHTTPSRoute ensures a HTTPS enabled Route exist
// The Route is updated if needed
// The function returns false when the controller reconcile loop must be re-triggered because
// the route setting changed.
func (r *SFUtilContext) ensureHTTPSRoute(
	name string, host string, serviceName string, path string,
	port int, annotations map[string]string, fqdn string, le *sfv1.LetsEncryptSpec) bool {

	tlsDataReady := true
	var sslCA, sslCrt, sslKey []byte

	if le == nil {
		// Letsencrypt config has not been set so we check the `customSSLSecretName` Secret
		// for any custom TLS data to setup the Route
		sslCA, sslCrt, sslKey = r.extractStaticTLSFromSecret(name, host)
	} else {
		// Letsencrypt config has been set so we ensure we set a Certificate via the
		// cert-manager Issuer and then we'll setup the Route based on the Certificate's Secret
		tlsDataReady, sslCA, sslCrt, sslKey = r.extractTLSFromLECertificateSecret(name, host, fqdn, *le)
	}

	if !tlsDataReady {
		return false
	}

	var route apiroutev1.Route

	// Checking if there is any content and setting the Route with TLS data from the Secret
	if len(sslCrt) > 0 && len(sslKey) > 0 {
		r.log.V(1).Info("SSL certificate for Route detected", "host", host, "route name", name)
		tls := apiroutev1.TLSConfig{
			InsecureEdgeTerminationPolicy: apiroutev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   apiroutev1.TLSTerminationEdge,
			Certificate:                   string(sslCrt),
			Key:                           string(sslKey),
			CACertificate:                 string(sslCA),
		}
		route = base.MkHTTPSRoute(name, r.ns, host, serviceName, path, port, annotations, fqdn, &tls)
	} else {
		route = base.MkHTTPSRoute(name, r.ns, host, serviceName, path, port, annotations, fqdn, nil)
	}
	return r.ensureRoute(route, name)
}

// EnsureLocalCA ensures cert-manager resources exists to enable of local CA Issuer
// This function does not support update
func (r *SFUtilContext) EnsureLocalCA() {
	// https://cert-manager.io/docs/configuration/selfsigned/#bootstrapping-ca-issuers
	selfSignedIssuer := cert.MkSelfSignedIssuer("selfsigned-issuer", r.ns)
	CAIssuer := cert.MkCAIssuer("ca-issuer", r.ns)
	duration, _ := time.ParseDuration("87600h") // 10y
	commonName := "cacert"
	rootCACertificate := cert.MkBaseCertificate("ca-cert", r.ns, "selfsigned-issuer", []string{"caroot"},
		"ca-cert", true, duration, nil, &commonName, nil)
	r.GetOrCreate(&selfSignedIssuer)
	r.GetOrCreate(&CAIssuer)
	r.GetOrCreate(&rootCACertificate)
}

// --- Functions and Structs below are helper for handle cert-manager / Let's Encrypt ---

// getLetsEncryptServer returns a tuple with the production or statging URL based on sfv1.LetsEncryptSpec
// and the a proposed name for the Issuer.
func getLetsEncryptServer(le sfv1.LetsEncryptSpec) (string, string) {
	var serverURL string
	name := ""
	switch server := le.Server; server {
	case sfv1.LEServerProd:
		serverURL = "https://acme-v02.api.letsencrypt.org/directory"
		name = "cm-le-issuer-production"
	case sfv1.LEServerStaging:
		serverURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
		name = "cm-le-issuer-staging"
	}
	return serverURL, name
}

// This function ensures the cert-manager / Let's Encrypt issuer is created
// Thus function does not support update
func (r *SFUtilContext) ensureLetsEncryptIssuer(le sfv1.LetsEncryptSpec) bool {
	server, name := getLetsEncryptServer(le)
	issuer := cert.MkLetsEncryptIssuer(name, r.ns, server)
	return r.GetOrCreate(&issuer)
}

//----------------------------------------------------------------------------
// --- TODO clean functions below / remove useless code and set doc string ---
//----------------------------------------------------------------------------

func getStorageClassname(storageClassName string) string {
	if storageClassName != "" {
		return storageClassName
	} else {
		return "topolvm-provisioner"
	}
}

// Create a default statefulset.
func (r *SFUtilContext) mkStatefulSet(name string, image string, storageConfig base.StorageConfig, accessMode apiv1.PersistentVolumeAccessMode, nameSuffix ...string) appsv1.StatefulSet {
	serviceName := name
	if nameSuffix != nil {
		serviceName = name + "-" + nameSuffix[0]
	}

	container := base.MkContainer(name, image)
	pvc := base.MkPVC(name, r.ns, storageConfig, accessMode)
	return base.MkStatefulset(name, r.ns, 1, serviceName, container, pvc)
}

// Create a default headless statefulset.
func (r *SFUtilContext) mkHeadlessSatefulSet(
	name string, image string, storageConfig base.StorageConfig,
	accessMode apiv1.PersistentVolumeAccessMode) appsv1.StatefulSet {
	return r.mkStatefulSet(name, image, storageConfig, accessMode, "headless")
}

func (r *SFUtilContext) IsStatefulSetReady(dep *appsv1.StatefulSet) bool {
	if dep.Status.ReadyReplicas > 0 {
		var podList apiv1.PodList
		matchLabels := dep.Spec.Selector.MatchLabels
		labels := labels.SelectorFromSet(labels.Set(matchLabels))
		labelSelectors := client.MatchingLabelsSelector{Selector: labels}
		if err := r.Client.List(r.ctx, &podList, labelSelectors); err != nil {
			panic(err.Error())
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != "Running" {
				r.log.V(1).Info(
					"Waiting for statefulset state: Running",
					"name", dep.GetName(),
					"status", dep.Status)
				return false
			}
			containerStatuses := pod.Status.ContainerStatuses
			for _, containerStatus := range containerStatuses {
				if !containerStatus.Ready {
					r.log.V(1).Info(
						"Waiting for statefulset containers ready",
						"name", dep.GetName(),
						"status", dep.Status,
						"podStatus", pod.Status,
						"containerStatuses", containerStatuses)
					return false
				}
			}
		}
		// All containers in Ready state
		return true && base.IsStatefulSetRolloutDone(dep)
	}
	// No Replica available
	return false
}

func (r *SFUtilContext) IsDeploymentReady(dep *appsv1.Deployment) bool {
	if base.IsDeploymentReady(dep) {
		return true
	}
	r.log.V(1).Info("Waiting for deployment", "name", dep.GetName())
	return false
}

func (r *SFUtilContext) DebugStatefulSet(name string) {
	var dep appsv1.StatefulSet
	if !r.GetM(name, &dep) {
		panic("Can't find the statefulset")
	}
	// Disable probes
	dep.Spec.Template.Spec.Containers[0].ReadinessProbe = nil
	dep.Spec.Template.Spec.Containers[0].LivenessProbe = nil
	// Set sleep command
	dep.Spec.Template.Spec.Containers[0].Command = []string{"sleep", "infinity"}
	r.UpdateR(&dep)
	r.log.V(1).Info("Debugging service", "name", name)
}

func GetCustomRouteSSLSecretName(host string) string {
	return host + "-ssl-cert"
}

func (r *SFUtilContext) extractStaticTLSFromSecret(name string, host string) ([]byte, []byte, []byte) {
	var customSSLSecret apiv1.Secret
	customSSLSecretName := GetCustomRouteSSLSecretName(host)

	if !r.GetM(customSSLSecretName, &customSSLSecret) {
		return nil, nil, nil
	} else {
		// Fetching secret expected TLS Keys content
		return customSSLSecret.Data["CA"], customSSLSecret.Data["crt"], customSSLSecret.Data["key"]
	}
}

func (r *SFUtilContext) extractTLSFromLECertificateSecret(name string, host string, fqdn string, le sfv1.LetsEncryptSpec) (bool, []byte, []byte, []byte) {
	_, issuerName := getLetsEncryptServer(le)
	dnsNames := []string{host + "." + fqdn}
	certificate := cert.MkCertificate(name, r.ns, issuerName, dnsNames, name+"-tls", nil)

	current := certv1.Certificate{}

	found := r.GetM(name, &current)
	if !found {
		r.log.V(1).Info("Creating Cert-Manager LetsEncrypt Certificate ...", "name", name)
		r.CreateR(&certificate)
		return false, nil, nil, nil
	} else {
		if current.Spec.IssuerRef.Name != certificate.Spec.IssuerRef.Name ||
			!reflect.DeepEqual(current.Spec.DNSNames, certificate.Spec.DNSNames) {
			// We need to update the Certficate
			r.log.V(1).Info("Updating Cert-Manager LetsEncrypt Certificate ...", "name", name)
			current.Spec = *certificate.Spec.DeepCopy()
			r.UpdateR(&current)
			return false, nil, nil, nil
		}
		// The certificate is found and have the required Spec, so let's check
		// the Ready status
		ready := cert.IsCertificateReady(&current)

		if ready {
			r.log.V(1).Info("Cert-Manager LetsEncrypt Certificate is Ready ...", "name", name)
			var leSSLSecret apiv1.Secret
			if r.GetM(current.Spec.SecretName, &leSSLSecret) {
				// Extract the TLS material
				return true, nil, leSSLSecret.Data["tls.crt"], leSSLSecret.Data["tls.key"]
				// Nothing more to do the rest of the function will setup the Route's TLS
			} else {
				// We are not able to find the Certificate's secret
				r.log.V(1).Info("Cert-Manager LetsEncrypt Certificate is Ready but waiting for the Secret ...",
					"name", name, "secret", current.Spec.SecretName)
				return false, nil, nil, nil
			}
		} else {
			// Return false to force a new Reconcile as the certificate is not Ready yet
			r.log.V(1).Info("Cert-Manager LetsEncrypt Certificate is not Ready yet ...", "name", name)
			return false, nil, nil, nil
		}
	}
}

// GetSecretbyNameRef Get Secret by Name Reference
func (r *SFUtilContext) GetSecretbyNameRef(name string) (apiv1.Secret, error) {
	var dep apiv1.Secret
	if r.GetM(name, &dep) {
		return dep, nil
	}
	return apiv1.Secret{}, fmt.Errorf("secret name with ref %s not found", name)
}

// GetValueFromKeySecret gets the Value of the Keyname from a Secret
func GetValueFromKeySecret(secret apiv1.Secret, keyname string) ([]byte, error) {
	keyvalue := secret.Data[keyname]
	if len(keyvalue) == 0 {
		return []byte{}, fmt.Errorf("key named %s not found in Secret %s at namespace %s", keyname, secret.Name, secret.Namespace)
	}

	return keyvalue, nil
}

func (r *SFUtilContext) GetSecretDataFromKey(name string, key string) ([]byte, error) {
	secret, err := r.GetSecretbyNameRef(name)
	if err != nil {
		return []byte{}, err
	}
	var subkey string
	if key == "" {
		subkey = name
	} else {
		subkey = key
	}
	data, err := GetValueFromKeySecret(secret, subkey)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Gets Secret Data in which the Keyname is the same as the Secret Name
func (r *SFUtilContext) getSecretData(name string) ([]byte, error) {
	return r.GetSecretDataFromKey(name, "")
}

func BaseGetStorageConfOrDefault(storageSpec sfv1.StorageSpec, storageClassName string) base.StorageConfig {
	var size = utils.Qty1Gi()
	var className = getStorageClassname(storageClassName)
	if !storageSpec.Size.IsZero() {
		size = storageSpec.Size
	}
	if storageSpec.ClassName != "" {
		className = storageSpec.ClassName
	}
	return base.StorageConfig{
		StorageClassName: className,
		Size:             size,
	}
}

func (r *SFUtilContext) reconcileExpandPVC(pvcName string, newStorageSpec sfv1.StorageSpec) bool {
	newQTY := newStorageSpec.Size

	foundPVC := &apiv1.PersistentVolumeClaim{}
	if !r.GetM(pvcName, foundPVC) {
		r.log.V(1).Info("PVC " + pvcName + " not found")
		return false
	}
	r.log.V(1).Info("Inspecting volume " + foundPVC.Name)

	currentQTY := foundPVC.Status.Capacity.Storage()
	if currentQTY.IsZero() {
		// When the PVC is just created but not fully up it might happen that the current
		// size is 0. So let force a new reconcile.
		return false
	}

	// Is a resize in progress?
	for _, condition := range foundPVC.Status.Conditions {
		switch condition.Type {
		case
			apiv1.PersistentVolumeClaimResizing,
			apiv1.PersistentVolumeClaimFileSystemResizePending:
			r.log.V(1).Info("Volume resizing in progress, not ready")
			return false
		}
	}

	switch newQTY.Cmp(*currentQTY) {
	case -1:
		r.log.V(1).Info("Cannot downsize volume " + pvcName + ". Current size: " +
			currentQTY.String() + ", Expected size: " + newQTY.String())
		return true
	case 0:
		r.log.V(1).Info("Volume " + pvcName + " at expected size, nothing to do")
		return true
	case 1:
		r.log.V(1).Info("Volume expansion required for  " + pvcName +
			". current size: " + currentQTY.String() + " -> new size: " + newQTY.String())
		newResources := apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				"storage": newQTY,
			},
		}
		foundPVC.Spec.Resources = newResources
		if err := r.Client.Update(r.ctx, foundPVC); err != nil {
			r.log.V(1).Error(err, "Updating PVC failed for volume  "+pvcName)
			return false
		}
		// We return false to notify that a volume expansion was just
		// requested. Technically we could consider the reconcile is
		// over as most storage classes support hot resizing without
		// service interruption.
		r.log.V(1).Info("Expansion started for volume " + pvcName)
		return false
	}
	return true
}

// SFController struct-context scoped utils //

func (r *SFController) getStorageConfOrDefault(storageSpec sfv1.StorageSpec) base.StorageConfig {
	return BaseGetStorageConfOrDefault(storageSpec, r.cr.Spec.StorageClassName)
}

func (r *SFController) isConfigRepoSet() bool {
	return r.cr.Spec.ConfigLocation.BaseURL != "" &&
		r.cr.Spec.ConfigLocation.Name != "" &&
		r.cr.Spec.ConfigLocation.ZuulConnectionName != ""
}

func (r *SFController) MkClientDNSNames(serviceName string) []string {
	return []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, r.cr.GetName()),
		fmt.Sprintf("%s.%s", serviceName, r.cr.Spec.FQDN),
		r.cr.GetName(),
	}
}
