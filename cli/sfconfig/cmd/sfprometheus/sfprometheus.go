// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package sfprometheus manages a basic prometheus instance for monitoring.
package sfprometheus

import (
	"context"
	"fmt"

	_ "embed"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/softwarefactory-project/sf-operator/controllers"
)

type PromCMDContext struct {
	env  *utils.ENV
	fqdn string
}

var ns = "sf"

const PROMETHEUS_NAME = "prometheus"
const PROMETHEUS_PORT = 9090

func EnsurePrometheusOperator(env *utils.ENV) error {
	fmt.Println("Ensure prometheus operator is present...")
	promOpMatchLabels := map[string]string{
		"app.kubernetes.io/name": "prometheus-operator",
	}
	labels := labels.SelectorFromSet(labels.Set(promOpMatchLabels))
	labelSelectors := client.MatchingLabelsSelector{Selector: labels}
	var podList apiv1.PodList
	err := env.Cli.List(env.Ctx, &podList, labelSelectors)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		fmt.Println("Prometheus operator not installed. Installing...")
		utils.RunMake("install-prometheus-operator")
	} else {
		fmt.Println("Prometheus operator is already present.")
	}
	return nil
}

func EnsurePrometheusInstanceServiceAccount(env *utils.ENV) {
	utils.EnsureServiceAccount(env, PROMETHEUS_NAME)
}

func EnsurePrometheusInstanceClusterRole(env *utils.ENV) {
	cr := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{APIVersion: rbac.SchemeGroupVersion.String(), Kind: "ClusterRole"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PROMETHEUS_NAME,
			Namespace: env.Ns,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Name:      cr.GetObjectMeta().GetName(),
		Namespace: env.Ns,
	}, &rbac.ClusterRole{})
	if errors.IsNotFound(err) {
		fmt.Println("Creating Role \"prometheus\"...")
		env.Cli.Create(env.Ctx, cr)
	} else if err != nil {
		panic(err)
	}
}

func EnsurePrometheusInstanceCRBinding(env *utils.ENV) {
	crbinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbac.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PROMETHEUS_NAME,
			Namespace: env.Ns,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     PROMETHEUS_NAME,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PROMETHEUS_NAME,
				Namespace: env.Ns,
			},
		},
	}
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Name:      crbinding.GetObjectMeta().GetName(),
		Namespace: env.Ns,
	}, &rbac.ClusterRoleBinding{})
	if errors.IsNotFound(err) {
		fmt.Println("Creating Role Binding \"prometheus\"...")
		env.Cli.Create(env.Ctx, crbinding)
	} else if err != nil {
		panic(err)
	}
}

func EnsurePrometheusInstance(env *utils.ENV) {
	memRequest, _ := resource.ParseQuantity("400Mi")
	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PROMETHEUS_NAME,
			Namespace: env.Ns,
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ServiceAccountName: PROMETHEUS_NAME,
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      controllers.SERVICEMONITOR_LABEL_SELECTOR,
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
				ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
				PodMonitorSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "sf-monitoring",
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
				PodMonitorNamespaceSelector: &metav1.LabelSelector{},
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						apiv1.ResourceMemory: memRequest,
					},
				},
			},
			RuleSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "sf-monitoring",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			RuleNamespaceSelector: &metav1.LabelSelector{},
			EnableAdminAPI:        true,
		},
	}

	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Name:      prom.GetObjectMeta().GetName(),
		Namespace: env.Ns,
	}, &monitoringv1.Prometheus{})
	if errors.IsNotFound(err) {
		fmt.Println("Creating Prometheus instance...")
		env.Cli.Create(env.Ctx, prom)
	} else if err != nil {
		panic(err)
	}
}

func EnsurePrometheusService(env *utils.ENV) {
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PROMETHEUS_NAME,
			Namespace: env.Ns,
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				{
					Name:       PROMETHEUS_NAME + "-web",
					Protocol:   apiv1.ProtocolTCP,
					Port:       PROMETHEUS_PORT,
					TargetPort: intstr.FromString("web"),
				},
			},
			Selector: map[string]string{
				"prometheus": PROMETHEUS_NAME,
			},
		},
	}
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Name:      service.GetObjectMeta().GetName(),
		Namespace: env.Ns,
	}, &apiv1.Service{})
	if errors.IsNotFound(err) {
		fmt.Println("Creating Prometheus service...")
		if err2 := env.Cli.Create(env.Ctx, service); err2 != nil {
			panic(err2)
		}
	} else if err != nil {
		panic(err)
	}
}

func (p *PromCMDContext) EnsurePrometheusRoute() {
	route := controllers.MkHTTSRoute(
		PROMETHEUS_NAME, p.env.Ns, PROMETHEUS_NAME, PROMETHEUS_NAME, "/", PROMETHEUS_PORT, map[string]string{}, p.fqdn, nil)
	err := p.env.Cli.Get(p.env.Ctx, client.ObjectKey{
		Name:      PROMETHEUS_NAME,
		Namespace: p.env.Ns,
	}, &apiroutev1.Route{})
	if errors.IsNotFound(err) {
		fmt.Println("Creating route...")
		p.env.Cli.Create(p.env.Ctx, &route)
	} else if err != nil {
		panic(err)
	}
}

func EnsurePrometheus(env *utils.ENV, fqdn string, skipOperator bool) {
	p := PromCMDContext{
		env:  env,
		fqdn: fqdn,
	}

	if !skipOperator {
		EnsurePrometheusOperator(p.env)
	}
	// Deploying a Prometheus instance requires a dedicated Service Account with the appropriate RBAC
	EnsurePrometheusInstanceServiceAccount(p.env)
	EnsurePrometheusInstanceClusterRole(p.env)
	EnsurePrometheusInstanceCRBinding(p.env)
	// Deploy the actual instance and make it reachable via HTTP
	EnsurePrometheusInstance(p.env)
	EnsurePrometheusService(p.env)
	p.EnsurePrometheusRoute()
	fmt.Println("Prometheus instance available at https://prometheus." + fqdn)
}

// PrometheusCmd represents the prometheus command
var PrometheusCmd = &cobra.Command{
	Use:   "prometheus",
	Short: "Deploy a demo prometheus instance for service monitoring",
	Long: `This command can be used to deploy a prometheus instance in the sf
namespace. It will first install the prometheus operator in the operators namespace
if required.

The sf namespace must exist prior to running this command.

The prometheus dashboard will be accessible at https://prometheus.<fqdn>.`,
	Run: func(cmd *cobra.Command, args []string) {
		fqdn, _ := cmd.Flags().GetString("fqdn")
		skipOperator, _ := cmd.Flags().GetBool("skip-operator-setup")

		// Get the kube client
		cl := utils.CreateKubernetesClientOrDie("")
		ctx := context.Background()
		env := utils.ENV{
			Cli: cl,
			Ns:  ns,
			Ctx: ctx,
		}
		EnsurePrometheus(&env, fqdn, skipOperator)
	},
}

func init() {
	PrometheusCmd.Flags().StringP("fqdn", "f", "sftests.com", "The FQDN for prometheus (prometheus.<FQDN>)")
	PrometheusCmd.Flags().BoolP("skip-operator-setup", "", false, "do not check if the prometheus operator is present and do not install it")
	// TODO we may want to deploy prometheus in a different namespace
}
