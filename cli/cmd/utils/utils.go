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

// Package utils provides CLI utility functions and structs
package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	opv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
)

// CLI config struct
type SoftwareFactoryConfigContext struct {
	ConfigRepository string `json:"config-repository-path" mapstructure:"config-repository-path"`
	Manifest         string `json:"manifest-file" mapstructure:"manifest-file"`
	IsStandalone     bool   `json:"standalone" mapstructure:"standalone"`
	Namespace        string `json:"namespace" mapstructure:"namespace"`
	KubeContext      string `json:"kube-context" mapstructure:"kube-context"`
	FQDN             string `json:"fqdn" mapstructure:"fqdn"`
	Dev              struct {
		AnsibleMicroshiftRolePath string `json:"ansible-microshift-role-path" mapstructure:"ansible-microshift-role-path"`
		SFOperatorRepositoryPath  string `json:"sf-operator-repository-path" mapstructure:"sf-operator-repository-path"`
		Microshift                struct {
			Host                string `json:"host" mapstructure:"host"`
			User                string `json:"user" mapstructure:"user"`
			OpenshiftPullSecret string `json:"openshift-pull-secret" mapstructure:"openshift-pull-secret"`
			DiskFileSize        string `json:"disk-file-size" mapstructure:"disk-file-size"`
		} `json:"microshift" mapstructure:"microshift"`
		Tests struct {
			ExtraVars map[string]string `json:"extra-vars" mapstructure:"extra-vars"`
		} `json:"tests" mapstructure:"tests"`
	} `json:"development" mapstructure:"development"`
	Components struct {
		Nodepool struct {
			CloudsFile string `json:"clouds-file" mapstructure:"clouds-file"`
			KubeFile   string `json:"kube-file" mapstructure:"kube-file"`
		} `json:"nodepool" mapstructure:"nodepool"`
	} `json:"components" mapstructure:"components"`
}

type SoftwareFactoryConfig struct {
	Contexts map[string]SoftwareFactoryConfigContext `json:"contexts" mapstructure:"contexts"`
	Default  string                                  `json:"default-context" mapstructure:"default-context"`
}

func loadConfigFile(command *cobra.Command) (cliConfig SoftwareFactoryConfig, err error) {
	configPath, _ := command.Flags().GetString("config")
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&cliConfig)
	return
}

func getContextFromFile(command *cobra.Command) (ctxName string, cliContext SoftwareFactoryConfigContext, err error) {
	cliConfig, err := loadConfigFile(command)
	if err != nil {
		return
	}
	ctx, _ := command.Flags().GetString("context")
	if ctx == "" {
		ctx = cliConfig.Default
	}
	for c := range cliConfig.Contexts {
		if ctx == "" || ctx == c {
			return c, cliConfig.Contexts[c], nil
		}
	}
	return ctxName, cliContext, errors.New("context not found")
}

func GetCLIContext(command *cobra.Command) (SoftwareFactoryConfigContext, error) {
	var cliContext SoftwareFactoryConfigContext
	var ctxName string
	var err error
	configPath, _ := command.Flags().GetString("config")
	if configPath != "" {
		ctxName, cliContext, err = getContextFromFile(command)
		if err != nil {
			ctrl.Log.Error(err, "Could not load config file")
		} else {
			ctrl.Log.Info("Using configuration context " + ctxName)
		}
	}
	// Override with defaults
	// We don't set a default namespace here so as not to interfere with rootcommand.
	ns, _ := command.Flags().GetString("namespace")
	if cliContext.Namespace == "" {
		cliContext.Namespace = ns
	}
	kubeContext, _ := command.Flags().GetString("kube-context")
	if cliContext.KubeContext == "" {
		cliContext.KubeContext = kubeContext
	}
	fqdn, _ := command.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}
	if cliContext.FQDN == "" {
		cliContext.FQDN = fqdn
	}
	return cliContext, nil
}

func GetCRUDSubcommands() (*cobra.Command, *cobra.Command, *cobra.Command) {
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
	}
	configureCmd := &cobra.Command{
		Use:   "configure",
		Short: "Configure a resource",
	}
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource",
	}
	return createCmd, configureCmd, getCmd
}

// Moving code from cli/sfconfig/cmd/utils/utils.go as we need it to avoid dead code
type ENV struct {
	Cli client.Client
	Ns  string
	Ctx context.Context
}

func CreateKubernetesClient(contextName string) (client.Client, error) {
	scheme := runtime.NewScheme()
	monitoring.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiroutev1.AddToScheme(scheme))
	utilruntime.Must(opv1.AddToScheme(scheme))
	utilruntime.Must(sfv1.AddToScheme(scheme))
	var conf = controllers.GetConfigContextOrDie(contextName)
	return client.New(conf, client.Options{
		Scheme: scheme,
	})
}

func CreateKubernetesClientOrDie(contextName string) client.Client {
	cli, err := CreateKubernetesClient(contextName)
	if err != nil {
		ctrl.Log.Error(err, "Error creating Kubernetes client")
		os.Exit(1)
	}
	return cli
}

func GetM(env *ENV, name string, obj client.Object) (bool, error) {
	err := env.Cli.Get(env.Ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: env.Ns,
		}, obj)
	if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func DeleteOrDie(env *ENV, obj client.Object, opts ...client.DeleteOption) bool {
	err := env.Cli.Delete(env.Ctx, obj, opts...)
	if apierrors.IsNotFound(err) {
		return false
	} else if err != nil {
		msg := fmt.Sprintf("Error while deleting %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		ctrl.Log.Error(err, msg)
		os.Exit(1)
	}
	return true
}

func GetMOrDie(env *ENV, name string, obj client.Object) bool {
	_, err := GetM(env, name, obj)
	if apierrors.IsNotFound(err) {
		return false
	} else if err != nil {
		msg := fmt.Sprintf("Error while fetching %s \"%s\"", reflect.TypeOf(obj).Name(), name)
		ctrl.Log.Error(err, msg)
		os.Exit(1)
	}
	return true
}

func UpdateROrDie(env *ENV, obj client.Object) {
	var msg = fmt.Sprintf("Updating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), env.Ns)
	ctrl.Log.Info(msg)
	if err := env.Cli.Update(env.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while updating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		ctrl.Log.Error(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" updated", reflect.TypeOf(obj).Name(), obj.GetName())
	ctrl.Log.Info(msg)
}

func CreateROrDie(env *ENV, obj client.Object) {
	var msg = fmt.Sprintf("Creating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), env.Ns)
	ctrl.Log.Info(msg)
	obj.SetNamespace(env.Ns)
	if err := env.Cli.Create(env.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while creating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		ctrl.Log.Error(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" created", reflect.TypeOf(obj).Name(), obj.GetName())
	ctrl.Log.Info(msg)
}

func DeleteAllOfOrDie(env *ENV, obj client.Object, opts ...client.DeleteAllOfOption) {
	if err := env.Cli.DeleteAllOf(env.Ctx, obj, opts...); err != nil {
		var msg = "Error while deleting"
		ctrl.Log.Error(err, msg)
		os.Exit(1)
	}
}

func GetCLIctxOrDie(kmd *cobra.Command, args []string, allowedArgs []string) SoftwareFactoryConfigContext {
	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing:")
		os.Exit(1)
	}
	if len(allowedArgs) == 0 {
		// no more validation needed
		return cliCtx
	} else {
		argumentError := errors.New("argument must be in: " + strings.Join(allowedArgs, ", "))
		if len(args) != 1 {
			ctrl.Log.Error(argumentError, "Need one argument")
			os.Exit(1)
		}
		for _, a := range allowedArgs {
			if args[0] == a {
				return cliCtx
			}
		}
		ctrl.Log.Error(argumentError, "Unknown argument "+args[0])
		os.Exit(1)
	}
	return SoftwareFactoryConfigContext{}
}

func GetFileContent(filePath string) ([]byte, error) {
	if filePath == "" {
		return []byte{}, nil
	}
	if _, err := os.Stat(filePath); err == nil {
		if data, err := os.ReadFile(filePath); err == nil {
			return data, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func RunCmdWithEnvOrDie(environ []string, cmd string, args ...string) string {
	kmd := exec.Command(cmd, args...)
	kmd.Env = append(os.Environ(), environ...)
	out, err := kmd.CombinedOutput()
	if err != nil {
		ctrl.Log.Error(err, "Could not run command '"+cmd+"'")
		ctrl.Log.Info("Captured output:\n" + string(out))
		os.Exit(1)
	}
	return string(out)
}

func RunCmdOrDie(cmd string, args ...string) string {
	return RunCmdWithEnvOrDie([]string{}, cmd, args...)
}
