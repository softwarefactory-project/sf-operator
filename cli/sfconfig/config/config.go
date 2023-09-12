// Copyright © 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// This module defines the Config data type for the sfconfig.yaml
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type SFConfig struct {
	AnsibleMicroshiftRolePath string `mapstructure:"ansible_microshift_role_path"`
	Microshift                struct {
		Host string
		User string
	}
	FQDN     string
	Nodepool struct {
		CloudsFile string `mapstructure:"clouds_file"`
		KubeFile   string `mapstructure:"kube_file"`
	}
}

func GetSFConfigOrDie() SFConfig {
	var C SFConfig

	// Setting configuration defaults
	viper.SetDefault("ansible_microshift_role_path", "~/src/github.com/openstack-k8s-operators/ansible-microshift-role")
	viper.SetDefault("fqdn", "sftests.com")
	viper.SetDefault("microshift.host", "microshift.dev")
	viper.SetDefault("microshift.user", "cloud-user")
	viper.SetDefault("nodepool.clouds_file", "/etc/sf-operator/nodepool/clouds.yaml")
	viper.SetDefault("nodepool.kube_file", "/etc/sf-operator/nodepool/kubeconfig.yaml")

	err := viper.Unmarshal(&C)
	if err != nil {
		panic(fmt.Errorf("unable to decode into struct, %v", err))
	}

	return C
}
