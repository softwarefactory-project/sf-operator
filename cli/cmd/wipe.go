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

package cmd

/*
"wipe" subcommand cleans up Software Factory resources.
*/

import (
	"context"
	"os"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"

	v1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getOperatorSelector() labels.Selector {
	selector := labels.NewSelector()
	req, err := labels.NewRequirement(
		"operators.coreos.com/sf-operator.operators",
		selection.Exists,
		[]string{})
	if err != nil {
		ctrl.Log.Error(err, "could not set label selector to clean subscriptions")
		os.Exit(1)
	}
	return selector.Add(*req)
}

func cleanSubscription(env *ENV) {
	selector := getOperatorSelector()

	subscriptionListOpts := []client.ListOption{
		client.InNamespace("operators"),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}

	subsList := v1alpha1.SubscriptionList{}
	if err := env.Cli.List(env.Ctx, &subsList, subscriptionListOpts...); err != nil {
		ctrl.Log.Error(err, "error listing subscriptions")
		os.Exit(1)
	}
	if len(subsList.Items) > 0 {
		subscriptionDeleteOpts := []client.DeleteAllOfOption{
			client.InNamespace("operators"),
			client.MatchingLabelsSelector{
				Selector: selector,
			},
		}
		sub := v1alpha1.Subscription{}
		DeleteAllOfOrDie(env, &sub, subscriptionDeleteOpts...)
	}
}

func cleanCatalogSource(env *ENV) {
	cs := v1alpha1.CatalogSource{}
	cs.SetName("sf-operator-catalog")
	cs.SetNamespace("operators")
	if !DeleteOrDie(env, &cs) {
		ctrl.Log.Info("CatalogSource object not found")
	}
}

func cleanClusterServiceVersion(env *ENV) {
	selector := getOperatorSelector()

	subscriptionListOpts := []client.ListOption{
		client.InNamespace("operators"),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}

	csvsList := v1alpha1.ClusterServiceVersionList{}
	if err := env.Cli.List(env.Ctx, &csvsList, subscriptionListOpts...); err != nil {
		ctrl.Log.Error(err, "error listing cluster service versions")
		os.Exit(1)
	}
	if len(csvsList.Items) > 0 {
		csvDeleteOpts := []client.DeleteAllOfOption{
			client.InNamespace("operators"),
			client.MatchingLabelsSelector{
				Selector: selector,
			},
		}
		csv := v1alpha1.ClusterServiceVersion{}
		DeleteAllOfOrDie(env, &csv, csvDeleteOpts...)
	}
}

func cleanSFInstance(env *ENV, ns string) {
	var sf sfv1.SoftwareFactory
	sfDeleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(ns),
	}
	DeleteAllOfOrDie(env, &sf, sfDeleteOpts...)
	var cm apiv1.ConfigMap
	cm.SetName("sf-standalone-owner")
	cm.SetNamespace(ns)
	if !DeleteOrDie(env, &cm) {
		ctrl.Log.Info("standalone mode configmap not found")
	}
}

func cleanPVCs(env *ENV, ns string) {
	selector := labels.NewSelector()
	appReq, err := labels.NewRequirement(
		"app",
		selection.In,
		[]string{"sf"})
	if err != nil {
		ctrl.Log.Error(err, "could not set app label requirement to clean PVCs")
		os.Exit(1)
	}
	runReq, err := labels.NewRequirement(
		"run",
		selection.NotIn,
		[]string{"gerrit"})
	if err != nil {
		ctrl.Log.Error(err, "could not set run label requirement to clean PVCs")
		os.Exit(1)
	}
	selector = selector.Add([]labels.Requirement{*appReq, *runReq}...)
	pvcDeleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(ns),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	}
	var pvc apiv1.PersistentVolumeClaim
	DeleteAllOfOrDie(env, &pvc, pvcDeleteOpts...)
}

func wipeSFCmd(kmd *cobra.Command, args []string) {
	cliCtx, err := GetCLIContext(kmd)
	if err != nil {
		ctrl.Log.Error(err, "Error initializing")
		os.Exit(1)
	}
	ns := cliCtx.Namespace
	kubeContext := cliCtx.KubeContext
	delPVCs, _ := kmd.Flags().GetBool("rm-data")
	delAll, _ := kmd.Flags().GetBool("all")
	env := ENV{
		Cli: CreateKubernetesClientOrDie(kubeContext),
		Ctx: context.TODO(),
		Ns:  ns,
	}
	cleanSFInstance(&env, ns)
	if delPVCs || delAll {
		cleanPVCs(&env, ns)
	}
	if delAll {
		cleanSubscription(&env)
		cleanCatalogSource(&env)
		cleanClusterServiceVersion(&env)
	}
}

func MkWipeCmd() *cobra.Command {

	var (
		deleteData bool
		deleteAll  bool
		wipeCmd    = &cobra.Command{
			Use:   "wipe",
			Short: "wipe SF instance and related resources",
			Long: `This command can be used to remove all Software Factory instances in the provided namespace,
their persistent volumes, and even remove the SF operator completely.`,
			Run: wipeSFCmd,
		}
	)

	wipeCmd.Flags().BoolVar(&deleteData, "rm-data", false, "Delete also all persistent volumes after removing the instances. This will result in data loss, like build results and artifacts.")
	wipeCmd.Flags().BoolVar(&deleteAll, "all", false, "Remove also the operator completely from the cluster.")
	return wipeCmd
}
