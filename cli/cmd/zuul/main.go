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
	"errors"
	"fmt"
	"os"
	"strings"

	cliutils "github.com/softwarefactory-project/sf-operator/cli/cmd/utils"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	zuulGetAllowedArgs = []string{"client-config", "auth-token"}
)

func zuulCreate(kmd *cobra.Command, args []string) {
	cliCtx := cliutils.GetCLIctxOrDie(kmd, args, zuulGetAllowedArgs)
	target := args[0]
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	authConfig, _ := kmd.Flags().GetString("auth-config")
	tenant, _ := kmd.Flags().GetString("tenant")
	user, _ := kmd.Flags().GetString("user")
	expiry, _ := kmd.Flags().GetInt("expires-in")
	if target == "auth-token" {
		if tenant == "" {
			ctrl.Log.Error(errors.New("missing argument"), "A tenant is required")
			os.Exit(1)
		}
		token := CreateAuthToken(kubeContext, ns, authConfig, tenant, user, expiry)
		fmt.Println(token)
	}
	if target == "client-config" {
		insecure, _ := kmd.Flags().GetBool("insecure")
		fqdn := cliCtx.FQDN
		config := CreateClientConfig(kubeContext, ns, fqdn, authConfig, tenant, user, expiry, !insecure)
		fmt.Println(config)
	}
}

func MkZuulCmd() *cobra.Command {
	var (
		authConfig string
		tenant     string
		user       string
		expiry     int
		insecure   bool
		zuulCmd    = &cobra.Command{
			Use:   "zuul",
			Short: "Zuul subcommands",
			Long:  "These subcommands can be used to interact with the Zuul component of a Software Factory deployment",
		}
		createCmd, _, _ = cliutils.GetCRUDSubcommands()
	)

	createCmd.Run = zuulCreate
	createCmd.Use = "create {" + strings.Join(zuulGetAllowedArgs, ", ") + "}"
	createCmd.Long = "Create a Zuul resource: an authentication token or a CLI configuration file"
	createCmd.ValidArgs = zuulGetAllowedArgs
	createCmd.Flags().StringVar(&authConfig, "auth-config", "zuul_client", "the local authentication config to use to generate a token")
	createCmd.Flags().StringVar(&tenant, "tenant", "", "the tenant on which the token should grant admin access")
	createCmd.Flags().StringVar(&user, "user", "John Doe", "a username, only used for audit purposes in Zuul's access logs")
	createCmd.Flags().IntVar(&expiry, "expires-in", 3600, "how long the authentication token should be valid for")
	createCmd.Flags().BoolVar(&insecure, "insecure", false, "do not verify SSL certificates when connection to Zuul")

	zuulCmd.AddCommand(createCmd)
	return zuulCmd
}
