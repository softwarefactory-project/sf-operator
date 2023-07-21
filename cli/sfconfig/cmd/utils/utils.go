package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	apiroutev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"

	opv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
)

func RenderYAML(o interface{}) string {
	y, err := yaml.Marshal(o)
	if err != nil {
		panic(fmt.Errorf("err: %v\n", err))
	}
	return string(y)
}

func GetConfigContextOrDie(contextName string) *rest.Config {
	var conf *rest.Config
	var err error
	if conf, err = config.GetConfigWithContext(contextName); err != nil {
		panic(fmt.Errorf("Couldn't find context %s: %s", contextName, err))
	}
	return conf
}

func CreateKubernetesClient(contextName string) client.Client {
	scheme := runtime.NewScheme()
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
	client, err := client.New(conf, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}
	return client
}

// Function to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func Parse_string(text string, data any) (string, error) {

	template.New("StringtoParse").Parse(text)
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

func GetSF(name string) (sfv1.SoftwareFactory, error) {
	cli := CreateKubernetesClient("")
	var sf sfv1.SoftwareFactory
	err := cli.Get(context.Background(), client.ObjectKey{
		Namespace: "sf",
		Name:      name,
	}, &sf)
	return sf, err
}

func IsCRDMissing(err error) bool {
	// FIXME: replace stringly check with something more solid?
	return strings.Contains(err.Error(), `no matches for kind "SoftwareFactory"`)
}

func IsCertManagerRunning() bool {
	cli := CreateKubernetesClient("")
	var dep appsv1.Deployment
	cli.Get(context.Background(), client.ObjectKey{
		Namespace: "operators",
		Name:      "cert-manager-webhook",
	}, &dep)
	return dep.Status.ReadyReplicas >= 1
}
