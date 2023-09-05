// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains common helper functions

package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
	ini "gopkg.in/ini.v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/pointer"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apiroutev1 "github.com/openshift/api/route/v1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

const BUSYBOX_IMAGE = "quay.io/software-factory/sf-op-busybox:1.5-3"

// TODO this could be a spec parameter.
const SERVICEMONITOR_LABEL_SELECTOR = "sf-monitoring"

type SFUtilContext struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
	ns         string
	log        logr.Logger
	ctx        context.Context
	owner      client.Object
}

//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

func (r *SFUtilContext) create_ServiceMonitor(name string, port string, selector metav1.LabelSelector) monitoringv1.ServiceMonitor {
	return monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
			Labels: map[string]string{
				SERVICEMONITOR_LABEL_SELECTOR: name,
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: monitoringv1.Duration("30s"),
					Port:     port,
					Scheme:   "http",
				},
			},
			Selector: selector,
		},
	}
}

func (r *SFUtilContext) create_PodMonitor(name string, port string, selector metav1.LabelSelector) monitoringv1.PodMonitor {
	return monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
			Labels: map[string]string{
				SERVICEMONITOR_LABEL_SELECTOR: name,
			},
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: selector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Port: port,
				},
			},
		},
	}
}

func (r *SFUtilContext) create_PrometheusRuleCR(name string) monitoringv1.PrometheusRule {
	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
			Labels: map[string]string{
				SERVICEMONITOR_LABEL_SELECTOR: name,
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}
}

func create_PrometheusRuleGroup(name string, rules []monitoringv1.Rule) monitoringv1.RuleGroup {
	// d := monitoringv1.Duration(duration)
	return monitoringv1.RuleGroup{
		Name: name,
		// Interval: &d,
		Rules: rules,
	}
}

func create_PrometheusAlertRule(name string, expr intstr.IntOrString, forDuration string, labels map[string]string, annotations map[string]string) monitoringv1.Rule {
	f := monitoringv1.Duration(forDuration)
	return monitoringv1.Rule{
		Alert:       name,
		Expr:        expr,
		For:         &f,
		Labels:      labels,
		Annotations: annotations,
	}
}

func DEFAULT_QTY_1Gi() resource.Quantity {
	q, _ := resource.ParseQuantity("1Gi")
	return q
}

func checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

type SSHKey struct {
	Pub  []byte
	Priv []byte
}

type StorageConfig struct {
	StorageClassName string
	Size             resource.Quantity
}

// GetEnvVarValue returns the value of the named env var. Return an empty string when not found.
func GetEnvVarValue(varName string) (string, error) {
	ns, found := os.LookupEnv(varName)
	if !found {
		return "", fmt.Errorf("%s unable to find env var", varName)
	}
	return ns, nil
}

func getOperatorConditionName() string {
	value, _ := GetEnvVarValue("OPERATOR_CONDITION_NAME")
	return value
}

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

func new_condition(conditiontype string, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditiontype,
		Status:             metav1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}

// Function to add or update conditions of a resource
//
// If a condition of type t does not exist, it creates a new condition,
// otherwise updates with the new parameters

func refresh_condition(conditions *[]metav1.Condition, conditiontype string, status metav1.ConditionStatus, reason string, message string) {
	foundCondition := metav1.Condition{}
	for i, condition := range *conditions {
		if condition.Type == conditiontype {
			foundCondition = condition
			if condition.Status != status ||
				condition.Reason != reason ||
				condition.Message != message {
				(*conditions)[i].Status = status
				(*conditions)[i].Reason = reason
				(*conditions)[i].Message = message
				(*conditions)[i].LastTransitionTime = metav1.NewTime(time.Now())
			}
		}
	}
	if reflect.DeepEqual(foundCondition, metav1.Condition{}) {
		*conditions = append([]metav1.Condition{new_condition(conditiontype, reason, message)}, *conditions...)
	}
}

func updateConditions(conditions *[]metav1.Condition, condType string, ready bool) {
	var reason, message string
	var status metav1.ConditionStatus
	if ready {
		reason = "Complete"
		message = fmt.Sprintf("Initialization of %s service completed.", condType)
		status = metav1.ConditionTrue
	} else {
		reason = "Awaiting"
		message = fmt.Sprintf("Initializing %s service...", condType)
		status = metav1.ConditionFalse
	}
	refresh_condition(conditions, condType, status, reason, message)
}

// Function to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func Parse_string(text string, data any) (string, error) {
	// Create Template object
	template_body, err := template.New("StringtoParse").Parse(text)
	if err != nil {
		return "", fmt.Errorf("Text not in the right format: " + text)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template_body.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", text)
	}

	return buf.String(), nil
}

// Take one or more section name and compute a checkum
func IniSectionsChecksum(cfg *ini.File, names []string) string {

	var IniGetSectionBody = func(cfg *ini.File, section *ini.Section) string {
		var s string = ""
		keys := section.KeyStrings()
		sort.Strings(keys)
		for _, k := range keys {
			s = s + k + section.Key(k).String()
		}
		return s
	}

	var data string = ""
	for _, name := range names {
		section, err := cfg.GetSection(name)
		if err != nil {
			panic("No such ini section: " + name)
		}
		data += IniGetSectionBody(cfg, section)
	}

	return checksum([]byte(data))
}

// Get Ini section names filtered by prefix
func IniGetSectionNamesByPrefix(cfg *ini.File, prefix string) []string {
	filteredNames := []string{}
	names := cfg.SectionStrings()
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			filteredNames = append(filteredNames, n)
		}
	}
	return filteredNames
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

func Create_secret_env(env string, secret string, key string) apiv1.EnvVar {
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

func create_security_context(privileged bool) *apiv1.SecurityContext {
	return &apiv1.SecurityContext{
		Privileged:               pointer.Bool(privileged),
		AllowPrivilegeEscalation: pointer.Bool(privileged),
		Capabilities: &apiv1.Capabilities{
			Drop: []apiv1.Capability{
				"ALL",
			},
		},
	}
}

func Create_env(env string, value string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name:  env,
		Value: value,
	}
}

func Create_container_port(port int, name string) apiv1.ContainerPort {
	return apiv1.ContainerPort{
		Name:          name,
		Protocol:      apiv1.ProtocolTCP,
		ContainerPort: int32(port),
	}
}

func Create_volume_cm(volume_name string, config_map_ref string) apiv1.Volume {
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

func create_volume_secret(name string, secret_name ...string) apiv1.Volume {
	sec_name := name
	if secret_name != nil {
		sec_name = secret_name[0]
	}
	return apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: sec_name,
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

func get_storage_classname(storageClassName string) string {
	if storageClassName != "" {
		return storageClassName
	} else {
		return "topolvm-provisioner"
	}
}

func MkPVC(name string, ns string, storageParams StorageConfig, accessMode apiv1.PersistentVolumeAccessMode) apiv1.PersistentVolumeClaim {
	qty := storageParams.Size
	return apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			StorageClassName: &storageParams.StorageClassName,
			AccessModes:      []apiv1.PersistentVolumeAccessMode{accessMode},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": qty,
				},
			},
		},
	}
}

func (r *SFUtilContext) create_pvc(name string, storageParams StorageConfig, accessMode apiv1.PersistentVolumeAccessMode) apiv1.PersistentVolumeClaim {
	return MkPVC(name, r.ns, storageParams, accessMode)
}

func MkStatefulset(
	name string, ns string, replicas int32, service_name string,
	container apiv1.Container, pvc apiv1.PersistentVolumeClaim) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    int32Ptr(replicas),
			ServiceName: service_name,
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
					SecurityContext: &defaultPodSecurityContext,
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

func MkContainer(name string, image string) apiv1.Container {
	return apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: create_security_context(false),
	}
}

// Create a default statefulset.
func (r *SFUtilContext) create_statefulset(name string, image string, storageConfig StorageConfig, replicas int32, accessMode apiv1.PersistentVolumeAccessMode, nameSuffix ...string) appsv1.StatefulSet {
	service_name := name
	if nameSuffix != nil {
		service_name = name + "-" + nameSuffix[0]
	}

	if replicas == 0 {
		replicas = 1
	}

	container := MkContainer(name, image)
	pvc := r.create_pvc(name, storageConfig, accessMode)
	return MkStatefulset(name, r.ns, replicas, service_name, container, pvc)
}

// Create a default headless statefulset.
func (r *SFUtilContext) create_headless_statefulset(name string, image string, storageConfig StorageConfig, replicas int32, accessMode apiv1.PersistentVolumeAccessMode) appsv1.StatefulSet {
	return r.create_statefulset(name, image, storageConfig, replicas, accessMode, "headless")
}

// Create a default deployment.
func (r *SFUtilContext) create_deployment(name string, image string) appsv1.Deployment {
	container := apiv1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: create_security_context(false),
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
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
					SecurityContext:              &defaultPodSecurityContext,
				},
			},
		},
	}
}

// create a default service.
func (r *SFUtilContext) create_service(name string, selector string, ports []int32, port_name string) apiv1.Service {
	service_ports := []apiv1.ServicePort{}
	for _, p := range ports {
		service_ports = append(
			service_ports,
			apiv1.ServicePort{
				Name:     fmt.Sprintf("%s-%d", port_name, p),
				Protocol: apiv1.ProtocolTCP,
				Port:     p,
			})
	}
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			Ports: service_ports,
			Selector: map[string]string{
				"app": "sf",
				"run": selector,
			},
		}}
}

// create a headless service.
func (r *SFUtilContext) create_headless_service(name string, selector string, ports []int32, port_name string) apiv1.Service {
	service_ports := []apiv1.ServicePort{}
	for _, p := range ports {
		service_ports = append(
			service_ports,
			apiv1.ServicePort{
				Name:     fmt.Sprintf("%s-%d", port_name, p),
				Protocol: apiv1.ProtocolTCP,
				Port:     p,
			})
	}

	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-headless",
			Namespace: r.ns,
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "None",
			Ports:     service_ports,
			Selector: map[string]string{
				"app": "sf",
				"run": selector,
			},
		},
	}
}

// --- readiness probes (validate a pod is ready to serve) ---
func create_readiness_probe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:     handler,
		TimeoutSeconds:   5,
		PeriodSeconds:    10,
		FailureThreshold: 20,
	}
}

func Create_readiness_cmd_probe(cmd []string) *apiv1.Probe {
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

// --- liveness probes (validate a pod is up and running) ---
func create_liveness_probe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:        handler,
		TimeoutSeconds:      5,
		PeriodSeconds:       20,
		InitialDelaySeconds: 5,
	}
}

func Create_liveness_cmd_probe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return create_liveness_probe(handler)
}

func create_liveness_http_probe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return create_liveness_probe(handler)
}

// --- startup probes (validate when pod has been started running) ---
func create_startup_probe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:        handler,
		TimeoutSeconds:      2,
		PeriodSeconds:       20,
		FailureThreshold:    10,
		InitialDelaySeconds: 5,
	}
}

func Create_startup_cmd_probe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return create_startup_probe(handler)
}

func create_startup_http_probe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return create_startup_probe(handler)
}

// Get a resources, returning if it was found
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

// Create resources with the owner as the ownerReferences.
func (r *SFUtilContext) CreateR(obj client.Object) {
	controllerutil.SetControllerReference(r.owner, obj, r.Scheme)
	if err := r.Client.Create(r.ctx, obj); err != nil && !k8s_errors.IsAlreadyExists(err) {
		panic(err.Error())
	}
}

func (r *SFUtilContext) DeleteR(obj client.Object) {
	if err := r.Client.Delete(r.ctx, obj); err != nil {
		panic(err.Error())
	}
}

func IsStatefulSetRolloutDone(obj *appsv1.StatefulSet) bool {
	return obj.Status.ObservedGeneration >= obj.Generation &&
		obj.Status.Replicas == obj.Status.ReadyReplicas &&
		obj.Status.Replicas == obj.Status.CurrentReplicas
}

func IsDeploymentRolloutDone(obj *appsv1.Deployment) bool {
	return obj.Status.ObservedGeneration >= obj.Generation &&
		obj.Status.Replicas == obj.Status.ReadyReplicas &&
		obj.Status.Replicas == obj.Status.AvailableReplicas
}

func (r *SFUtilContext) IsStatefulSetReady(dep *appsv1.StatefulSet) bool {
	if dep.Status.ReadyReplicas > 0 {
		var podList apiv1.PodList
		matchLabels := dep.Spec.Selector.MatchLabels
		labels := labels.SelectorFromSet(labels.Set(matchLabels))
		labelSelectors := runtimeClient.MatchingLabelsSelector{Selector: labels}
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
		return true && IsStatefulSetRolloutDone(dep)
	}
	// No Replica available
	return false
}

func (r *SFUtilContext) IsDeploymentReady(dep *appsv1.Deployment) bool {
	if dep.Status.ReadyReplicas > 0 {
		return true && IsDeploymentRolloutDone(dep)
	}
	r.log.V(1).Info("Waiting for deployment", "name", dep.GetName())
	return false
}

func (r *SFUtilContext) DeleteDeployment(name string) {
	var dep appsv1.Deployment
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFUtilContext) DeleteStatefulSet(name string) {
	var dep appsv1.StatefulSet
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFUtilContext) DeleteConfigMap(name string) {
	var dep apiv1.ConfigMap
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFUtilContext) DeleteSecret(name string) {
	var dep apiv1.Secret
	if r.GetM(name, &dep) {
		r.DeleteR(&dep)
	}
}

func (r *SFUtilContext) DeleteService(name string) {
	var srv apiv1.Service
	if r.GetM(name, &srv) {
		r.DeleteR(&srv)
	}
}

func (r *SFUtilContext) DeleteRoute(name string) {
	var route apiroutev1.Route
	if r.GetM(name, &route) {
		r.DeleteR(&route)
	}
}

func (r *SFUtilContext) UpdateR(obj client.Object) bool {
	controllerutil.SetControllerReference(r.owner, obj, r.Scheme)
	r.log.V(1).Info("Updating object", "name", obj.GetName())
	if err := r.Client.Update(r.ctx, obj); err != nil {
		r.log.Error(err, "Unable to update the object")
		return false
	}
	return true
}

func (r *SFUtilContext) PatchR(obj client.Object, patch client.Patch) {
	if err := r.Client.Patch(r.ctx, obj, patch); err != nil {
		panic(err.Error())
	}
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

// This does not change an existing object, update needs to be used manually.
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

// Create resource from YAML description
func (r *SFUtilContext) CreateYAML(y string) {
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(y), &obj); err != nil {
		panic(err.Error())
	}
	obj.SetNamespace(r.ns)
	r.GetOrCreate(&obj)
}

func (r *SFUtilContext) CreateYAMLs(ys string) {
	for _, y := range strings.Split(ys, "\n---\n") {
		r.CreateYAML(y)
	}
}

func CreateSecretFromFunc(name string, namespace string, getData func() string) apiv1.Secret {
	var secret apiv1.Secret
	secret = apiv1.Secret{
		Data:       map[string][]byte{name: []byte(getData())},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return secret
}

func (r *SFUtilContext) GenerateSecret(name string, getData func() string) apiv1.Secret {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating secret", "name", name)
		secret = CreateSecretFromFunc(name, r.ns, getData)
		r.CreateR(&secret)
	}
	return secret
}

func NewUUIDString() string {
	return uuid.New().String()
}

// generate a secret if needed using a uuid4 value.
func (r *SFUtilContext) GenerateSecretUUID(name string) apiv1.Secret {
	return r.GenerateSecret(name, NewUUIDString)
}

func CreateSSHKeySecret(name string, namespace string) apiv1.Secret {
	var secret apiv1.Secret
	sshkey := create_ssh_key()
	secret = apiv1.Secret{
		Data: map[string][]byte{
			"priv": sshkey.Priv,
			"pub":  sshkey.Pub,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return secret
}

func (r *SFUtilContext) EnsureSSHKey(name string) {
	var secret apiv1.Secret
	if !r.GetM(name, &secret) {
		r.log.V(1).Info("Creating ssh key", "name", name)
		secret := CreateSSHKeySecret(name, r.ns)
		r.CreateR(&secret)
	}
}

// ensure a config map exists.
func (r *SFUtilContext) EnsureConfigMap(base_name string, data map[string]string) apiv1.ConfigMap {
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

func (r *SFUtilContext) EnsureCA() {
	r.CreateYAMLs(ca_objs)
}

func MkJob(name string, ns string, container apiv1.Container) batchv1.Job {
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
					RestartPolicy:   "Never",
					SecurityContext: &defaultPodSecurityContext,
				},
			},
		}}
}

func (r *SFUtilContext) create_job(name string, container apiv1.Container) batchv1.Job {
	return MkJob(name, r.ns, container)
}

// This function returns false when the resource is just created/updated
func (r *SFUtilContext) ensure_route(route apiroutev1.Route, name string) bool {
	current := apiroutev1.Route{}
	found := r.GetM(name, &current)
	if !found {
		r.log.V(1).Info("Creating route...", "name", name)
		r.CreateR(&route)
		return false
	} else {
		// Route already exist - check if we need to update the Route
		// Use the String repr of the RouteSpec to compare for changes
		// This comparaison mechanics may fail in case of some Route Spec default values
		// not specified in the wanted version.
		wanted_repr := route.Spec.String()
		current_repr := current.Spec.String()
		if wanted_repr != current_repr {
			r.log.V(1).Info("Updating route...", "name", name)
			current.Spec = route.Spec
			r.UpdateR(&current)
			return false
		}
	}
	return true
}

func (r *SFUtilContext) getRouteByName(routeName string) (*apiroutev1.Route, error) {
	route := &apiroutev1.Route{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: r.ns,
		Name:      routeName,
	}, route)

	return route, err
}

func MkHTTSRoute(
	name string, ns string, host string, serviceName string, path string,
	port int, annotations map[string]string, fqdn string, customTls *apiroutev1.TLSConfig) apiroutev1.Route {
	tls := apiroutev1.TLSConfig{
		InsecureEdgeTerminationPolicy: apiroutev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   apiroutev1.TLSTerminationEdge,
	}
	if customTls != nil {
		tls = *customTls
	}
	return apiroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
		Spec: apiroutev1.RouteSpec{
			TLS:  &tls,
			Host: host + "." + fqdn,
			To: apiroutev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: pointer.Int32(100),
			},
			Port: &apiroutev1.RoutePort{
				TargetPort: intstr.FromInt(port),
			},
			Path:           path,
			WildcardPolicy: "None",
		},
	}
}

func GetCustomRouteSSLSecretName(host string) string {
	return host + "-ssl-cert"
}

func getLetsEncryptServer(le sfv1.LetsEncryptSpec) (string, string) {
	serverUrl := ""
	name := ""
	switch server := le.Server; server {
	case sfv1.LEServerProd:
		serverUrl = "https://acme-v02.api.letsencrypt.org/directory"
		name = "cm-le-issuer-production"
	case sfv1.LEServerStaging:
		serverUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"
		name = "cm-le-issuer-staging"
	}
	return serverUrl, name
}

func (r *SFUtilContext) extractStaticTLSFromSecret(name string, host string) (bool, []byte, []byte, []byte) {
	var customSSLSecret apiv1.Secret
	customSSLSecretName := GetCustomRouteSSLSecretName(host)

	// We set a place holder secret to ensure that the Secret is owned (ControllerReference)
	// Or we adopt the existing secret
	if !r.GetM(customSSLSecretName, &customSSLSecret) {
		r.CreateR(&apiv1.Secret{
			Data:       map[string][]byte{},
			ObjectMeta: metav1.ObjectMeta{Name: customSSLSecretName, Namespace: r.ns}})
		return false, nil, nil, nil
	} else {
		if len(customSSLSecret.GetOwnerReferences()) == 0 {
			r.log.V(1).Info("Adopting the route secret to set the owner reference", "secret", customSSLSecretName, "route name", name)
			if !r.UpdateR(&customSSLSecret) {
				return false, nil, nil, nil
			}
		}
	}
	// Fetching secret expected TLS Keys content
	return true, customSSLSecret.Data["CA"], customSSLSecret.Data["crt"], customSSLSecret.Data["key"]
}

func (r *SFUtilContext) extractTLSFromLECertificateSecret(name string, host string, fqdn string, le sfv1.LetsEncryptSpec) (bool, []byte, []byte, []byte) {
	_, issuerName := getLetsEncryptServer(le)
	dnsNames := []string{host + "." + fqdn}
	certificate := MKCertificate(name, r.ns, issuerName, dnsNames)

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
		ready := isCertificateReady(&current)

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

// This function returns false when the controller reconcile loop must be re-triggered.
// Returns true when the Route is 'Ready'
func (r *SFUtilContext) ensureHTTPSRoute(
	name string, host string, serviceName string, path string,
	port int, annotations map[string]string, fqdn string, le *sfv1.LetsEncryptSpec) bool {

	var tlsDataReady bool
	var sslCA, sslCrt, sslKey []byte

	if le == nil {
		// Letsencrypt config has not been set so we check the `customSSLSecretName` Secret
		// for any custom TLS data to setup the Route
		tlsDataReady, sslCA, sslCrt, sslKey = r.extractStaticTLSFromSecret(name, host)
	} else {
		// Letsencrypt config has been set so we ensure we set a Certificate via the
		// cert-manager Issuer and then we'll setup the Route based on the Certificate's Secret
		tlsDataReady, sslCA, sslCrt, sslKey = r.extractTLSFromLECertificateSecret(name, host, fqdn, *le)
	}

	if !tlsDataReady {
		return false
	}

	route := apiroutev1.Route{}

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
		route = MkHTTSRoute(name, r.ns, host, serviceName, path, port, annotations, fqdn, &tls)
	} else {
		route = MkHTTSRoute(name, r.ns, host, serviceName, path, port, annotations, fqdn, nil)
	}
	return r.ensure_route(route, name)
}

// Get the service clusterIP. Return an empty string if service not found.
func (r *SFUtilContext) get_service_ip(service string) string {
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

func (r *SFUtilContext) create_client_certificate(
	name string, issuer string, secret string, servicename string, fqdn string) certv1.Certificate {
	return certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.ns,
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
				fmt.Sprintf("%s.%s", servicename, r.owner.GetName()),
				fmt.Sprintf("%s.%s", servicename, fqdn),
				r.owner.GetName(),
			},
		},
	}
}

// TODO: merge with create_client_certificate function above
func MKCertificate(name string, ns string, issuerName string, dnsNames []string) certv1.Certificate {
	return certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.CertificateSpec{
			DNSNames:   dnsNames,
			SecretName: name + "-tls",
			IssuerRef: certmetav1.ObjectReference{
				Kind: "Issuer",
				Name: issuerName,
			},
			IsCA: false,
			Usages: []certv1.KeyUsage{
				certv1.UsageServerAuth,
			},
		},
	}
}

func isCertificateReady(cert *certv1.Certificate) bool {
	for _, condition := range cert.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func MKLetsEncryptIssuer(name string, ns string, server string) certv1.Issuer {
	return certv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certv1.IssuerSpec{
			IssuerConfig: certv1.IssuerConfig{
				ACME: &cmacme.ACMEIssuer{
					Server:         server,
					PreferredChain: "ISRG Root X1",
					PrivateKey: cmmeta.SecretKeySelector{
						LocalObjectReference: cmmeta.LocalObjectReference{
							Name: name,
						},
					},
					Solvers: []cmacme.ACMEChallengeSolver{
						{
							HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
								Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{},
							},
						},
					},
				},
			},
		},
	}
}

// Gets Secret by Name Reference
func (r *SFUtilContext) getSecretbyNameRef(name string) (apiv1.Secret, error) {
	var dep apiv1.Secret
	if r.GetM(name, &dep) {
		return dep, nil
	}
	return apiv1.Secret{}, fmt.Errorf("secret name with ref %s not found", name)
}

// Gets the Value of the Keyname from a Secret
func GetValueFromKeySecret(secret apiv1.Secret, keyname string) ([]byte, error) {
	keyvalue := secret.Data[keyname]
	if len(keyvalue) == 0 {
		return []byte{}, fmt.Errorf("key named %s not found in Secret %s at namespace %s", keyname, secret.Name, secret.Namespace)
	}

	return keyvalue, nil
}

func (r *SFUtilContext) getSecretDataFromKey(name string, key string) ([]byte, error) {
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
	data, err := GetValueFromKeySecret(secret, subkey)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Gets Secret Data in which the Keyname is the same as the Secret Name
func (r *SFUtilContext) getSecretData(name string) ([]byte, error) {
	return r.getSecretDataFromKey(name, "")
}

func (r *SFUtilContext) ImageToBase64(imagepath string) (string, error) {
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

func BaseGetStorageConfOrDefault(storageSpec sfv1.StorageSpec, storageClassName string) StorageConfig {
	var size = DEFAULT_QTY_1Gi()
	var className = get_storage_classname(storageClassName)
	if !storageSpec.Size.IsZero() {
		size = storageSpec.Size
	}
	if storageSpec.ClassName != "" {
		className = storageSpec.ClassName
	}
	return StorageConfig{
		StorageClassName: className,
		Size:             size,
	}
}

func (r *SFUtilContext) reconcile_expand_pvc(pvc_name string, newStorageSpec sfv1.StorageSpec) bool {
	new_qty := newStorageSpec.Size

	found_pvc := &apiv1.PersistentVolumeClaim{}
	if !r.GetM(pvc_name, found_pvc) {
		r.log.V(1).Info("PVC " + pvc_name + " not found")
		return false
	}
	r.log.V(1).Info("Inspecting volume " + found_pvc.Name)

	current_qty := found_pvc.Status.Capacity.Storage()
	if current_qty.IsZero() {
		// When the PVC is just created but not fully up it might happen that the current
		// size is 0. So let force a new reconcile.
		return false
	}

	// Is a resize in progress?
	for _, condition := range found_pvc.Status.Conditions {
		switch condition.Type {
		case
			apiv1.PersistentVolumeClaimResizing,
			apiv1.PersistentVolumeClaimFileSystemResizePending:
			r.log.V(1).Info("Volume resizing in progress, not ready")
			return false
		}
	}

	switch new_qty.Cmp(*current_qty) {
	case -1:
		r.log.V(1).Info("Cannot downsize volume " + pvc_name)
		return true
	case 0:
		r.log.V(1).Info("Volume " + pvc_name + " at expected size, nothing to do")
		return true
	case 1:
		r.log.V(1).Info("Volume expansion required for  " + pvc_name +
			". current size: " + current_qty.String() + " -> new size: " + new_qty.String())
		new_resources := apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				"storage": new_qty,
			},
		}
		found_pvc.Spec.Resources = new_resources
		if err := r.Client.Update(r.ctx, found_pvc); err != nil {
			r.log.V(1).Error(err, "Updating PVC failed for volume  "+pvc_name)
			return false
		}
		// We return false to notify that a volume expansion was just
		// requested. Technically we could consider the reconcile is
		// over as most storage classes support hot resizing without
		// service interruption.
		r.log.V(1).Info("Expansion started for volume " + pvc_name)
		return false
	}
	return true
}

func (r *SFUtilContext) ensureLetsEncryptIssuer(le sfv1.LetsEncryptSpec) bool {
	server, name := getLetsEncryptServer(le)
	issuer := MKLetsEncryptIssuer(name, r.ns, server)
	return r.GetOrCreate(&issuer)
}

// SFController struct-context scoped utils //

func (r *SFController) getStorageConfOrDefault(storageSpec sfv1.StorageSpec) StorageConfig {
	return BaseGetStorageConfOrDefault(storageSpec, r.cr.Spec.StorageClassName)
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

var Execmod int32 = 493 // decimal for 0755 octal

func (r *SFController) isConfigRepoSet() bool {
	return r.cr.Spec.ConfigLocation.BaseURL != "" &&
		r.cr.Spec.ConfigLocation.Name != "" &&
		r.cr.Spec.ConfigLocation.ZuulConnectionName != ""
}
