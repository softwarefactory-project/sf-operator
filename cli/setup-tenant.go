// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"

	"github.com/softwarefactory-project/sf-operator/controllers"
)

var tenantTemplate = `
- tenant:
    name: {{ .name }}
    source:
      gerrit:
        config-projects:
          - config
        untrusted-projects:
          - demo-project
`

func SetupTenant(configPath string, tenantName string) {
	tenantDir := filepath.Join(configPath, "zuul")
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		panic(err)
	}
	tenantFile := filepath.Join(tenantDir, "main.yaml")
	tenantData, err := controllers.Parse_string(tenantTemplate, map[string]string{"name": tenantName})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(tenantFile, []byte(tenantData), 0644); err != nil {
		panic(err)
	}
	runCmd("git", "-C", configPath, "add", "zuul/main.yaml")
}
