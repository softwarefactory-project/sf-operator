// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
)

func checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type SSHKey struct {
	Pub  []byte
	Priv []byte
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

func create_volume_cm(name string, config_map_ref string) apiv1.Volume {
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: config_map_ref,
				},
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

// Create a default persistent volume claim.
// With kind, PVC should be automatically provisioned with https://github.com/rancher/local-path-provisioner
// If the PVC is stuck in `Pending`, then a local hostPath must be created, e.g.:
/*
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: pv-kind4
   spec:
     storageClassName: standard
     accessModes:
       - ReadWriteOnce
     capacity:
       storage: 1Gi
     hostPath:
       path: /src/pvs4
*/
func create_pvc(ns string, name string) apiv1.PersistentVolumeClaim {
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			StorageClassName: strPtr("standard"),
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
func create_statefulset(ns string, name string, image string) appsv1.StatefulSet {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
	}
	pvc := create_pvc(ns, name)
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

func create_readiness_http_probe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
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

// Create resources with the controller reference
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

func (r *SFController) IsStatefulSetReady(name string) bool {
	var dep appsv1.StatefulSet
	if r.GetM(name, &dep) {
		if dep.Status.ReadyReplicas > 0 {
			return true
		}
		r.log.V(1).Info("Waiting for statefulset", "name", name)
	}
	return false
}

func (r *SFController) IsDeploymentReady(name string) bool {
	var dep appsv1.Deployment
	if r.GetM(name, &dep) {
		if dep.Status.ReadyReplicas > 0 {
			return true
		}
		r.log.V(1).Info("Waiting for deployment", "name", name)
	}
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

func (r *SFController) DeleteService(name string) {
	var srv apiv1.Service
	if r.GetM(name, &srv) {
		r.DeleteR(&srv)
	}
}

func (r *SFController) UpdateR(obj client.Object) {
	controllerutil.SetControllerReference(r.cr, obj, r.Scheme)
	if err := r.Update(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) PatchR(obj client.Object, patch client.Patch) {
	if err := r.Patch(r.ctx, obj, patch); err != nil {
		panic(err.Error())
	}
}

func (r *SFController) Apply(desired client.Object) {
	var obj unstructured.Unstructured

	// get gvk
	gvk, err := apiutil.GVKForObject(desired, r.Scheme)
	if err != nil {
		panic(err.Error())
	}
	obj.SetGroupVersionKind(gvk)
	name := desired.GetName()

	if r.GetM(name, &obj) {
		// desired.SetResourceVersion(obj.GetResourceVersion())
		if gvk.Kind != "Issuer" && gvk.Kind != "Certificate" {
			// r.log.V(1).Info("Updating object", "name", name, "gvk", gvk)
			r.UpdateR(desired)
		}
	} else {
		r.log.V(1).Info("Creating object", "name", name, "gkv", gvk)
		r.CreateR(desired)
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
	r.Apply(&obj)
}

func (r *SFController) CreateYAMLs(ys string) {
	for _, y := range strings.Split(ys, "\n---\n") {
		r.CreateYAML(y)
	}
}

// generate a secret if needed using a uuid4 value.
func (r *SFController) EnsureSecret(name string) apiv1.Secret {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating secret", "name", name)
		secret = apiv1.Secret{
			Data: map[string][]byte{
				// The data key is the same as the secret name.
				// This means that a Secret object presently only contains a single value.
				name: []byte(uuid.New().String()),
			},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.ns}}
		// We don't use CreateR to not own the resource, and keep it after deletion
		if err := r.Create(r.ctx, &secret); err != nil {
			panic(err.Error())
		}
	}
	return secret
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
		// We don't use CreateR to not own the resource, and keep it after deletion
		if err := r.Create(r.ctx, &secret); err != nil {
			panic(err.Error())
		}
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

func create_ingress_rule(host string, service string, port int) netv1.IngressRule {
	pt := netv1.PathTypePrefix
	return netv1.IngressRule{
		Host: host,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						PathType: &pt,
						Path:     "/",
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{
								Name: service,
								Port: netv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *SFController) SetupIngress(keycloakEnabled bool) {
	var ingress netv1.Ingress
	found := r.GetM(r.cr.Name, &ingress)
	ingress = netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.cr.Name,
			Namespace: r.ns,
		},
	}
	if r.cr.Spec.Etherpad {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressEtherpad())
	}
	if keycloakEnabled {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressKeycloak()...)
	}
	if r.cr.Spec.Gerrit {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressGerrit()...)
	}
	if r.cr.Spec.Zuul {
		ingress.Spec.Rules = append(ingress.Spec.Rules, r.IngressZuul())
	}
	if !found {
		r.CreateR(&ingress)
	} else {
		if err := r.Update(r.ctx, &ingress); err != nil {
			panic(err.Error())
		}
	}
}

func (r *SFController) PodExec(pod string, container string, command []string) {
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
		panic(err.Error())
	}

	// r.log.V(1).Info("Streaming start")
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		panic(err.Error())
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
