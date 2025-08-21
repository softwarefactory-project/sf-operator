// Copyright (C) 2025 Red Hat
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"os/exec"
	"strings"
)

// version is a variable that will be set at build time
var version string

// GetVersion returns the latest tag known to git.
// If it doesn't work, try to get the current commit hash.
// If it doesn't work, return "development".
func GetVersion() string {
	if version != "" {
		return version
	}
	cmd := exec.Command("git", "describe", "--tags")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return "development"
}
