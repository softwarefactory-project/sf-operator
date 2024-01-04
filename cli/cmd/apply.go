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

package cmd

/*
"apply" subcommand applies a SF CR manifest without the need for CRDs on the cluster.
*/

import (
	"errors"
	"os"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

func applyCmd(kmd *cobra.Command, args []string) {
	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
	sfResource, _ := kmd.Flags().GetString("cr")
	hasManifest := &cliCtx.Manifest
	if sfResource == "" && hasManifest != nil {
		sfResource = cliCtx.Manifest
	}
	if (sfResource != "" && ns == "") || (sfResource == "" && ns != "") {
		err := errors.New("standalone mode requires both --cr and --namespace to be set")
		ctrl.Log.Error(err, "Argument error:")
		os.Exit(1)
	} else if sfResource != "" && ns != "" {
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
		controllers.Standalone(sf, ns, cliCtx.KubeContext)
		os.Exit(0)
	}
}

func MkApplyCmd() *cobra.Command {

	var (
		sfResource string

		applyCmd = &cobra.Command{
			Use:   "apply",
			Short: "Apply a SoftwareFactory Custom Resource to a cluster",
			Long: `This command can be used to deploy a SoftwareFactory resource without installing the operator or its associated CRDs on a cluster.
				This will run the operator runtime locally, deploy the resource's components on the cluster, then exit.`,
			Run: applyCmd,
		}
	)

	applyCmd.Flags().StringVar(&sfResource, "cr", "", "The path to the CR to apply.")
	return applyCmd
}
