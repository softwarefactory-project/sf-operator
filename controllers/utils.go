// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/labels"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apiroutev1 "github.com/openshift/api/route/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const BUSYBOX_IMAGE = "quay.io/software-factory/sf-op-busybox:1.3-1"

func checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type SSHKey struct {
	Pub  []byte
	Priv []byte
}

// TODO: the line below can be removed when we move to compiler version 1.18 (current 1.17)
type any = interface{}

// Function to easilly use templates files.
//
// Pass the template path relative to the root of the project.
// And the data structure to be applied to the template
func parse_template(templatePath string, data any) (string, error) {

	// Opening Template file
	template, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("file not found: " + templatePath)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", templatePath)
	}

	return buf.String(), nil
}

// Function to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func parse_string(text string, data any) (string, error) {

	template.New("StringtoParse").Parse(text)
	// Opening Template file
	template, err := template.New("StringtoParse").Parse(text)
	if err != nil {
		return "", fmt.Errorf("Text not in the right format: " + text)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", text)
	}

	return buf.String(), nil
}

func create_ssh_key() SSHKey {
	bitSize := 4096

	generatePrivateKey := func(bitSize int) (*rsa.PrivateKey, error) {
		// Private Key generation
		privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
		if err != nil {
			return nil, err
		}
		// Validate Private Key
		err = privateKey.Validate()
		if err != nil {
			return nil, err
		}
		return privateKey, nil
	}

	generatePublicKey := func(privatekey *rsa.PublicKey) ([]byte, error) {
		publicRsaKey, err := ssh.NewPublicKey(privatekey)
		if err != nil {
			return nil, err
		}

		pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

		return pubKeyBytes, nil
	}

	encodePrivateKeyToPEM := func(privateKey *rsa.PrivateKey) []byte {
		// Get ASN.1 DER format
		privDER := x509.MarshalPKCS1PrivateKey(privateKey)

		// pem.Block
		privBlock := pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   privDER,
		}

		// Private key in PEM format
		privatePEM := pem.EncodeToMemory(&privBlock)

		return privatePEM
	}

	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		panic(err.Error())
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(err.Error())
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	return SSHKey{
		Pub:  publicKeyBytes,
		Priv: privateKeyBytes,
	}
}

func create_secret_env(env string, secret string, key string) apiv1.EnvVar {
	if key == "" {
		key = secret
	}
	return apiv1.EnvVar{
		Name: env,
		ValueFrom: &apiv1.EnvVarSource{
			SecretKeyRef: &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: secret,
				},
				Key: key,
			},
		},
	}
}

var defaultPodSecurityContext = apiv1.PodSecurityContext{
	RunAsNonRoot: pointer.Bool(true),
	SeccompProfile: &apiv1.SeccompProfile{
		Type: "RuntimeDefault",
	},
}

var defaultContainerSecurityContext = apiv1.SecurityContext{
	AllowPrivilegeEscalation: pointer.Bool(false),
	Capabilities: &apiv1.Capabilities{
		Drop: []apiv1.Capability{
			"ALL",
		},
	},
}

func create_env(env string, value string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name:  env,
		Value: value,
	}
}

func create_container_port(port int, name string) apiv1.ContainerPort {
	return apiv1.ContainerPort{
		Name:          name,
		Protocol:      apiv1.ProtocolTCP,
		ContainerPort: int32(port),
	}
}

func create_volume_cm(volume_name string, config_map_ref string) apiv1.Volume {
	return apiv1.Volume{
		Name: volume_name,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: config_map_ref,
				},
			},
		},
	}
}

// Mounts specific ConfigMap keys on a volume.
//
// name - volume name ref
// config_map_ref - ConfigMap name ref
// keys - array of key to mount into the volume
// Each element of the array has a Key and a Path
//
//	Key - Reference to the ConfigMap Key name
//	Path - The relative path of the file to map the key to
func create_volume_cm_keys(volume_name string, config_map_ref string, keys []apiv1.KeyToPath) apiv1.Volume {
	return apiv1.Volume{
		Name: volume_name,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: config_map_ref,
				},
				Items: keys,
			},
		},
	}
}

func create_volume_secret(name string) apiv1.Volume {
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

func create_empty_dir(name string) apiv1.Volume {
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{},
		},
	}
}

func get_storage_classname(spec sfv1.SoftwareFactorySpec) string {
	if spec.StorageClassName != "" {
		return spec.StorageClassName
	} else {
		return "topolvm-provisioner"
	}
}

func create_pvc(ns string, name string, storageClassName string) apiv1.PersistentVolumeClaim {
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes:      []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": *resource.NewQuantity(1*1000*1000*1000, resource.DecimalSI),
				},
			},
		},
	}
}

// Create a default statefulset.
func create_statefulset(ns string, name string, image string, storageClassName string) appsv1.StatefulSet {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
	}
	pvc := create_pvc(ns, name, storageClassName)
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "sf",
					"run": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "sf",
						"run": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					AutomountServiceAccountToken: boolPtr(false),
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				pvc,
			},
		},
	}
}

// Create a default deployment.
func create_deployment(ns string, name string, image string) appsv1.Deployment {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "sf",
					"run": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "sf",
						"run": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					AutomountServiceAccountToken: boolPtr(false),
				},
			},
		},
	}
}

// create a default service.
func create_service(ns string, name string, selector string, port int32, port_name string) apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     port_name,
					Protocol: apiv1.ProtocolTCP,
					Port:     port,
				},
			},
			Selector: map[string]string{
				"app": "sf",
				"run": selector,
			},
		}}
}

// --- readiness probes (validate a pod is ready to serve) ---
func create_readiness_probe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:     handler,
		TimeoutSeconds:   3,
		PeriodSeconds:    5,
		FailureThreshold: 20,
	}
}

func create_readiness_cmd_probe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return create_readiness_probe(handler)
}

func create_readiness_http_probe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return create_readiness_probe(handler)
}

func create_readiness_https_probe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path:   path,
			Port:   intstr.FromInt(port),
			Scheme: apiv1.URISchemeHTTPS,
		}}
	return create_readiness_probe(handler)
}

func create_readiness_tcp_probe(port int) *apiv1.Probe {
	handler :=
		apiv1.ProbeHandler{
			TCPSocket: &apiv1.TCPSocketAction{
				Port: intstr.FromInt(port),
			}}
	return create_readiness_probe(handler)
}

// Get a resources, returning if it was found
func (r *SFController) GetM(name string, obj client.Object) bool {
	err := r.Get(r.ctx,
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

// Create resources with the software-factory as the ownerReferences.
func (r *SFController) CreateR(obj client.Object) {
	controllerutil.SetControllerReference(r.cr, obj, r.Scheme)
	if err := r.Create(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) DeleteR(obj client.Object) {
	if err := r.Delete(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) IsStatefulSetReady(dep *appsv1.StatefulSet) bool {
	if dep.Status.ReadyReplicas > 0 {
		var podList apiv1.PodList
		matchLabels := dep.Spec.Selector.MatchLabels
		labels := labels.SelectorFromSet(labels.Set(matchLabels))
		labelSelectors := runtimeClient.MatchingLabelsSelector{Selector: labels}
		if err := r.List(r.ctx, &podList, labelSelectors); err != nil {
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
				if containerStatus.Ready == false {
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
		return true
	}
	// No Replica available
	return false
}

func (r *SFController) IsDeploymentReady(dep *appsv1.Deployment) bool {
	if dep.Status.ReadyReplicas > 0 {
		return true
	}
	r.log.V(1).Info("Waiting for deployment", "name", dep.GetName())
	return false
}

func (r *SFController) DeleteDeployment(name string) {
	var dep appsv1.Deployment
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFController) DeleteStatefulSet(name string) {
	var dep appsv1.StatefulSet
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFController) DeleteConfigMap(name string) {
	var dep apiv1.ConfigMap
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFController) DeleteSecret(name string) {
	var dep apiv1.Secret
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFController) DeleteService(name string) {
	var srv apiv1.Service
	if r.GetM(name, &srv) {
		r.DeleteR(&srv)
	}
}

func (r *SFController) UpdateR(obj client.Object) {
	controllerutil.SetControllerReference(r.cr, obj, r.Scheme)
	r.log.V(1).Info("Updating object", "name", obj.GetName())
	if err := r.Update(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) PatchR(obj client.Object, patch client.Patch) {
	if err := r.Patch(r.ctx, obj, patch); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) DebugStatefulSet(name string) {
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

// This does not change an existing object, update needs to be used manually.
func (r *SFController) GetOrCreate(obj client.Object) {
	name := obj.GetName()

	if !r.GetM(name, obj) {
		r.log.V(1).Info("Creating object", "name", obj.GetName())
		r.CreateR(obj)
	}
}

// Create resource from YAML description
func (r *SFController) CreateYAML(y string) {
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(y), &obj); err != nil {
		panic(err.Error())
	}
	obj.SetNamespace(r.ns)
	controllerutil.SetControllerReference(r.cr, &obj, r.Scheme)
	r.GetOrCreate(&obj)
}

func (r *SFController) CreateYAMLs(ys string) {
	for _, y := range strings.Split(ys, "\n---\n") {
		r.CreateYAML(y)
	}
}

func (r *SFController) GenerateSecret(name string, getData func() string) apiv1.Secret {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating secret", "name", name)
		secret = apiv1.Secret{
			Data:       map[string][]byte{name: []byte(getData())},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns},
		}
		r.CreateR(&secret)
	}
	return secret
}

// generate a secret if needed using a uuid4 value.
func (r *SFController) GenerateSecretUUID(name string) apiv1.Secret {
	return r.GenerateSecret(name, func() string { return uuid.New().String() })
}

func (r *SFController) EnsureSSHKey(name string) {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating ssh key", "name", name)
		sshkey := create_ssh_key()
		secret = apiv1.Secret{
			Data: map[string][]byte{
				"priv": sshkey.Priv,
				"pub":  sshkey.Pub,
			},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns}}
		r.CreateR(&secret)
	}
}

// ensure a config map exists.
func (r *SFController) EnsureConfigMap(base_name string, data map[string]string) apiv1.ConfigMap {
	name := base_name + "-config-map"
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

func (r *SFController) EnsureSecret(secret *apiv1.Secret) {
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

func map_equals(m1 *map[string]string, m2 *map[string]string) bool {
	return reflect.DeepEqual(m1, m2)
}

// Merge m2 values into m1, return true if the map is updated.
func map_ensure(m1 *map[string]string, m2 *map[string]string) bool {
	dirty := false
	for k, v := range *m2 {
		if (*m1)[k] != v {
			dirty = true
			(*m1)[k] = v
		}
	}
	return dirty
}

//go:embed static/certificate-authority/certs.yaml
var ca_objs string

func (r *SFController) EnsureCA() {
	r.CreateYAMLs(ca_objs)
}

func create_job(ns string, name string, container apiv1.Container) batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						container,
					},
					RestartPolicy: "Never",
				},
			},
		}}
}

func (r *SFController) ensure_route(route apiroutev1.Route, name string) {
	found := r.GetM(name, &route)
	if !found {
		r.CreateR(&route)
	} else {
		if err := r.Update(r.ctx, &route); err != nil {
			panic(err.Error())
		}
	}
}

func (r *SFController) ensureHTTPSRoute(name string, host string, serviceName string, path string, port int, annotations map[string]string) {
	route := apiroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   r.ns,
			Annotations: annotations,
		},
		Spec: apiroutev1.RouteSpec{
			TLS: &apiroutev1.TLSConfig{
				InsecureEdgeTerminationPolicy: apiroutev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   apiroutev1.TLSTerminationEdge,
			},
			Host: host + "." + r.cr.Spec.FQDN,
			To: apiroutev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Port: &apiroutev1.RoutePort{
				TargetPort: intstr.FromInt(port),
			},
			Path: path,
		},
	}
	r.ensure_route(route, name)
}

// Get the service clusterIP. Return an empty string if service not found.
func (r *SFController) get_service_ip(service string) string {
	var obj apiv1.Service
	found := r.GetM(service, &obj)
	if !found {
		return ""
	}
	return obj.Spec.ClusterIP
}

func gen_bcrypt_pass(pass string) string {
	password := []byte(pass)
	hashedPassword, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hashedPassword)
}

func (r *SFController) PodExec(pod string, container string, command []string) error {
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
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *SFController) create_client_certificate(ns string, name string, issuer string, secret string, servicename string) certv1.Certificate {
	return certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.CertificateSpec{
			CommonName: "client",
			SecretName: secret,
			PrivateKey: &certv1.CertificatePrivateKey{
				Encoding: certv1.PKCS8,
			},
			IssuerRef: certmetav1.ObjectReference{
				Name: issuer,
				Kind: "Issuer",
			},
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageServerAuth,
				certv1.UsageClientAuth,
			},
			// Example DNSNames: service, service.my-sf, service.sftests.com, my-sf
			DNSNames: []string{
				servicename,
				fmt.Sprintf("%s.%s", servicename, r.cr.Name),
				fmt.Sprintf("%s.%s", servicename, r.cr.Spec.FQDN),
				r.cr.Name,
			},
		},
	}
}

// Gets Secret by Name Reference
func (r *SFController) getSecretbyNameRef(name string) (apiv1.Secret, error) {
	var dep apiv1.Secret
	if r.GetM(name, &dep) {
		return dep, nil
	}
	return apiv1.Secret{}, fmt.Errorf("secret name with ref %s not found", name)
}

// Gets the Value of the Keyname from a Secret
func (r *SFController) getValueFromKeySecret(secret apiv1.Secret, keyname string) ([]byte, error) {
	keyvalue := secret.Data[keyname]
	if len(keyvalue) == 0 {
		return []byte{}, fmt.Errorf("key named %s not found in Secret %s at namespace %s", keyname, secret.Name, secret.Namespace)
	}

	return keyvalue, nil
}

func (r *SFController) getSecretDataFromKey(name string, key string) ([]byte, error) {
	secret, err := r.getSecretbyNameRef(name)
	if err != nil {
		return []byte{}, err
	}
	var subkey string
	if key == "" {
		subkey = name
	} else {
		subkey = key
	}
	data, err := r.getValueFromKeySecret(secret, subkey)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Gets Secret Data in which the Keyname is the same as the Secret Name
func (r *SFController) getSecretData(name string) ([]byte, error) {
	return r.getSecretDataFromKey(name, "")
}

func (r *SFController) ImageToBase64(imagepath string) (string, error) {
	// Read the file to bytes
	bytes, err := os.ReadFile(imagepath)
	if err != nil {
		r.log.V(1).Error(err, "Something wrong while reading file "+imagepath)
		return "", err
	}

	var base64Encoding string

	mimeType := http.DetectContentType(bytes)

	switch mimeType {
	case "image/jpeg":
		base64Encoding += "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding += "data:image/png;base64,"
	}

	encodedimage := base64.StdEncoding.EncodeToString(bytes)

	return encodedimage, nil
}

func (r *SFController) getConfigRepoCNXInfo() (string, string) {
	var config_repo_url string
	var config_repo_user string
	if r.cr.Spec.ConfigLocations.ConfigRepo == "" {
		config_repo_url = "gerrit-sshd:29418/config"
		config_repo_user = "admin"
	} else if r.cr.Spec.ConfigLocations.ConfigRepo != "" {
		var user string
		if r.cr.Spec.ConfigLocations.User != "" {
			user = r.cr.Spec.ConfigLocations.User
		} else {
			user = "git"
		}
		config_repo_url = r.cr.Spec.ConfigLocations.ConfigRepo
		config_repo_user = user
	} else {
		// TODO: uncomment the panic once the config repo is actually working
		// panic("ConfigRepo settings not supported !")
	}
	return config_repo_url, config_repo_user
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
