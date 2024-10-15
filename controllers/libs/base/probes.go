// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package base provides various utility functions regarding base k8s resources used by the sf-operator
package base

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// --- readiness probes (validate a pod is ready to serve) ---
func mkReadinessProbe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:     handler,
		TimeoutSeconds:   5,
		PeriodSeconds:    10,
		FailureThreshold: 20,
	}
}

func MkReadinessCMDProbe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return mkReadinessProbe(handler)
}

func MkReadinessHTTPProbe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return mkReadinessProbe(handler)
}

func MkReadinessTCPProbe(port int) *apiv1.Probe {
	handler :=
		apiv1.ProbeHandler{
			TCPSocket: &apiv1.TCPSocketAction{
				Port: intstr.FromInt(port),
			}}
	return mkReadinessProbe(handler)
}

// --- liveness probes (validate a pod is up and running) ---
func mkLivenessProbe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:        handler,
		TimeoutSeconds:      5,
		PeriodSeconds:       20,
		InitialDelaySeconds: 5,
		FailureThreshold:    20,
	}
}

func MkLivenessCMDProbe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return mkLivenessProbe(handler)
}

func MkLiveHTTPProbe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return mkLivenessProbe(handler)
}

// --- startup probes (validate when pod has been started running) ---
func mkStartupProbe(handler apiv1.ProbeHandler) *apiv1.Probe {
	return &apiv1.Probe{
		ProbeHandler:        handler,
		TimeoutSeconds:      2,
		PeriodSeconds:       20,
		FailureThreshold:    10,
		InitialDelaySeconds: 5,
	}
}

func MkStartupCMDProbe(cmd []string) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		Exec: &apiv1.ExecAction{
			Command: cmd,
		}}
	return mkStartupProbe(handler)
}

func MkStartupHTTPProbe(path string, port int) *apiv1.Probe {
	handler := apiv1.ProbeHandler{
		HTTPGet: &apiv1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}}
	return mkStartupProbe(handler)
}

func SlowStartingProbe(probe *apiv1.Probe) {
	probe.PeriodSeconds = 60
	probe.FailureThreshold = 60
}
