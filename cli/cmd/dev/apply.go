/*
Copyright © 2023 Red Hat

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

package dev

/*
"applyStandalone" subcommand applies a SF CR manifest without the need for CRDs on the cluster.
*/

import (
	"os"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

func applyStandalone(ns string, sfResource string, kubeContext string) {
	if sfResource != "" && ns != "" {
		var sf sfv1.SoftwareFactory
		dat, err := os.ReadFile(sfResource)
		if err != nil {
			ctrl.Log.Error(err, "Error reading manifest:")
			os.Exit(1)
		}
		if err := yaml.Unmarshal(dat, &sf); err != nil {
			ctrl.Log.Error(err, "Error interpreting the SF custom resource:")
			os.Exit(1)
		}
		ctrl.Log.Info("Applying custom resource with the following parameters:",
			"CR", sf,
			"CR name", sf.ObjectMeta.Name,
			"Namespace", ns)
		controllers.Standalone(sf, ns, kubeContext)
		os.Exit(0)
	}
}
