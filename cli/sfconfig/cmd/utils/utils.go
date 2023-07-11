package utils

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	apiroutev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

func RenderYAML(o interface{}) string {
	y, err := yaml.Marshal(o)
	if err != nil {
		panic(fmt.Errorf("err: %v\n", err))
	}
	return string(y)
}

func CreateKubernetesClient() client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiroutev1.AddToScheme(scheme))
	client, err := client.New(config.GetConfigOrDie(), client.Options{
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
