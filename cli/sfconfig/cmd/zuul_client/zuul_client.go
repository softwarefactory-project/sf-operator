/*
Copyright © 2023 Red Hat
*/

// Package zuul_client exposes function for use zuul-client
package zuul_client

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func xORNameSpace(args []string) []string {
	remainingargs := []string{}
	isNextIdx := false
	for idx := range args {
		if (args[idx] == "-n" || args[idx] == "--namespace") ||
			isNextIdx {
			isNextIdx = !isNextIdx
			continue
		} else {
			remainingargs = append(remainingargs, args[idx])
		}
	}
	return remainingargs
}

// ZuulClientCmd operator ZuulClientCmd represents the zuul-client command
var ZuulClientCmd = &cobra.Command{
	Use:   "zuul-client",
	Short: "Run zuul-client command",
	Long: `Run zuul-client command on the corresponding pod

./tools/sfconfig zuul-client [flags] [OPTIONS]

flags:
 [-n|--namespace <name>] - executes comand in the corresponding namespace (default: "sf")

OPTIONS:
zuul-client [OPTIONS] - all zuul-client available options

Examples:
./sfconfig zuul-client - print sfconfig zuul-client subcommand options
./sfconfig zuul-client -h - prints zuul-client options
./sfconfig zuul-client [-n|--namespace <name>] [OPTIONS] executes comand in the corresponding namespace (default: "sf")

	`,
	Aliases: []string{"zc"},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}

		zuulClientArgs := xORNameSpace(args)
		cmd.DisableFlagParsing = false
		cmd.ParseFlags(args)
		cmd.DisableFlagParsing = true
		namespace, _ := cmd.Flags().GetString("namespace")

		kubeConfig, kubeClientSet := utils.GetKubernetesClientSet()

		podslist, _ := kubeClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})

		var zuulwebcontainer *v1.Pod = nil
		zuulwebprefix := "zuul-web"

		for _, container := range podslist.Items {
			if strings.HasPrefix(container.Name, zuulwebprefix) {
				zuulwebcontainer = &container
				break
			}
		}

		if zuulwebcontainer == nil {
			fmt.Println("Container with the prefix " + zuulwebprefix + " not found in namespace " + namespace)
			os.Exit(1)
		}

		zuulClientBaseArgs := []string{
			"zuul-client", "--use-config", "webclient",
		}

		zuulClientArgs = append(zuulClientBaseArgs, zuulClientArgs...)

		buf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		request := kubeClientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(zuulwebcontainer.Name).SubResource("exec").VersionedParams(&v1.PodExecOptions{
			Command: zuulClientArgs,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

		exec, _ := remotecommand.NewSPDYExecutor(kubeConfig, "POST", request.URL())
		err := exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdout: buf,
			Stderr: errBuf,
		})
		if err != nil {
			fmt.Println(errBuf)
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Print(buf)
	},
}

func init() {
	ZuulClientCmd.Flags().StringP("namespace", "n", "sf", "Name of the namespace")
	ZuulClientCmd.DisableFlagParsing = true
}
