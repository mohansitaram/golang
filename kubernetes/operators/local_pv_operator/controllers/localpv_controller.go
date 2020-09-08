/*
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

package controllers

import (
	"context"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	uhanavmwarev1alpha1 "gitlab.eng.vmware.com/core-build/uhana_piran/api/v1alpha1"
)

type LabelReplace struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

// LocalPVReconciler reconciles a LocalPV object
type LocalPVReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list;patch;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=list;watch;create;delete

func (r *LocalPVReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("localpv", req.NamespacedName)
	log.Info("Received request")
	ctx := context.Background()

	// Fetch the LocalPVInstance(s)
	log.Info("Fetching localpv instance")
	localpv := &uhanavmwarev1alpha1.LocalPV{}
	err := r.Client.Get(ctx, req.NamespacedName, localpv)
	log.Info("Sent API request to fetch instance. Checking result..")
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("LocalPV resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
	}
	// Check if the node labels are created, if not create them
	nodeList := &corev1.NodeList{}
	listOpts := []client.ListOption{}
	log.Info("Listing nodes")
	err = r.Client.List(ctx, nodeList, listOpts...)
	if err != nil {
		log.Info("Failed to list nodes. Generating error")
		log.Error(err, "Failed to list nodes")
		return ctrl.Result{}, err
	}
	log.Info("Nodes are ", nodeList.Items)
	var pvIndices []string
	var nodesWithoutLabel []corev1.Node
	for _, node := range nodeList.Items {
		log.Info("Checking labels on ", node.Name)
		// Check localpv.Name on every node and create if not present
		for label, value := range node.Labels {
			if label == localpv.Name {
				log.Info("Node ", node.Name, " already has label ", label)
				pvIndices = append(pvIndices, strings.Split(value, "-")[2])
			} else {
				log.Info(node.Name, " doesn't have the label")
				nodesWithoutLabel = append(nodesWithoutLabel, node)
			}
		}
	}
	log.Info("May or may not patch node here")
	if int32(len(pvIndices)) < localpv.Spec.Instances {
		// Randomize the node list. Ideally we should take into account
		// the nodes' current free disk
		r.RandomizeNodes(nodesWithoutLabel[:])
		log.Info("Randomized nodes")
		// Assign labels to nodes
		labelValuePrefix := localpv.Name + "-" + "pv" + "-"
		remainingLabelIndices := r.getDiff(pvIndices, localpv.Spec.Instances)
		log.Info("Remaining label indices are ", remainingLabelIndices)
		for i, labelIndex := range remainingLabelIndices {
			nodeToLabel := nodesWithoutLabel[i]
			// TODO: Create the label and patch nodeToLabel
			nodeLabels := nodeToLabel.Labels
			label := labelValuePrefix + labelIndex
			nodeLabels[localpv.Name] = label
			newLabels := make([]LabelReplace, 1)
			newLabels[0].Op = "replace"
			newLabels[0].Path = "/metadata/labels"
			newLabels[0].Value = nodeLabels
			patchBytes, _ := json.Marshal(newLabels)
			log.Info("Patching node ", nodeToLabel.Name)
			patch := client.RawPatch(types.JSONPatchType, patchBytes)
			err = r.Client.Patch(ctx, &nodeToLabel, patch)
			if err != nil {
				log.Error(err, "Failed to patch node ", nodeToLabel.Name, " with label ", label)
			} else {
				log.Info("Successfully assigned label ", label, " to node ", nodeToLabel.Name)
			}
		}
	}
	log.Info("Ending Reconcile loop")
	return ctrl.Result{}, nil
}

func (r *LocalPVReconciler) RandomizeNodes(nodes []corev1.Node) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})
}

func (r *LocalPVReconciler) getDiff(pvIndices []string, numInstances int32) []string {
	tempMap := make(map[int]string)
	for _, pvIndex := range pvIndices {
		i, _ := strconv.Atoi(pvIndex)
		tempMap[i] = pvIndex
	}
	var diff []string
	for i := 0; i < int(numInstances); i++ {
		_, ok := tempMap[i]
		if !ok {
			diff = append(diff, strconv.Itoa(i))
		}
	}
	return diff
}

func (r *LocalPVReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uhanavmwarev1alpha1.LocalPV{}).
		Owns(&corev1.PersistentVolume{}).
		Complete(r)
}
