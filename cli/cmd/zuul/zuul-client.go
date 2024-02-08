/*
Copyright © 2024 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zuul

import (
	"bytes"
	"os"
	"strings"
	"text/template"

	ctrl "sigs.k8s.io/controller-runtime"
)

type ZuulClientConfigSection struct {
	Name      string
	URL       string
	Tenant    string
	VerifySSL bool
	AuthToken string
}

var configTemplate = `
[{{ .Name }}]
url={{ .URL }}
{{ if .Tenant }}tenant={{ .Tenant }}{{ else }}{{ end }}
verify_ssl={{ if .VerifySSL }}True{{ else }}False{{ end }}
{{ if .AuthToken }}auth_token={{ .AuthToken }}{{ else }}{{ end }}

`

func CreateClientConfig(kubeContext string, namespace string, fqdn string, authConfig string, tenant string, user string, expiry int, verify bool) string {
	var tenants []string
	if tenant != "" {
		tenants = []string{tenant}
	} else {
		tenants = GetTenants(fqdn, verify)
	}
	var config string
	for _, t := range tenants {
		token := CreateAuthToken(kubeContext, namespace, authConfig, t, user, expiry)
		section := ZuulClientConfigSection{
			Name:      t,
			URL:       "https://" + fqdn + "/zuul",
			Tenant:    t,
			VerifySSL: verify,
			AuthToken: strings.TrimPrefix(token, "Bearer "),
		}
		confTemplate, err := template.New("configSection").Parse(configTemplate)
		if err != nil {
			ctrl.Log.Error(err, "Error initializing config template")
			os.Exit(1)
		}
		var buf bytes.Buffer
		if err := confTemplate.Execute(&buf, section); err != nil {
			ctrl.Log.Error(err, "Error applying config template")
			os.Exit(1)
		}
		config += buf.String()
	}
	return config
}
