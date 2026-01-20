// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	"bytes"

	. "github.com/onsi/gomega"
)

// Ensure the zuul.conf matches the secret, and return its value
func zuulConfMatchSecret() string {
	// zuul.conf from secret
	secret := readSecretValue("zuul-config", "zuul.conf")

	// zuul.conf from pod
	var buf bytes.Buffer
	sfctx.PodExecOut("zuul-scheduler-0", "zuul-scheduler", []string{"cat", "/etc/zuul/zuul.conf"}, &buf)
	config := buf.String()

	// validate it matches
	Ω(config).Should(Equal(secret))

	return config
}
