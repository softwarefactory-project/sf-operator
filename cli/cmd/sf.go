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
"sf" subcommand relates to managing a Software Factory resource.
*/

import (
	"errors"
	"os"

	bootstraptenantconfigrepo "github.com/softwarefactory-project/sf-operator/cli/cmd/bootstrap-tenant-config-repo"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

func sfConfigureCmd(kmd *cobra.Command, args []string) {
	if args[0] == "TLS" {
		TLSConfigureCmd(kmd, args)
	} else {
		ctrl.Log.Error(errors.New("unknown argument"), args[0]+" is not a supported target")
		os.Exit(1)
	}
}

func MkSFCmd() *cobra.Command {

	var (
		CAPath          string
		CertificatePath string
		KeyPath         string

		sfCmd = &cobra.Command{
			Use:   "SF",
			Short: "subcommands related to managing a Software Factory resource",
			Long:  `Use these subcommands to perform management tasks at the resource level.`,
		}

		configureCmd = &cobra.Command{
			Use:       "configure {TLS}",
			Short:     "configure {TLS}",
			Long:      "Configure a SF resource. The resource can be the TLS certificates",
			ValidArgs: []string{"TLS"},
			Run:       sfConfigureCmd,
		}
	)
	configureCmd.Flags().StringVar(&CAPath, "CA", "", "path to the PEM-encoded Certificate Authority file")
	configureCmd.Flags().StringVar(&CertificatePath, "cert", "", "path to the domain certificate file")
	configureCmd.Flags().StringVar(&KeyPath, "key", "", "path to the private key file")

	sfCmd.AddCommand(MkBackupCmd())
	sfCmd.AddCommand(MkRestoreCmd())
	sfCmd.AddCommand(configureCmd)
	sfCmd.AddCommand(bootstraptenantconfigrepo.MkBootstrapCmd())

	return sfCmd
}
