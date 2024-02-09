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

// Package zuul deals with zuul-related subcommands.
package zuul

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
)

type ZuulAPITenant struct {
	Name     string `json:"name"`
	Projects int    `json:"projects"`
	Queue    int    `json:"queue"`
}

func GetTenants(fqdn string, verify bool) []string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verify,
		},
	}
	client := &http.Client{Transport: tr}
	tenantsURL := "https://" + fqdn + "/zuul/api/tenants"
	resp, err := client.Get(tenantsURL)
	if err != nil {
		ctrl.Log.Error(err, "HTTP protocol error")
		os.Exit(1)
	}
	if resp.StatusCode >= 400 {
		ctrl.Log.Error(errors.New("bad status"), "API returned status "+resp.Status)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctrl.Log.Error(err, "Error reading API response")
		os.Exit(1)
	}
	_tenants := []ZuulAPITenant{}
	err = json.Unmarshal(body, &_tenants)
	if err != nil {
		ctrl.Log.Error(err, "Error marshalling JSON response")
		os.Exit(1)
	}
	tenants := []string{}
	for _, tenant := range _tenants {
		tenants = append(tenants, tenant.Name)
	}
	return tenants
}

func getFirstPod(prefix string, namespace string, kubeContext string) *v1.Pod {
	var ctr *v1.Pod = nil

	_, kubeClientset := cliutils.GetClientset(kubeContext)

	podslist, _ := kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	for _, container := range podslist.Items {
		if strings.HasPrefix(container.Name, prefix) {
			ctr = &container
			break
		}
	}
	return ctr
}
