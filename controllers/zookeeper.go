// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	_ "embed"
	"strings"
)

//go:embed templates/zookeeper.yaml
var zk_objs string

func (r *SFController) DeployZK(enabled bool) bool {
	if enabled {
		r.CreateYAMLs(strings.ReplaceAll(zk_objs, "{{ NS }}", r.ns))
		return r.IsStatefulSetReady("zookeeper")
	} else {
		r.DeleteStatefulSet("zookeeper")
		r.DeleteService("zookeeper")
		r.DeleteService("zookeeper-headless")
		return true
	}
}
