/*
Copyright © 2023 Red Hat
*/

// Package zuul exposes useful commands for Zuul deployers.
package zuul

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	v1 "k8s.io/api/core/v1"
)

var ZuulCmd = &cobra.Command{
	Use:   "zuul",
	Short: "Commands related to the administration of Zuul deployment with SF Operator",
	Long:  `The following commands simplify administrative tasks on a Zuul cluster deployed with SF Operator.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(0)
	},
}

var CreateAuthTokenCmd = &cobra.Command{
	Use:   "create-auth-token",
	Short: "Create a time-limited authentication token scoped to a tenant",
	Long: `This command is a proxy for the "zuul-admin create-auth-token" command run on a scheduler pod.

The command will output a JWT that can be passed to the zuul-client CLI or used with cURL to perform
administrative actions on a specified tenant.`,
	Run: func(cmd *cobra.Command, args []string) {
		tenant, _ := cmd.Flags().GetString("tenant")
		user, _ := cmd.Flags().GetString("user")
		expiry, _ := cmd.Flags().GetInt32("expires-in")
		namespace, _ := cmd.Flags().GetString("namespace")

		kubeConfig, kubeClientSet := utils.GetKubernetesClientSet()
		podslist, _ := kubeClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

		var zuulSchedulerContainer *v1.Pod = nil
		prefix := "zuul-scheduler"

		for _, container := range podslist.Items {
			if strings.HasPrefix(container.Name, prefix) {
				zuulSchedulerContainer = &container
				break
			}
		}

		if zuulSchedulerContainer == nil {
			fmt.Println("No Zuul scheduler pod running in the given namespace")
			os.Exit(1)
		}

		zuulAdminArgs := []string{
			// auth-config needs to refer to the one in controllers/static/zuul/zuul.conf
			"zuul-admin", "create-auth-token", "--auth-config", "zuul_client",
			"--tenant", tenant, "--user", user, "--expires-in", strconv.Itoa(int(expiry)),
		}

		buffer := &bytes.Buffer{}
		errorBuffer := &bytes.Buffer{}
		request := kubeClientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(zuulSchedulerContainer.Name).SubResource("exec").VersionedParams(&v1.PodExecOptions{
			Command: zuulAdminArgs,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

		exec, _ := remotecommand.NewSPDYExecutor(kubeConfig, "POST", request.URL())
		err := exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdout: buffer,
			Stderr: errorBuffer,
		})
		if err != nil {
			fmt.Println(errorBuffer)
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Print(buffer)
	},
}

func init() {
	CreateAuthTokenCmd.Flags().StringP("namespace", "n", "sf", "Name of the namespace where Zuul is deployed")
	CreateAuthTokenCmd.Flags().StringP("tenant", "t", "local", "The Zuul tenant on which to grant administrative powers")
	CreateAuthTokenCmd.Flags().StringP("user", "u", "cli_user", "A username for the token holder. Used for logs auditing only")
	CreateAuthTokenCmd.Flags().Int32P("expires-in", "x", 900, "The lifespan in seconds of the token. Defaults to 15 minutes (900s)")
	ZuulCmd.AddCommand(CreateAuthTokenCmd)
}
