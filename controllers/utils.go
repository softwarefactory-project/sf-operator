// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"bytes"
	"context"
	_ "embed"
	e "errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/softwarefactory-project/sf-operator/controllers/libs/base"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/cert"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
	sfmonitoring "github.com/softwarefactory-project/sf-operator/controllers/libs/monitoring"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/utils"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const (
	CorporateCACerts            = "corporate-ca-certs"
	UpdateCATrustAnchorsPath    = "/usr/share/pki/ca-trust-source/anchors/"
	TrustedCAExtractedMountPath = "/etc/pki/ca-trust/extracted"
	UpdateCATrustCommand        = "set -x && update-ca-trust extract -o " + TrustedCAExtractedMountPath
)

//go:embed static/fetch-config-repo.sh
var fetchConfigRepoScript string

type HostAlias struct {
	IP        string   `json:"ip" mapstructure:"ip"`
	Hostnames []string `json:"hostnames" mapstructure:"hostnames"`
}

// --- API Interact primitive functions ---

// setOwnerReference set the Owner of a resources
// Whether we are running the controller or standalone mode the owneship must
// be managed differently
func (r *SFKubeContext) setOwnerReference(controlled metav1.Object) error {
	var err error
	if r.Standalone {
		err = controllerutil.SetOwnerReference(r.Owner, controlled, r.Scheme)
	} else {
		err = controllerutil.SetControllerReference(r.Owner, controlled, r.Scheme)
	}
	if err != nil {
		logging.LogE(err, "Unable to set controller reference, name="+controlled.GetName())

	}
	return err
}

// GetM gets a resource, returning if it was found
func (r *SFKubeContext) GetM(name string, obj client.Object) bool {
	err := r.Client.Get(r.Ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: r.Ns,
		},
		obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(err.Error())
	}
	return true
}

func (r *SFKubeContext) ReadSecret(name string) map[string][]byte {
	var sec apiv1.Secret
	if r.GetM(name, &sec) {
		return sec.Data
	} else {
		return make(map[string][]byte)
	}
}

func (r *SFKubeContext) ReadSecretValue(name string, key string) string {
	return string(r.ReadSecret(name)[key])
}

// CreateR creates a resource with the owner as the ownerReferences.
func (r *SFKubeContext) CreateR(obj client.Object) {
	r.setOwnerReference(obj)
	var err error
	msg := "Creating object, name: " + obj.GetName()
	opts := []client.CreateOption{}
	if r.DryRun {
		msg += " (server dry run)"
		opts = append(opts, client.DryRunAll)
	}
	logging.LogI(msg)
	err = r.Client.Create(r.Ctx, obj, opts...)
	if err != nil && !errors.IsAlreadyExists(err) {
		panic(err.Error())
	}
}

// DeleteR delete a resource.
func (r *SFKubeContext) DeleteR(obj client.Object) {
	var err error
	msg := "Deleting object, name: " + obj.GetName()
	opts := []client.DeleteOption{}
	if r.DryRun {
		msg += " (server dry run)"
		opts = append(opts, client.DryRunAll)
	}
	logging.LogI(msg)
	err = r.Client.Delete(r.Ctx, obj, opts...)
	if err != nil && !errors.IsNotFound(err) {
		panic(err.Error())
	}
}

// UpdateR updates resource with the owner as the ownerReferences.
func (r *SFKubeContext) UpdateR(obj client.Object) bool {
	r.setOwnerReference(obj)
	var err error
	msg := "Updating object, name: " + obj.GetName()
	opts := []client.UpdateOption{}
	if r.DryRun {
		msg += " (server dry run)"
		opts = append(opts, client.DryRunAll)
	}
	logging.LogI(msg)
	err = r.Client.Update(r.Ctx, obj, opts...)
	if err != nil {
		// A not found error is ignored during dry-run because the object might be created
		// in the same reconciliation loop.
		if r.DryRun && errors.IsNotFound(err) {
			logging.LogI("Object not found during dry-run update, name: " + obj.GetName())
			return true
		}
		panic(err.Error())
	}
	return true
}

// GetOrCreate does not change an existing object, update needs to be used manually.
// In the case the object already exists then the function return True
func (r *SFKubeContext) GetOrCreate(obj client.Object) bool {
	name := obj.GetName()

	if !r.GetM(name, obj) {
		r.CreateR(obj)
		return false
	}
	return true
}

// PodExecOut connects to a container's Pod and execute a command
// Stderr is output on the caller's Stdout
// The function returns an Error for any issue
func (r *SFKubeContext) PodExecOut(pod string, container string, command []string, out io.Writer) error {
	logging.LogI(fmt.Sprintf("Running pod execution pod: %s, command: %s", pod, command))
	execReq := r.RESTClient.
		Post().
		Namespace(r.Ns).
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
		Stdout: out,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	return nil
}

// PodExec connects to a container's Pod and execute a command
// Stdout and Stderr is output on the caller's Stdout
func (r *SFKubeContext) PodExec(pod string, container string, command []string) error {
	return r.PodExecOut(pod, container, command, os.Stdout)
}

func (r *SFKubeContext) PodExecM(pod string, container string, command []string) {
	if err := r.PodExecOut(pod, container, command, os.Stdout); err != nil {
		panic(fmt.Sprintf("Command exec failed: %s", err))
	}
}

func (r *SFKubeContext) PodExecBytes(pod string, container string, command []string) bytes.Buffer {
	var buf bytes.Buffer
	if err := r.PodExecOut(pod, container, command, &buf); err != nil {
		panic(fmt.Sprintf("Command exec failed: %s", err))
	}
	return buf
}

// --- Ensure resources functions ---

// EnsureConfigMap ensures a config map exist
// The ConfigMap is updated if needed
func (r *SFKubeContext) EnsureConfigMap(baseName string, data map[string]string) apiv1.ConfigMap {
	name := baseName + "-config-map"
	var cm apiv1.ConfigMap
	if !r.GetM(name, &cm) {
		if !r.DryRun {
			logging.LogI("Creating config map name: " + name)
		}
		cm = apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.Ns},
			Data:       data,
		}
		r.CreateR(&cm)
	} else {
		if !reflect.DeepEqual(cm.Data, data) {
			if r.DryRun {
				logging.LogI("[Dry Run] Would update ConfigMap, name: " + name + ". Reason: Data has changed")
			} else {
				logging.LogI("Updating configmap, name: " + name)
			}
			cm.Data = data
			r.UpdateR(&cm)
		}
	}
	return cm
}

// EnsureSecret ensures a Secret exist
// The Secret is updated if needed
func (r *SFKubeContext) EnsureSecret(secret *apiv1.Secret) {
	var current apiv1.Secret
	name := secret.GetName()
	if !r.GetM(name, &current) {
		if !r.DryRun {
			logging.LogI("Creating secret, name: " + name)
		}
		r.CreateR(secret)
	} else {
		if !reflect.DeepEqual(current.Data, secret.Data) {
			if r.DryRun {
				logging.LogI("[Dry Run] Would update Secret, name: " + name + ". Reason: Data has changed")
			} else {
				logging.LogI("Updating secret, name: " + name)
			}
			current.Data = secret.Data
			r.UpdateR(&current)
		}
	}
}

// ensureSecretFromFunc ensure a Secret exists
// If it does not the Secret is created from the getData function
// This function does not support Secret update
// This function returns the Secret
func (r *SFKubeContext) ensureSecretFromFunc(name string, getData func() string) apiv1.Secret {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		if !r.DryRun {
			logging.LogI("Creating secret, name: " + name)
		}
		secret = base.MkSecretFromFunc(name, r.Ns, getData)
		r.CreateR(&secret)
	}
	return secret
}

// EnsureSecretUUID ensures a Secret containing an UUID
// This function does not support update
func (r *SFKubeContext) EnsureSecretUUID(name string) apiv1.Secret {
	return r.ensureSecretFromFunc(name, utils.NewUUIDString)
}

// EnsureSSHKeySecret ensures a Secret exists container an autogenerated SSH key pair
// If it does not exixtthe Secret is created
// This function does not support Secret update
func (r *SFKubeContext) EnsureSSHKeySecret(name string) {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		if !r.DryRun {
			logging.LogI("Creating ssh key, name: " + name)
		}
		secret := base.MkSSHKeySecret(name, r.Ns)
		r.CreateR(&secret)
	}
}

// EnsureService ensures a Service exists
// The Service is updated if needed
func (r *SFKubeContext) EnsureService(service *apiv1.Service) {
	var current apiv1.Service
	spsAsString := func(sps []apiv1.ServicePort) string {
		s := []string{}
		for _, p := range sps {
			s = append(s, []string{strconv.Itoa(int(p.Port)), p.Name, p.TargetPort.String(), string(p.Protocol)}...)
		}
		sort.Strings(s)
		return strings.Join(s[:], "")
	}
	name := service.GetName()
	if !r.GetM(name, &current) {
		if !r.DryRun {
			logging.LogI("Creating service, name: " + name)
		}
		r.CreateR(service)
	} else {
		if !reflect.DeepEqual(current.Spec.Selector, service.Spec.Selector) ||
			spsAsString(current.Spec.Ports) != spsAsString(service.Spec.Ports) {
			if r.DryRun {
				logging.LogI("[Dry Run] Would update Service, name: " + name + ". Reason: Spec has changed")
			} else {
				logging.LogI("Updating service, name: " + name)
			}
			current.Spec = *service.Spec.DeepCopy()
			r.UpdateR(&current)
		}
	}
}

// EnsureZookeeperCertificates ensures the following TLS secrets for zookeeper/zuul/nodepool
// connections:
// - self-signed root certificate authority
// - client certificate for zookeeper, localhost (so we can use zkClient locally from the pod)
// - server certificates for each zookeeper replica
func (r *SFController) EnsureZookeeperCertificates(ZookeeperIdent string, ZookeeperReplicas int) {
	annotations := map[string]string{
		"serial": "2",
	}

	caCert, caPrivKey, caPEM, caPrivKeyPEM := cert.X509CA()
	certificateCASecret := apiv1.Secret{
		Data: map[string][]byte{
			"ca.crt":  caPEM.Bytes(),
			"tls.crt": caPEM.Bytes(),
			"tls.key": caPrivKeyPEM.Bytes(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ca-cert",
			Namespace:   r.Ns,
			Annotations: annotations,
		},
	}
	currentCASecret := apiv1.Secret{}
	if r.GetM(certificateCASecret.Name, &currentCASecret) {
		if !utils.MapEquals(&currentCASecret.ObjectMeta.Annotations, &annotations) {
			r.UpdateR(&certificateCASecret)
		}
	} else {
		r.GetOrCreate(&certificateCASecret)
	}

	// client cert
	clientDNSNames := []string{
		ZookeeperIdent,
		fmt.Sprintf("%s.%s", ZookeeperIdent, r.cr.GetName()),
		fmt.Sprintf("%s.%s", ZookeeperIdent, r.cr.Spec.FQDN),
		"localhost",
	}
	for i := range ZookeeperReplicas {
		clientDNSNames = append(
			clientDNSNames,
			fmt.Sprintf("%s-%d", ZookeeperIdent, i),
			fmt.Sprintf("%s-%d.%s", ZookeeperIdent, i, r.cr.GetName()),
			fmt.Sprintf("%s-%d.%s", ZookeeperIdent, i, r.cr.Spec.FQDN),
		)
	}
	zkClientCertPEM, zkClientPrivKeyPEM := cert.X509Cert(caCert, caPrivKey, clientDNSNames)

	zkClientCertificateSecret := apiv1.Secret{
		Data: map[string][]byte{
			"ca.crt":  caPEM.Bytes(),
			"tls.crt": zkClientCertPEM.Bytes(),
			"tls.key": zkClientPrivKeyPEM.Bytes(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "zookeeper-client-tls",
			Namespace:   r.Ns,
			Annotations: annotations,
		},
	}
	currentClientSecret := apiv1.Secret{}
	if r.GetM(zkClientCertificateSecret.Name, &currentClientSecret) {
		if !utils.MapEquals(&currentClientSecret.ObjectMeta.Annotations, &annotations) {
			r.UpdateR(&zkClientCertificateSecret)
		}
	} else {
		r.GetOrCreate(&zkClientCertificateSecret)
	}

	// servers certificates
	var serversSecretData = make(map[string][]byte)
	serversSecretData["ca.crt"] = caPEM.Bytes()
	for i := range ZookeeperReplicas {
		serversDNSNames := []string{}
		var replicaName = fmt.Sprintf("%s-%d", ZookeeperIdent, i)
		var replicaWithService = fmt.Sprintf("%s.%s-headless", replicaName, ZookeeperIdent)
		var replicaNamespaced = fmt.Sprintf("%s.%s", replicaWithService, r.Ns)
		var replicaFQDN = fmt.Sprintf("%s.%s", replicaWithService, r.cr.Spec.FQDN)
		serversDNSNames = append(serversDNSNames, replicaName, replicaWithService, replicaNamespaced, replicaFQDN)
		zkServersCertPEM, zkServersPrivKeyPEM := cert.X509Cert(caCert, caPrivKey, serversDNSNames)
		serversSecretData[fmt.Sprintf("%d-tls.crt", i)] = zkServersCertPEM.Bytes()
		serversSecretData[fmt.Sprintf("%d-tls.key", i)] = zkServersPrivKeyPEM.Bytes()
	}

	zkServersCertificateSecret := apiv1.Secret{
		Data: serversSecretData,
		ObjectMeta: metav1.ObjectMeta{
			Name:        "zookeeper-server-tls",
			Namespace:   r.Ns,
			Annotations: annotations,
		},
		Type: "Opaque",
	}
	currentServersSecret := apiv1.Secret{}
	if r.GetM(zkServersCertificateSecret.Name, &currentServersSecret) {
		if !utils.MapEquals(&currentServersSecret.ObjectMeta.Annotations, &annotations) {
			r.UpdateR(&zkServersCertificateSecret)
		}
	} else {
		r.GetOrCreate(&zkServersCertificateSecret)
	}
}

// mkStatefulSet Create a default statefulset.
func (r *SFKubeContext) mkStatefulSet(name string, image string, storageConfig base.StorageConfig, accessMode apiv1.PersistentVolumeAccessMode, extraLabels map[string]string, openshiftUser bool, nameSuffix ...string) appsv1.StatefulSet {
	serviceName := name
	if nameSuffix != nil {
		serviceName = name + "-" + nameSuffix[0]
	}

	container := base.MkContainer(name, image, openshiftUser)
	pvc := base.MkPVC(name, r.Ns, storageConfig, accessMode)
	return base.MkStatefulset(name, r.Ns, 1, serviceName, container, pvc, extraLabels)
}

// mkHeadlessStatefulSet Create a default headless statefulset.
func (r *SFKubeContext) mkHeadlessStatefulSet(
	name string, image string, storageConfig base.StorageConfig,
	accessMode apiv1.PersistentVolumeAccessMode, extraLabels map[string]string, openshiftUser bool) appsv1.StatefulSet {
	return r.mkStatefulSet(name, image, storageConfig, accessMode, extraLabels, openshiftUser, "headless")
}

// getPods return the StatefulSet pods and a bool which is true when all the pods are ready.
func (r *SFKubeContext) getPods(dep *appsv1.StatefulSet) ([]apiv1.Pod, bool) {
	pods := make([]apiv1.Pod, 0)
	if dep.Status.ReadyReplicas > 0 {
		var podList apiv1.PodList
		matchLabels := dep.Spec.Selector.MatchLabels
		labels := labels.SelectorFromSet(labels.Set(matchLabels))
		labelSelectors := client.MatchingLabelsSelector{Selector: labels}
		if err := r.Client.List(r.Ctx, &podList, labelSelectors, client.InNamespace(r.Ns)); err != nil {
			panic(err.Error())
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != "Running" {
				logging.LogI(fmt.Sprintf(
					"Waiting for pod name: %s, pod state: %s, statefulset name: %s, statefulset status: %v", pod.Name, pod.Status.Phase, dep.GetName(), dep.Status))
				return pods, false
			}
			containerStatuses := pod.Status.ContainerStatuses
			for _, containerStatus := range containerStatuses {
				if !containerStatus.Ready {
					logging.LogI(fmt.Sprintf(
						"Waiting for statefulset containers ready, name: %s, status: %v, podStatus: %v; containerStatuses: %v",
						dep.GetName(),
						dep.Status,
						pod.Status,
						containerStatuses))
					return pods, false
				}
			}
			pods = append(pods, pod)
		}
	}
	return pods, true
}

// IsStatefulSetReady checks if StatefulSet is ready
func (r *SFKubeContext) IsStatefulSetReady(dep *appsv1.StatefulSet) bool {
	if r.DryRun {
		return true
	}

	logging.LogI("Waiting for statefulset, name: " + dep.ObjectMeta.GetName())
	rolloutOk := base.IsStatefulSetRolloutDone(dep)
	if rolloutOk {
		_replicas := dep.Spec.Replicas
		var replicas int32
		if _replicas == nil {
			replicas = 1
		} else {
			replicas = *_replicas
		}
		if replicas != dep.Status.ReadyReplicas {
			logging.LogI("Waiting for statefulset readyReplicas, name: " + dep.ObjectMeta.GetName())
			return false
		} else if replicas > 0 {
			_, ok := r.getPods(dep)
			return ok
		} else {
			// Nothing left to do
			return true
		}
	}
	logging.LogI("statefulset rollout not ok, name: " + dep.ObjectMeta.GetName())
	return false
}

// IsDeploymentReady checks if Deployment is ready
func (r *SFKubeContext) IsDeploymentReady(dep *appsv1.Deployment) bool {
	if r.DryRun {
		return true
	}

	logging.LogI("Waiting for deployment, name: " + dep.ObjectMeta.GetName())
	rolloutOk := base.IsDeploymentRolloutDone(dep)
	if rolloutOk {
		_replicas := dep.Spec.Replicas
		var replicas int32
		if _replicas == nil {
			replicas = 1
		} else {
			replicas = *_replicas
		}
		if replicas > 0 {
			// At least one replica up is enough
			logging.LogI("Checking deployment ready replicas, name: " + dep.ObjectMeta.GetName())
			return dep.Status.ReadyReplicas > 0
		} else {
			return true
		}
	}
	logging.LogI("deployment rollout not ok, name: " + dep.ObjectMeta.GetName())
	return false
}

// DebugStatefulSet disables StatefulSet main container probes
func (r *SFKubeContext) DebugStatefulSet(name string) {
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
	logging.LogI("Debugging service, name: " + name)
}

// GetConfigMap Get ConfigMap by name
func (r *SFKubeContext) GetConfigMap(name string) (apiv1.ConfigMap, error) {
	var dep apiv1.ConfigMap
	if name != "" && r.GetM(name, &dep) {
		return dep, nil
	}
	return apiv1.ConfigMap{}, fmt.Errorf("configMap named '%s' was not found", name)
}

// GetSecret Get Secret by name
func (r *SFKubeContext) GetSecret(name string) (apiv1.Secret, error) {
	var dep apiv1.Secret
	if name != "" && r.GetM(name, &dep) {
		return dep, nil
	}
	return apiv1.Secret{}, fmt.Errorf("secret named '%s' was not found", name)
}

// GetValueFromKeySecret gets the Value of the Keyname from a Secret
func GetValueFromKeySecret(secret apiv1.Secret, keyname string) ([]byte, error) {
	keyvalue := secret.Data[keyname]
	if len(keyvalue) == 0 {
		return []byte{}, fmt.Errorf("key named %s not found in Secret %s at namespace %s", keyname, secret.Name, secret.Namespace)
	}

	return keyvalue, nil
}

// compareAnnotations compares two maps of annotations and returns a slice of strings
// describing the differences.
func compareAnnotations(current, desired map[string]string) []string {
	var diffs []string

	// Find added or modified annotations
	for key, desiredValue := range desired {
		currentValue, exists := current[key]
		if !exists {
			diffs = append(diffs, fmt.Sprintf("annotation '%s' added with value: %s", key, desiredValue))
		} else if currentValue != desiredValue {
			diffs = append(diffs, fmt.Sprintf("annotation '%s' changed from '%s' to '%s'", key, desiredValue, currentValue))
		}
	}

	// Find removed annotations
	for key, currentValue := range current {
		if _, exists := desired[key]; !exists {
			diffs = append(diffs, fmt.Sprintf("annotation '%s' with value '%s' removed", key, currentValue))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// GetSecretDataFromKey Get Data from Secret Key
func (r *SFKubeContext) GetSecretDataFromKey(name string, key string) ([]byte, error) {
	secret, err := r.GetSecret(name)
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

// getSecretData Gets Secret Data in which the Keyname is the same as the Secret Name
func (r *SFKubeContext) getSecretData(name string) ([]byte, error) {
	return r.GetSecretDataFromKey(name, "")
}

// BaseGetStorageConfOrDefault sets the default storageClassName
func BaseGetStorageConfOrDefault(storageSpec sfv1.StorageSpec, storageDefault sfv1.StorageDefaultSpec) base.StorageConfig {
	var size = utils.Qty1Gi()
	var className *string
	if storageDefault.ClassName != "" {
		className = &storageDefault.ClassName
	}
	if !storageSpec.Size.IsZero() {
		size = storageSpec.Size
	}
	if storageSpec.ClassName != "" {
		className = &storageSpec.ClassName
	}
	return base.StorageConfig{
		StorageClassName: className,
		Size:             size,
		ExtraAnnotations: storageDefault.ExtraAnnotations,
	}
}

func (r *SFKubeContext) reconcileExpandPVCs(serviceName string, newStorageSpec sfv1.StorageSpec) bool {
	PVCList := &apiv1.PersistentVolumeClaimList{}
	selector := client.MatchingLabels{"run": serviceName, "app": "sf"}
	err := r.Client.List(r.Ctx, PVCList, selector, client.InNamespace(r.Ns))
	if err != nil {
		logging.LogE(err, "Unable to get the list of PVC for service "+serviceName)
		return false
	}
	readyList := []bool{}
	for _, pvc := range PVCList.Items {
		readyList = append(readyList, r.reconcileExpandPVC(pvc.Name, newStorageSpec))
	}
	for _, r := range readyList {
		if !r {
			return false
		}
	}
	return true
}

func (r *SFKubeContext) canStorageResize(storageName *string) bool {
	var sc storagev1.StorageClass
	if storageName == nil || *storageName == "" || !r.GetM(*storageName, &sc) {
		// This is odd, so let's assume that unknown storage class support expansion
		return true
	}
	return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
}

// reconcileExpandPVC  resizes the pvc with the spec
func (r *SFKubeContext) reconcileExpandPVC(pvcName string, newStorageSpec sfv1.StorageSpec) bool {
	newQTY := newStorageSpec.Size
	if newQTY.Sign() <= 0 {
		return true
	}

	foundPVC := &apiv1.PersistentVolumeClaim{}
	if !r.GetM(pvcName, foundPVC) {
		logging.LogI("PVC " + pvcName + " not found")
		return false
	}
	logging.LogD("Inspecting volume " + foundPVC.Name)

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
			logging.LogI("Volume " + pvcName + " resizing in progress, not ready")
			return false
		}
	}

	switch newQTY.Cmp(*currentQTY) {
	case -1:
		logging.LogE(e.New("volume downsize"), "Cannot downsize volume "+pvcName+". Current size: "+
			currentQTY.String()+", Expected size: "+newQTY.String())
		return true
	case 0:
		logging.LogD("Volume " + pvcName + " at expected size, nothing to do")
		return true
	case 1:
		if !r.canStorageResize(foundPVC.Spec.StorageClassName) {
			scn := ""
			if foundPVC.Spec.StorageClassName != nil {
				scn = *(foundPVC.Spec.StorageClassName)
			}
			logging.LogI("Volume expansion is not supported for " + scn)
			return true
		}
		logging.LogI("Volume expansion required for  " + pvcName +
			". current size: " + currentQTY.String() + " -> new size: " + newQTY.String())
		newResources := apiv1.VolumeResourceRequirements{
			Requests: apiv1.ResourceList{
				"storage": newQTY,
			},
		}
		foundPVC.Spec.Resources = newResources
		if err := r.Client.Update(r.Ctx, foundPVC); err != nil {
			logging.LogE(err, "Updating PVC failed for volume, name: "+pvcName)
			return false
		}
		// We return false to notify that a volume expansion was just
		// requested. Technically we could consider the reconcile is
		// over as most storage classes support hot resizing without
		// service interruption.
		logging.LogI("Expansion started for volume " + pvcName)
		return false
	}
	return true
}

// SFController struct-context scoped utils //

// getStorageConfOrDefault get storage configuration or sets the default configuration
func (r *SFController) getStorageConfOrDefault(storageSpec sfv1.StorageSpec) base.StorageConfig {
	return BaseGetStorageConfOrDefault(storageSpec, r.cr.Spec.StorageDefault)
}

// isConfigRepoSet checks if config repository is set in the CR
func (r *SFController) isConfigRepoSet() bool {
	return r.cr.Spec.ConfigRepositoryLocation.Name != "" &&
		r.cr.Spec.ConfigRepositoryLocation.ZuulConnectionName != ""
}

// MkClientDNSNames returns an array of DNS Names
func (r *SFController) MkClientDNSNames(serviceName string) []string {
	return []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, r.cr.GetName()),
		fmt.Sprintf("%s.%s", serviceName, r.cr.Spec.FQDN),
		r.cr.GetName(),
	}
}

// EnsureDiskUsagePromRule sync Prometheus Rules
func (r *SFController) EnsureDiskUsagePromRule(ruleGroups []monitoringv1.RuleGroup) bool {
	desiredDUPromRule := sfmonitoring.MkDiskUsagePromRule(ruleGroups, r.Ns)
	currentPromRule := monitoringv1.PrometheusRule{}
	if !r.GetM(desiredDUPromRule.Name, &currentPromRule) {
		r.CreateR(&desiredDUPromRule)
		return false
	} else {
		if !utils.MapEquals(&currentPromRule.ObjectMeta.Annotations, &desiredDUPromRule.ObjectMeta.Annotations) {
			if !r.DryRun {
				logging.LogI("Default disk usage Prometheus rules changed, updating...")
			}
			currentPromRule.Spec = desiredDUPromRule.Spec
			currentPromRule.ObjectMeta.Annotations = desiredDUPromRule.ObjectMeta.Annotations
			r.UpdateR(&currentPromRule)
			return false
		}
	}
	return true
}

// EnsureSFPodMonitor Create or Updates Software Factory Monitor for metrics
func (r *SFController) EnsureSFPodMonitor(ports []string, selector metav1.LabelSelector) bool {
	desiredPodMonitor := sfmonitoring.MkPodMonitor("sf-monitor", r.Ns, ports, selector)
	// add annotations so we can handle lifecycle
	var portsChecksumable string
	sort.Strings(ports)
	for _, port := range ports {
		portsChecksumable += port + " "
	}
	annotations := map[string]string{
		"version": "1",
		"ports":   utils.Checksum([]byte(portsChecksumable)),
	}
	desiredPodMonitor.ObjectMeta.Annotations = annotations
	currentPodMonitor := monitoringv1.PodMonitor{}
	if !r.GetM(desiredPodMonitor.Name, &currentPodMonitor) {
		r.CreateR(&desiredPodMonitor)
		return false
	} else {
		if !utils.MapEquals(&currentPodMonitor.ObjectMeta.Annotations, &annotations) {
			if !r.DryRun {
				logging.LogI("SF PodMonitor configuration changed, updating...")
			}
			currentPodMonitor.Spec = desiredPodMonitor.Spec
			currentPodMonitor.ObjectMeta.Annotations = annotations
			r.UpdateR(&currentPodMonitor)
			return false
		}
	}
	return true
}

// injectStorageNodeAffinity injects the node affinity setting when the StatefulSet uses a given storageClass.
// The node affinity is set to ensure that the pods are not scheduled to a different host by setting the current node name as a node selector requirement.
// returns true if nodeAffinity is modified by calling this function.
func (r *SFController) injectStorageNodeAffinity(storageClass *string, sts *appsv1.StatefulSet) bool {
	if sts.Spec.Replicas != nil {
		replicas := *sts.Spec.Replicas
		if replicas == 0 {
			logging.LogI(fmt.Sprintf("%s: replicas set to 0, skipping node affinity injection", sts.ObjectMeta.Name))
			return false
		}
	}
	storageDefault := r.cr.Spec.StorageDefault
	if storageClass == nil {
		return false
	}
	if storageDefault.NodeAffinity && storageDefault.ClassName == *storageClass {
		pods, ok := r.getPods(sts)
		if !ok {
			logging.LogI(fmt.Sprintf("%s: Unknown error encountered while listing pods, skipping node affinity", sts.ObjectMeta.Name))
			return false
		} else {
			nodes := make([]string, 0)
			for _, pod := range pods {
				name := pod.Spec.NodeName
				if !slices.Contains(nodes, name) {
					nodes = append(nodes, name)
				}
			}
			if len(nodes) == 0 {
				logging.LogI(fmt.Sprintf("%s: could not establish which node(s) to set affinity for, skipping nodeAffinity setup. "+
					"If this message persists at the end of the reconciliation, "+
					"re-run sf-operator when pods are actually deployed to set node affinity", sts.ObjectMeta.Name))
				return false
			}
			// Have we set the nodeAffinity before?
			logging.LogI(fmt.Sprintf("%s: check if node affinity is set ...", sts.ObjectMeta.Name))
			if sts.Spec.Template.Spec.Affinity != nil {
				affinity := *sts.Spec.Template.Spec.Affinity
				if affinity.NodeAffinity != nil {
					nodeAffinity := *affinity.NodeAffinity
					for _, schedTerm := range nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
						matchExprs := schedTerm.Preference.MatchExpressions
						for _, matchExpr := range matchExprs {
							if matchExpr.Key == "kubernetes.io/hostname" && matchExpr.Operator == "In" {
								var fullMatch = true
								for _, node := range nodes {
									if !slices.Contains(matchExpr.Values, node) {
										fullMatch = false
									}
								}
								if fullMatch {
									logging.LogI(fmt.Sprintf("%s: node affinity already set, skipping", sts.ObjectMeta.Name))
									return false
								}
							}
						}
					}
				}
			}
			logging.LogI(fmt.Sprintf("%s: assigning node affinity to %s", sts.ObjectMeta.Name, nodes))
			sts.Spec.Template.Spec.Affinity = &apiv1.Affinity{
				NodeAffinity: &apiv1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []apiv1.PreferredSchedulingTerm{
						{
							Weight: 100,
							Preference: apiv1.NodeSelectorTerm{
								MatchExpressions: []apiv1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: "In",
										Values:   nodes,
									},
								},
							},
						},
					},
				},
			}

			return true
		}
	} else {
		return false
	}
}

func getGracePeriod(sts appsv1.StatefulSet) int64 {
	if sts.Spec.Template.Spec.TerminationGracePeriodSeconds != nil {
		return *sts.Spec.Template.Spec.TerminationGracePeriodSeconds
	}
	return 30
}

func imageChanged(desired []apiv1.Container, current []apiv1.Container) []string {
	missing := make([]string, 0)
	for idx, desiredContainer := range desired {
		if idx >= len(current) || current[idx].Image != desiredContainer.Image {
			missing = append(missing, desiredContainer.Image)
		}
	}
	return missing
}

func (r *SFController) ensureDeployment(dep appsv1.Deployment, desiredReplicaCount *int32) (*appsv1.Deployment, bool) {
	current := appsv1.Deployment{}
	name := dep.ObjectMeta.Name
	needUpdate := false
	var diffs []string
	logPrefix := ""
	logger := logging.LogD
	if r.GetM(name, &current) {
		// Assume a default count of 1 replica
		currentReplicas := int32(1)
		if current.Spec.Replicas != nil {
			currentReplicas = *current.Spec.Replicas
		}
		if desiredReplicaCount != nil {
			if *desiredReplicaCount != currentReplicas {
				needUpdate = true
				// force replica count programmatically
				diffs = append(diffs, fmt.Sprintf("forcing replica count change from %d to %d", currentReplicas, *desiredReplicaCount))
				current.Spec.Replicas = desiredReplicaCount
			}
		}
		if missing := imageChanged(dep.Spec.Template.Spec.Containers, current.Spec.Template.Spec.Containers); len(missing) > 0 {
			needUpdate = true
			//TODO explicit which images
			diffs = append(diffs, "some images changed")
		}
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &dep.Spec.Template.ObjectMeta.Annotations) {
			needUpdate = true
			diffs = append(diffs, compareAnnotations(current.Spec.Template.ObjectMeta.Annotations, dep.Spec.Template.ObjectMeta.Annotations)...)
		}
		if (!reflect.DeepEqual(current.Spec.Strategy, dep.Spec.Strategy) && dep.Spec.Strategy != appsv1.DeploymentStrategy{}) {
			needUpdate = true
			current.Spec.Strategy = *dep.Spec.Strategy.DeepCopy()
			diffs = append(diffs, fmt.Sprintf("strategy changed from %+v to %+v", current.Spec.Strategy, dep.Spec.Strategy))
		}

		if needUpdate {
			if r.DryRun {
				logger = logging.LogI
				logPrefix = "[Dry Run] "
			}
			current.Spec.Template = *dep.Spec.Template.DeepCopy()
			reason := strings.Join(diffs, ", ")
			logging.LogI(fmt.Sprintf("%sStatefulset \"%s\" configuration changed, applying...", logPrefix, name))
			logger(fmt.Sprintf("%sReason: %s", logPrefix, reason))
			r.UpdateR(&current)
			return &current, true
		}
	} else {
		current := dep
		r.CreateR(&current)
		return &current, true
	}
	return &current, false
}

// ensureStatefulSet ensures that a StatefulSet object is as expected.
// The function takes the expected StatefulSet and returns a tuple with the current object on
// the cluster and a boolean indicating whether the function performed a create or update on the object.
func (r *SFController) ensureStatefulset(storageClass *string, sts appsv1.StatefulSet, desiredReplicaCount *int32) (*appsv1.StatefulSet, bool) {
	current := appsv1.StatefulSet{}
	name := sts.ObjectMeta.Name
	needUpdate := false
	var diffs []string
	logPrefix := ""
	logger := logging.LogD

	if r.GetM(name, &current) {
		// Assume a default count of 1 replica
		currentReplicas := int32(1)
		if current.Spec.Replicas != nil {
			currentReplicas = *current.Spec.Replicas
		}
		if desiredReplicaCount != nil {
			if *desiredReplicaCount != currentReplicas {
				needUpdate = true
				// force replica count programmatically
				diffs = append(diffs, fmt.Sprintf("forcing replica count change from %d to %d", currentReplicas, *desiredReplicaCount))
				current.Spec.Replicas = desiredReplicaCount
			}
		}
		if missing := imageChanged(sts.Spec.Template.Spec.Containers, current.Spec.Template.Spec.Containers); len(missing) > 0 {
			needUpdate = true
			//TODO explicit which images
			diffs = append(diffs, "some images changed")
		}
		if !utils.MapEquals(&current.Spec.Template.ObjectMeta.Annotations, &sts.Spec.Template.ObjectMeta.Annotations) {
			needUpdate = true
			diffs = append(diffs, compareAnnotations(current.Spec.Template.ObjectMeta.Annotations, sts.Spec.Template.ObjectMeta.Annotations)...)
		}
		if getGracePeriod(current) != getGracePeriod(sts) {
			diffs = append(diffs, "terminationGracePeriodSeconds changed")
			needUpdate = true
		}
		// TODO does this need to be done before the call to injectStorageNodeAffinity?
		if needUpdate {
			current.Spec.Template = *sts.Spec.Template.DeepCopy()
		}
		if r.injectStorageNodeAffinity(storageClass, &current) {
			needUpdate = true
			diffs = append(diffs, "storage node affinity config changed")
		}
		if needUpdate {
			if r.DryRun {
				logger = logging.LogI
				logPrefix = "[Dry Run] "
			}
			reason := strings.Join(diffs, ", ")
			logging.LogI(fmt.Sprintf("%sStatefulset \"%s\" configuration changed, applying...", logPrefix, name))
			logger(fmt.Sprintf("%sReason: %s", logPrefix, reason))
			r.UpdateR(&current)
			return &current, true
		}
	} else {
		current := sts
		r.CreateR(&current)
		return &current, true
	}
	return &current, false
}

// CorporateCAConfigMapExists check if the ConfigMap named "corporate-ca-certs" exists
func (r *SFKubeContext) CorporateCAConfigMapExists() (apiv1.ConfigMap, bool) {
	cm, err := r.GetConfigMap(CorporateCACerts)
	return cm, err == nil
}

func AppendCorporateCACertsVolumeMount(volumeMounts []apiv1.VolumeMount, volumeName string) []apiv1.VolumeMount {
	volumeMounts = append(volumeMounts, apiv1.VolumeMount{
		Name:      volumeName,
		MountPath: UpdateCATrustAnchorsPath,
	})
	return volumeMounts
}

func AppendToolingVolume(volumeMounts []apiv1.Volume) []apiv1.Volume {
	return append(volumeMounts, apiv1.Volume{
		Name: "tooling-vol",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "zuul-scheduler-tooling-config-map",
				},
				DefaultMode: &utils.Execmod,
			},
		}})
}

func RunPodCmdRaw(restConfig *rest.Config, kubeClientset *kubernetes.Clientset, namespace string, podName string, containerName string, cmdArgs []string) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}
	request := kubeClientset.CoreV1().RESTClient().Post().Resource("Pods").Namespace(namespace).Name(podName).SubResource("exec").VersionedParams(
		&apiv1.PodExecOptions{
			Container: containerName,
			Command:   cmdArgs,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		},
		scheme.ParameterCodec,
	)
	exec, _ := remotecommand.NewSPDYExecutor(restConfig, "POST", request.URL())
	err := exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: buffer,
		Stderr: errorBuffer,
	})
	if err != nil {
		errMsg := fmt.Sprintf("Command \"%s\" [Pod: %s - Container: %s] failed with the following stderr: %s",
			strings.Join(cmdArgs, " "), podName, containerName, errorBuffer.String())
		logging.LogE(err, errMsg)
		return nil, err
	}
	return buffer, nil
}

func (r *SFKubeContext) RunPodCmd(podName string, containerName string, cmdArgs []string) (*bytes.Buffer, error) {
	return RunPodCmdRaw(r.RESTConfig, r.ClientSet, r.Ns, podName, containerName, cmdArgs)
}
