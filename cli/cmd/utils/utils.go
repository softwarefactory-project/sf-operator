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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	apiroutev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	opv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
	"github.com/softwarefactory-project/sf-operator/controllers/libs/logging"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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
			ETCDOnRAMDisk       bool   `json:"etcd-on-ramdisk" mapstructure:"etcd-on-ramdisk"`
			RAMDiskSize         string `json:"ramdisk-size" mapstructure:"ramdisk-size"`
		} `json:"microshift" mapstructure:"microshift"`
		Tests struct {
			DemoReposPath string            `json:"demo-repos-path" mapstructure:"demo-repos-path"`
			ExtraVars     map[string]string `json:"extra-vars" mapstructure:"extra-vars"`
		} `json:"tests" mapstructure:"tests"`
	} `json:"development" mapstructure:"development"`
	Components struct {
		Nodepool struct {
			CloudsFile string `json:"clouds-file" mapstructure:"clouds-file"`
			KubeFile   string `json:"kube-file" mapstructure:"kube-file"`
		} `json:"nodepool" mapstructure:"nodepool"`
	} `json:"components" mapstructure:"components"`
	HostAliases []sfv1.HostAlias `json:"hostaliases,omitempty" mapstructure:"hostaliases"`
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

func GetCLIContext(command *cobra.Command) (SoftwareFactoryConfigContext, error) {

	// This is usually called for every CLI command so here let's set the Logger settings
	SetLogger(command)

	var cliContext SoftwareFactoryConfigContext
	var ctxName string
	var err error
	configPath, _ := command.Flags().GetString("config")
	if configPath != "" {
		ctxName, cliContext, err = getContextFromFile(command)
		if err != nil {
			logging.LogE(err, "Could not load config file")
			os.Exit(1)
		} else {
			logging.LogD("Using configuration context " + ctxName)
		}
	}
	// Override with defaults
	// We don't set a default namespace here so as not to interfere with rootcommand.

	ns, _ := command.Flags().GetString("namespace")

	kubeContext, _ := command.Flags().GetString("kube-context")
	currentContext, contextName := GetKubeConfigContextByName(kubeContext)

	defaultFunc := func(userProvided string, defaultValue string) string {
		if userProvided != "" {
			return userProvided
		}
		return defaultValue
	}

	// Default ladder: 1st what is in sf-operator cli config file passed as --config argument
	//                 2st what is in sf-operator cli passed as --namespace argumente
	//                 3rd what is defind default kubeconfig
	if cliContext.Namespace == "" {
		// The user did not provide a --namespace argument, let's find it in the context
		currentContextNamespace := ""
		if currentContext != nil {
			currentContextNamespace = currentContext.Namespace
		}
		cliContext.Namespace = defaultFunc(ns, currentContextNamespace)
	}

	// Default ladder: 1st what is in sf-operator cli config file passed as --config argument
	//                 2st what is in sf-operator cli passed as --kube-context argumente
	//                 3rd what is defind default kubeconfig
	if cliContext.KubeContext == "" {
		cliContext.KubeContext = defaultFunc(kubeContext, contextName)
	}

	fqdn, _ := command.Flags().GetString("fqdn")
	if fqdn == "" {
		fqdn = "sfop.me"
	}
	if cliContext.FQDN == "" {
		cliContext.FQDN = fqdn
	}
	if cliContext.Dev.SFOperatorRepositoryPath == "" {
		defaultSFOperatorRepositoryPath, getwdErr := os.Getwd()
		if getwdErr != nil {
			logging.LogE(getwdErr,
				"sf-operator-repository-path is not set in `dev` section of the configuration file and unable to determine the current working directory")
			os.Exit(1)
		}
		cliContext.Dev.SFOperatorRepositoryPath = defaultSFOperatorRepositoryPath
		logging.LogD("Using current working directory for sf-operator-repository-path: " + cliContext.Dev.SFOperatorRepositoryPath)
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
	Cli         client.Client
	Ns          string
	Ctx         context.Context
	IsOpenShift bool
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
		logging.LogE(err, "Error creating Kubernetes client")
		os.Exit(1)
	}
	return cli
}

func GetCLIENV(kmd *cobra.Command) (string, ENV) {

	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		logging.LogE(err, "Error initializing CLI:")
		os.Exit(1)
	}

	kubeContext := cliCtx.KubeContext

	env := ENV{
		Cli: CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  cliCtx.Namespace,
	}

	return kubeContext, env
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
		logging.LogE(err, msg)
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
		logging.LogE(err, msg)
		os.Exit(1)
	}
	return true
}

func UpdateROrDie(env *ENV, obj client.Object) {
	var msg = fmt.Sprintf("Updating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), env.Ns)
	logging.LogI(msg)
	if err := env.Cli.Update(env.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while updating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		logging.LogE(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" updated", reflect.TypeOf(obj).Name(), obj.GetName())
	logging.LogI(msg)
}

func CreateROrDie(env *ENV, obj client.Object) {
	var msg = fmt.Sprintf("Creating %s \"%s\" in %s", reflect.TypeOf(obj).Name(), obj.GetName(), env.Ns)
	logging.LogI(msg)
	obj.SetNamespace(env.Ns)
	if err := env.Cli.Create(env.Ctx, obj); err != nil {
		msg = fmt.Sprintf("Error while creating %s \"%s\"", reflect.TypeOf(obj).Name(), obj.GetName())
		logging.LogE(err, msg)
		os.Exit(1)
	}
	msg = fmt.Sprintf("%s \"%s\" created", reflect.TypeOf(obj).Name(), obj.GetName())
	logging.LogI(msg)
}

func DeleteAllOfOrDie(env *ENV, obj client.Object, opts ...client.DeleteAllOfOption) {
	if err := env.Cli.DeleteAllOf(env.Ctx, obj, opts...); err != nil {
		var msg = "Error while deleting"
		logging.LogE(err, msg)
		os.Exit(1)
	}
}

func GetCLIctxOrDie(kmd *cobra.Command, args []string, allowedArgs []string) SoftwareFactoryConfigContext {
	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		logging.LogE(err, "Error initializing:")
		os.Exit(1)
	}
	if len(allowedArgs) == 0 {
		// no more validation needed
		return cliCtx
	} else {
		argumentError := errors.New("argument must be in: " + strings.Join(allowedArgs, ", "))
		if len(args) != 1 {
			logging.LogE(argumentError, "Need one argument")
			os.Exit(1)
		}
		for _, a := range allowedArgs {
			if args[0] == a {
				return cliCtx
			}
		}
		logging.LogE(argumentError, "Unknown argument "+args[0])
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
		logging.LogE(err, "Could not run command '"+cmd+"'")
		logging.LogI("Captured output:\n" + string(out))
		os.Exit(1)
	}
	return string(out)
}

func RunCmdOrDie(cmd string, args ...string) string {
	return RunCmdWithEnvOrDie([]string{}, cmd, args...)
}

func EnsureNamespaceOrDie(env *ENV, name string) {
	var ns apiv1.Namespace
	if err := env.Cli.Get(env.Ctx, client.ObjectKey{Name: name}, &ns); apierrors.IsNotFound(err) {
		ns.Name = name
		CreateROrDie(env, &ns)
	} else if err != nil {
		logging.LogE(err, "Error checking namespace "+name)
		os.Exit(1)
	}
}
func EnsureServiceAccountOrDie(env *ENV, name string) {
	var sa apiv1.ServiceAccount
	if !GetMOrDie(env, name, &sa) {
		sa.Name = name
		CreateROrDie(env, &sa)
	}
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

func GetClientset(kubeContext string) (*rest.Config, *kubernetes.Clientset) {
	restConfig := controllers.GetConfigContextOrDie(kubeContext)
	kubeClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logging.LogE(err, "Could not instantiate Clientset")
		os.Exit(1)
	}
	return restConfig, kubeClientset
}

func RunRemoteCmd(kubeContext string, namespace string, podName string, containerName string, cmdArgs []string) *bytes.Buffer {
	restConfig, kubeClientset := GetClientset(kubeContext)
	buffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}
	request := kubeClientset.CoreV1().RESTClient().Post().Resource("Pods").Namespace(namespace).Name(podName).SubResource("exec").VersionedParams(
		&apiv1.PodExecOptions{
			Container: containerName,
			Command:   cmdArgs,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		},
		scheme.ParameterCodec,
	)
	exec, _ := remotecommand.NewSPDYExecutor(restConfig, "POST", request.URL())
	err := exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: buffer,
		Stderr: errorBuffer,
	})
	if err != nil {
		errMsg := fmt.Sprintf("Command \"%s\" [Pod: %s - Container: %s] failed with the following stderr: %s",
			strings.Join(cmdArgs, " "), podName, containerName, errorBuffer.String())
		logging.LogE(err, errMsg)
		os.Exit(1)
	}
	return buffer
}

func ReadYAMLToMapOrDie(filePath string) map[string]interface{} {
	readFile, _ := GetFileContent(filePath)
	secretContent := make(map[string]interface{})
	err := yaml.Unmarshal(readFile, &secretContent)
	if err != nil {
		logging.LogE(err, "Problem on reading the file content")
	}
	if len(secretContent) == 0 {
		logging.LogE(errors.New("file is empty"), "The file is empty or it does not exist!")
		os.Exit(1)
	}
	return secretContent
}

func GetKubectlPath() string {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		logging.LogE(errors.New("no kubectl binary"),
			"No 'kubectl' binary found. Please install the 'kubectl' binary before attempting a restore")
		os.Exit(1)
	}
	return kubectlPath
}

func ExecuteKubectlClient(ns string, podName string, containerName string, executeCommand string) {
	cmd := exec.Command("sh", "-c", executeCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	if err != nil {
		logging.LogE(err, "There is an issue on executing command: "+executeCommand)
		os.Exit(1)
	}

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

func EnsureSelfSignCert(env *ENV) {
	name := "self-signed-cert"
	var secret apiv1.Secret
	if !GetMOrDie(env, name, &secret) {
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
		CreateROrDie(env, &secret)
	}
}

func ReadIngressIP(env *ENV, name string) string {
	attempt := 1
	maxTries := 25
	for {
		var ingress networkv1.Ingress
		if GetMOrDie(env, name, &ingress) {
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

func EnsureGatewayIngress(env *ENV, fqdn string) {
	EnsureSelfSignCert(env)
	ingress := MkHTTPSIngress(env.Ns, "sf-ingress", fqdn, "gateway", 8080, map[string]string{})
	if !GetMOrDie(env, ingress.Name, &ingress) {
		CreateROrDie(env, &ingress)
	}
}

func WriteIngressToEtcHosts(env *ENV, fqdn string) {
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
