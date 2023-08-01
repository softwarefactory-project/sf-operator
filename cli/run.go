// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the main sfconfig CLI for the end user.
// The goal is to be a onestop shop to get the service running with a single `sfconfig` command invocation.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/gerrit"
	"github.com/softwarefactory-project/sf-operator/cli/sfconfig/cmd/utils"
	controllers "github.com/softwarefactory-project/sf-operator/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
)

func Run(erase bool) {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))
	fmt.Println("sfconfig started with: ", GetConfigOrDie())
	env := utils.ENV{
		Ctx: context.TODO(),
		Ns:  "sf",
		Cli: utils.CreateKubernetesClient(""),
	}
	if erase {
		fmt.Println("Erasing...")
		// TODO: remove the sfconfig resource and the pv
	} else {
		// TODO: only do gerrit when provision demo is on?
		gerrit.EnsureGerrit(&env, "sftests.com")
		EnsureDeployement()
	}
}

// The goal of this function is to ensure a deployment is running.
func EnsureDeployement() {
	fmt.Println("[+] Checking SF resource...")
	sf, err := utils.GetSF("my-sf")
	if sf.Status.Ready {
		// running the operator should be a no-op
		RunOperator()

		fmt.Println("Software Factory is already ready!")
		// TODO: connect to the Zuul API and ensure it is running
		fmt.Println("Check https://zuul." + sf.Spec.FQDN)
		os.Exit(0)

	} else if err != nil {
		if errors.IsNotFound(err) {
			// The resource does not exist
			EnsureCR()
			EnsureCertManager()
			RunOperator()

		} else if utils.IsCRDMissing(err) {
			// The resource definition does not exist
			EnsureCRD()
			EnsureCR()
			EnsureCertManager()
			RunOperator()

		} else {
			// TODO: check what is the actual error and suggest counter measure, for example:
			// if microshift host is up but service is done, apply the ansible-microshift-role
			// if kubectl is not connecting ask for reboot or rebuild
			fmt.Printf("Error %v\n", errors.IsInvalid(err))
		}

	} else {
		// Software Factory resource exists, but it is not ready
		if IsOperatorRunning() {
			// TODO: check operator status
			// TODO: check cluster status and/or suggest sf resource delete/recreate
		} else {
			EnsureCertManager()
			RunOperator()
		}
	}

	// TODO: suggest sfconfig --erase if the command does not succeed.
	fmt.Println("[+] Couldn't deploy your software factory, sorry!")
}

func EnsureCR() {
	// TODO: implement natively
	cmd := exec.Command("kubectl", "apply", "-f", "config/samples/sf_v1_softwarefactory.yaml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("install CR failed: %w", err))
	}
}

func EnsureCRD() {
	// TODO: implement natively and avoir re-entry
	fmt.Println("[+] Installing CRD...")
	runMake("install")
}

func EnsureCertManager() {
	// TODO: implement natively
	fmt.Println("[+] Installing Cert-Manager...")
	runMake("install-cert-manager")
	// Mitigate the issue
	// failed calling webhook "mutate.webhooks.cert-manager.io": failed to call webhook: Post "https://cert-manager-webhook-service.operators.svc:443/mutate?timeout=10s": no endpoints available for service "cert-manager-webhook-service"
	fmt.Println("[+] Waiting for Cert-Manager")
	for i := 0; i < 10; i++ {
		if utils.IsCertManagerRunning() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	panic("cert-manager didn't become ready")
}

func RunOperator() {
	fmt.Println("[+] Running the operator...")
	controllers.Main("sf", ":8081", ":8080", false, true)
}

// temporary hack until make target are implemented natively
func runMake(arg string) {
	cmd := exec.Command("make", arg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("make %s failed: %w", arg, err))
	}
}

func IsOperatorRunning() bool {
	// TODO: implement
	return false
}
