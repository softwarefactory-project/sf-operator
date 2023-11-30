// Package utils provides utility functions for the CLI
package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	apiroutev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	opv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

type ENV struct {
	Cli client.Client
	Ns  string
	Ctx context.Context
}

// RunMake is a temporary hack until make target are implemented natively
func RunMake(arg string) {
	RunCmd("make", arg)
}

func RunCmd(cmdName string, args ...string) {
	if err := RunCmdNoPanic(cmdName, args...); err != nil {
		panic(fmt.Errorf("%s failed: %w", args, err))
	}
}

func RunCmdNoPanic(cmdName string, args ...string) error {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func EnsureNamespace(env *ENV, name string) {
	var ns apiv1.Namespace
	if err := env.Cli.Get(env.Ctx, client.ObjectKey{Name: name}, &ns); errors.IsNotFound(err) {
		ns.Name = name
		CreateR(env, &ns)
	} else if err != nil {
		panic(fmt.Errorf("could not get namespace: %s", err))
	}
}

func EnsureServiceAccount(env *ENV, name string) {
	var sa apiv1.ServiceAccount
	if !GetM(env, name, &sa) {
		sa.Name = name
		CreateR(env, &sa)
	}
}

func RenderYAML(o interface{}) string {
	y, err := yaml.Marshal(o)
	if err != nil {
		panic(fmt.Errorf("err: %v", err))
	}
	return string(y)
}

func GetConfigContextOrDie(contextName string) *rest.Config {
	var conf *rest.Config
	var err error
	if conf, err = config.GetConfigWithContext(contextName); err != nil {
		panic(fmt.Errorf("couldn't find context %s: %s", contextName, err))
	}
	return conf
}

func CreateKubernetesClient(contextName string) (client.Client, error) {
	scheme := runtime.NewScheme()
	monitoring.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiroutev1.AddToScheme(scheme))
	utilruntime.Must(opv1.AddToScheme(scheme))
	utilruntime.Must(sfv1.AddToScheme(scheme))
	var conf *rest.Config
	if contextName != "" {
		conf = GetConfigContextOrDie(contextName)
	} else {
		conf = config.GetConfigOrDie()
	}
	return client.New(conf, client.Options{
		Scheme: scheme,
	})
}

func CreateKubernetesClientOrDie(contextName string) client.Client {
	cli, err := CreateKubernetesClient(contextName)
	if err != nil {
		fmt.Println("failed to create client", err)
		os.Exit(1)
	}
	return cli
}

// ParseString allows to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func ParseString(text string, data any) (string, error) {

	// Opening Template file
	template, err := template.New("StringtoParse").Parse(text)
	if err != nil {
		return "", fmt.Errorf("Text not in the right format: " + text)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = template.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", text)
	}

	return buf.String(), nil
}

// GetM is an helper to fetch a kubernetes resource by name, returns true when it is found.
func GetM(env *ENV, name string, obj client.Object) bool {
	err := env.Cli.Get(env.Ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: env.Ns,
		}, obj)
	if errors.IsNotFound(err) {
		return false
	} else if err != nil {
		panic(fmt.Errorf("could not get %s: %s", name, err))
	}
	return true
}

// CreateR is an helper to create a kubernetes resource.
func CreateR(env *ENV, obj client.Object) {
	fmt.Fprintf(os.Stderr, "Creating %s in %s\n", obj.GetName(), env.Ns)
	obj.SetNamespace(env.Ns)
	if err := env.Cli.Create(env.Ctx, obj); err != nil {
		panic(fmt.Errorf("could not create %s: %s", obj, err))
	}
}

// UpdateR is an helper to update a kubernetes resource.
func UpdateR(env *ENV, obj client.Object) bool {
	fmt.Fprintf(os.Stderr, "Updating %s in %s\n", obj.GetName(), env.Ns)
	if err := env.Cli.Update(env.Ctx, obj); err != nil {
		panic(fmt.Errorf("could not update %s: %s", obj, err))
	}
	return true
}

func CreateTempPlaybookFile(content string) (*os.File, error) {
	file, e := os.CreateTemp("playbooks", "sfconfig-operator-create-")
	if e != nil {
		panic(e)
	}
	fmt.Println("Temp file name:", file.Name())
	_, e = file.Write([]byte(content))
	if e != nil {
		panic(e)
	}
	e = file.Close()
	return file, e
}

func RemoveTempPlaybookFile(file *os.File) {
	defer os.Remove(file.Name())
}

func GetSF(env *ENV, name string) (sfv1.SoftwareFactory, error) {
	var sf sfv1.SoftwareFactory
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: env.Ns,
		Name:      name,
	}, &sf)
	return sf, err
}

func IsCRDMissing(err error) bool {
	// FIXME: replace stringly check with something more solid?
	return strings.Contains(err.Error(), `no matches for kind "SoftwareFactory"`) ||
		// This case is encountered when make install has not been run prior
		strings.Contains(err.Error(), `sf.softwarefactory-project.io/v1: the server could not find the requested resource`)
}

func IsCertManagerRunning(env *ENV) bool {
	var dep appsv1.Deployment
	env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: "operators",
		Name:      "cert-manager-webhook",
	}, &dep)
	return dep.Status.ReadyReplicas >= 1
}

func GetSecret(env *ENV, name string) []byte {
	var secret apiv1.Secret
	err := env.Cli.Get(env.Ctx, client.ObjectKey{
		Namespace: env.Ns,
		Name:      name,
	}, &secret)
	if err != nil {
		panic(err)
	}
	return secret.Data[name]
}

func GetFileContent(filePath string) ([]byte, error) {
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

func GetKubernetesClientSet() (*rest.Config, *kubernetes.Clientset) {

	kubeConfig := config.GetConfigOrDie()

	// create the kubernetes Clientset
	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err.Error())
	}
	return kubeConfig, kubeClientset
}
