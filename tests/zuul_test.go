// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	. "github.com/onsi/gomega"
)

// Ensure the zuul.conf matches the secret, and return its value
func zuulConfMatchSecret() string {
	// zuul.conf from secret
	secret := readSecretValue("zuul-config", "zuul.conf")

	// zuul.conf from pod
	config := readZuulCommand("cat /etc/zuul/zuul.conf")

	// validate it matches
	Ω(config).Should(Equal(secret))

	return config
}

func readZuulCommand(command string) string {
	return readCommand("zuul-scheduler-0", "zuul-scheduler", command)
}

func readZuulCommandArgs(args []string) string {
	return readCommandArgs("zuul-scheduler-0", "zuul-scheduler", args)
}
