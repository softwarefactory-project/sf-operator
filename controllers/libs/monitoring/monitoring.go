// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package monitoring provides various utility functions regarding monitoring for the sf-operator
package monitoring

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceMonitorLabelSelector - TODO this could be a spec parameter.
const ServiceMonitorLabelSelector = "sf-monitoring"

func MkPrometheusRuleGroup(name string, rules []monitoringv1.Rule) monitoringv1.RuleGroup {
	// d := monitoringv1.Duration(duration)
	return monitoringv1.RuleGroup{
		Name: name,
		// Interval: &d,
		Rules: rules,
	}
}

func MkPrometheusAlertRule(name string, expr intstr.IntOrString, forDuration string, labels map[string]string, annotations map[string]string) monitoringv1.Rule {
	f := monitoringv1.Duration(forDuration)
	return monitoringv1.Rule{
		Alert:       name,
		Expr:        expr,
		For:         &f,
		Labels:      labels,
		Annotations: annotations,
	}
}

//lint:ignore U1000 this function will be used in a followup change
func mkServiceMonitor(name string, ns string, port string, selector metav1.LabelSelector) monitoringv1.ServiceMonitor {
	return monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
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

func MkPodMonitor(name string, ns string, port string, selector metav1.LabelSelector) monitoringv1.PodMonitor {
	return monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
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

func MkPrometheusRuleCR(name string, ns string) monitoringv1.PrometheusRule {
	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				ServiceMonitorLabelSelector: name,
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}
}
