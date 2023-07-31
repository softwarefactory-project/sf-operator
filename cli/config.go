// Copyright © 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// This module defines the Config data type for the sfconfig.yaml
package cli

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	AnsibleMicroshiftRolePath string `mapstructure:"ansible_microshift_role_path"`
	Microshift                struct {
		Host string
		User string
	}
	FQDN string `mapstructure:"sftests.com"`
}

func GetConfigOrDie() Config {
	var C Config

	err := viper.Unmarshal(&C)
	if err != nil {
		panic(fmt.Errorf("unable to decode into struct, %v", err))
	}
	if C.FQDN == "" {
		C.FQDN = "sftests.com"
	}
	return C
}
