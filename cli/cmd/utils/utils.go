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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"

	apiroutev1 "github.com/openshift/api/route/v1"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"
)

// SetLogger enables the DEBUG LogLevel in the Logger when the debug flag is set
func SetLogger(command *cobra.Command) {
	debug, _ := command.Flags().GetBool("debug")
	logLevel := zapcore.InfoLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}

	opts := zap.Options{
		Development: true,
		Level:       logLevel,
		DestWriter:  os.Stderr,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
}

func GetCLIContext(command *cobra.Command) *controllers.SFKubeContext {
	// This is usually called for every CLI command so here let's set the Logger settings
	SetLogger(command)

	namespace, _ := command.Flags().GetString("namespace")
	kubeContext, _ := command.Flags().GetString("kube-context")

	// TODO: handle dry-run here!
	ctx, err := controllers.MkSFKubeContext("", namespace, kubeContext, false)
	if err != nil {
		logging.LogE(err, "Error creating Kubernetes client")
		os.Exit(1)
	}
	return &ctx
}

// GetCLICRContext setup the SFKubeContext and read the CR from the args.
func GetCLICRContext(command *cobra.Command, args []string) (*controllers.SFKubeContext, sfv1.SoftwareFactory) {
	SetLogger(command)

	if len(args) == 0 {
		ctrl.Log.Error(errors.New("no custom resource provided"), "You need to pass the CR!")
		os.Exit(1)
	}
	crPath := args[0]

	var sf sfv1.SoftwareFactory
	sf, err := controllers.ReadSFYAML(crPath)
	if err != nil {
		ctrl.Log.Error(err, "Could not read resource")
		os.Exit(1)
	}

	kubeConfig := filepath.Dir(crPath) + "/kubeconfig"
	if _, err := os.Stat(kubeConfig); err == nil {
		ctrl.Log.Info("Using default kubeconfig", "path", kubeConfig)
	} else {
		kubeConfig = ""
	}

	namespace, _ := command.Flags().GetString("namespace")
	kubeContext, _ := command.Flags().GetString("kube-context")

	// TODO: handle dry-run here!
	env, err := controllers.MkSFKubeContext(kubeConfig, namespace, kubeContext, false)
	if err != nil {
		logging.LogE(err, "Error creating Kubernetes client")
		os.Exit(1)
	}

	// Discover the cr owner.
	env.GetStandaloneOwner()

	return &env, sf
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
		logging.LogE(err, "Could not run command '"+cmd+"'")
		logging.LogI("Captured output:\n" + string(out))
		os.Exit(1)
	}
	return string(out)
}

func RunCmdOrDie(cmd string, args ...string) string {
	return RunCmdWithEnvOrDie([]string{}, cmd, args...)
}

func WriteContentToFile(filePath string, content []byte, mode fs.FileMode) {
	err := os.WriteFile(filePath, content, mode)
	if err != nil {
		logging.LogE(err, "Can not write a file "+filePath)
		os.Exit(1)
	}
}

func VarListToMap(varsList []string) map[string]string {

	var vars = make(map[string]string)

	for _, v := range varsList {
		tokens := strings.Split(v, "=")

		if len(tokens) != 2 {
			logging.LogE(errors.New("parse error"), "parsed value `"+v+"` needs to be defined as 'foo=bar'")
			os.Exit(1)
		}
		vars[tokens[0]] = tokens[1]
	}
	return vars
}

func CreateDirectory(dirPath string, mode fs.FileMode) {
	err := os.MkdirAll(dirPath, mode)
	if err != nil {
		logging.LogE(err, "Can not create directory "+dirPath)
		os.Exit(1)
	}
}

func ConvertMapOfBytesToMapOfStrings(contentMap map[string][]byte) map[string]string {
	strMap := map[string]string{}
	for key, value := range contentMap {
		strValue := string(value)
		strMap[key] = strValue
	}
	return strMap
}

func GetKubeConfig() *clientcmdapi.Config {
	clientCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		logging.LogE(err, "Could not find the kubeconfig")
		os.Exit(1)
	}
	return clientCfg
}

func GetKubeConfigContextByName(contextName string) (*clientcmdapi.Context, string) {
	clientCfg := GetKubeConfig()

	// The user did not specify a context, let's pick the current one from the kubeconfig
	if contextName == "" {
		if clientCfg.CurrentContext == "" {
			// Use the first available context
			for k := range clientCfg.Contexts {
				contextName = k
				break
			}
		} else {
			contextName = clientCfg.CurrentContext
		}
	}
	// Load the context
	context, err := clientCfg.Contexts[contextName]
	if !err {
		logging.LogD("could not find the context " + contextName)
	}
	return context, contextName
}

// MkHTTPSRoute produces a Route on top of a Service
func MkHTTPSRoute(
	name string, ns string, host string, serviceName string, path string, port int, extraLabels map[string]string) apiroutev1.Route {
	tls := apiroutev1.TLSConfig{
		InsecureEdgeTerminationPolicy: apiroutev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   apiroutev1.TLSTerminationEdge,
	}
	return apiroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
		},
		Spec: apiroutev1.RouteSpec{
			TLS:  &tls,
			Host: host,
			To: apiroutev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: ptr.To[int32](100),
			},
			Port: &apiroutev1.RoutePort{
				TargetPort: intstr.FromInt(port),
			},
			Path:           path,
			WildcardPolicy: "None",
		},
	}
}

func Ptr[T any](v T) *T {
	return &v
}

func MkHTTPSIngress(ns string, name string, host string, service string, port int32, extraLabels map[string]string) networkv1.Ingress {
	rule := networkv1.IngressRuleValue{
		HTTP: &networkv1.HTTPIngressRuleValue{
			Paths: []networkv1.HTTPIngressPath{
				{
					Path:     "/",
					PathType: Ptr(networkv1.PathTypePrefix),
					Backend: networkv1.IngressBackend{
						Service: &networkv1.IngressServiceBackend{
							Name: service,
							Port: networkv1.ServiceBackendPort{
								Number: port,
							},
						},
					},
				},
			},
		},
	}
	return networkv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    extraLabels,
		},
		Spec: networkv1.IngressSpec{
			IngressClassName: ptr.To[string]("nginx"),
			TLS: []networkv1.IngressTLS{
				networkv1.IngressTLS{
					Hosts:      []string{host},
					SecretName: "self-signed-cert",
				},
			},
			Rules: []networkv1.IngressRule{
				networkv1.IngressRule{
					Host:             host,
					IngressRuleValue: rule,
				},
			},
		},
	}
}

func EnsureSelfSignCert(env *controllers.SFKubeContext) {
	name := "self-signed-cert"
	var secret apiv1.Secret
	if !env.GetM(name, &secret) {
		// Generate key
		var err error
		var priv *rsa.PrivateKey
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			ctrl.Log.Error(err, "Failed to generate private key")
			os.Exit(1)
		}

		// Generate cert
		template := x509.Certificate{
			SerialNumber: new(big.Int).Lsh(big.NewInt(1), 128),
			Subject: pkix.Name{
				Organization: []string{"Acme Co"},
			},
			DNSNames:              []string{"gerrit.sfop.me", "sfop.me"},
			BasicConstraintsValid: true,
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(100, 0, 0),
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		if err != nil {
			ctrl.Log.Error(err, "Unable to create public key")
			os.Exit(1)
		}

		// Generate priv key
		privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			ctrl.Log.Error(err, "Unable to marshal private key")
			os.Exit(1)
		}

		// Encode the certificate in a Secret
		data := make(map[string][]byte)
		data["tls.crt"] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		data["tls.key"] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
		secret = apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: env.Ns},
			Data:       data,
			Type:       "kubernetes.io/tls",
		}
		env.CreateROrDie(&secret)
	}
}

func ReadIngressIP(env *controllers.SFKubeContext, name string) string {
	attempt := 1
	maxTries := 25
	for {
		var ingress networkv1.Ingress
		if env.GetM(name, &ingress) {
			lb := ingress.Status.LoadBalancer.Ingress
			if len(lb) > 0 {
				return lb[0].IP
			}
		}
		if attempt > maxTries {
			ctrl.Log.Error(nil, fmt.Sprintf("Couldn't find the %s IP", name))
			os.Exit(1)
		}
		ctrl.Log.Info(fmt.Sprintf("Waiting for %s ... [attempt %d/%d]", name, attempt, maxTries))
		attempt += 1
		time.Sleep(7 * time.Second)
	}
}

func EnsureGatewayIngress(env *controllers.SFKubeContext, fqdn string) {
	EnsureSelfSignCert(env)
	ingress := MkHTTPSIngress(env.Ns, "sf-ingress", fqdn, "gateway", 8080, map[string]string{})
	if !env.GetM(ingress.Name, &ingress) {
		env.CreateROrDie(&ingress)
	}
}

func WriteIngressToEtcHosts(env *controllers.SFKubeContext, fqdn string) {
	// Grab the ingress ip
	var gerritIP = ReadIngressIP(env, "gerrit-ingress")
	var gatewayIP = ReadIngressIP(env, "sf-ingress")

	// Remove the previous dns from /etc/hosts
	hosts, err := GetFileContent("/etc/hosts")
	if err != nil {
		ctrl.Log.Error(nil, "Couldn't read /etc/hosts")
		os.Exit(1)
	}
	lines := strings.Split(string(hosts), "\n")
	newLines := make([]string, 0)
	for _, l := range lines {
		if !strings.Contains(l, fqdn) {
			newLines = append(newLines, l)
		}
	}

	// Add the new dns to /etc/hosts
	if gerritIP == gatewayIP {
		newLines = append(newLines, gatewayIP+" gerrit."+fqdn+" "+fqdn)
	} else {
		newLines = append(newLines, gerritIP+" gerrit."+fqdn)
		newLines = append(newLines, gatewayIP+" "+fqdn)
	}
	ctrl.Log.Info("Updating /etc/hosts")
	WriteContentToFile("/etc/hosts", []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}
