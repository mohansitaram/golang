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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	uhanavmwarev1alpha1 "gitlab.eng.vmware.com/core-build/uhana_piran/api/v1alpha1"
)

const HostPvDir string = "/mnt/kubernetes/persistent_volumes/"

type LabelReplace struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

// LocalPVReconciler reconciles a LocalPV object
type LocalPVReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
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

// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list;patch;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=list;watch;create;delete

func (r *LocalPVReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("localpv", req.NamespacedName)
	log.Info("Received request")
	ctx := context.Background()

	// Fetch the LocalPVInstance(s)
	log.Info("Fetching localpv instance")
	localpv := &uhanavmwarev1alpha1.LocalPV{}
	err := r.Get(ctx, req.NamespacedName, localpv)
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
	err = r.List(ctx, nodeList, listOpts...)
	if err != nil {
		log.Info("Failed to list nodes. Generating error")
		log.Error(err, "Failed to list nodes")
		return ctrl.Result{}, err
	}
	var pvIndices []string
	var nodesWithoutLabel []corev1.Node
	for _, node := range nodeList.Items {
		// Check localpv.Name on every node and create if not present
		for label, value := range node.Labels {
			if label == localpv.Name {
				// log.Info("Node ", node.Name, " already has label ", label)
				pvIndices = append(pvIndices, strings.Split(value, "-")[2])
			} else {
				//log.Info(node.Name, " doesn't have the label")
				nodesWithoutLabel = append(nodesWithoutLabel, node)
			}
		}
	}
	if int32(len(pvIndices)) < localpv.Spec.Instances {
		// Randomize the node list. Ideally we should take into account
		// the nodes' current free disk
		r.RandomizeNodes(nodesWithoutLabel[:])
		// Assign labels to nodes
		labelValuePrefix := localpv.Name + "-" + "pv" + "-"
		remainingLabelIndices := r.getDiff(pvIndices, localpv.Spec.Instances)
		for i, labelIndex := range remainingLabelIndices {
			nodeToLabel := nodesWithoutLabel[i]
			nodeLabels := nodeToLabel.Labels
			label := labelValuePrefix + labelIndex
			nodeLabels[localpv.Name] = label
			newLabels := make([]LabelReplace, 1)
			newLabels[0].Op = "replace"
			newLabels[0].Path = "/metadata/labels"
			newLabels[0].Value = nodeLabels
			patchBytes, _ := json.Marshal(newLabels)
			patch := client.RawPatch(types.JSONPatchType, patchBytes)
			err = r.Patch(ctx, &nodeToLabel, patch)
			if err != nil {
				log.Error(err, "Failed to patch node", nodeToLabel.Name, "with label", label)
				return ctrl.Result{}, err
			}
		}
	}
	err = r.CreatePersistentVolumes(req, ctx, localpv)
	if err != nil {
		return ctrl.Result{}, err
	}
	// TODO:
	// On CRD creation:
	//   1. Create Job to create folder on labeled node. Then delete the job on successful creation
	err = r.CreateJobToCreateFolder(req, ctx, localpv)
	if err != nil {
		return ctrl.Result{}, err
	}
	// On CRD deletion
	//   1. Use PV as 'owned' object. Owned objects are auto garbaged collected on deletion
	//   2. Labels and folders can't be 'owned' objects since they are not K8s native resources. Use finalizers for cleaning up labels and folders
	return ctrl.Result{}, nil
}

func (r *LocalPVReconciler) CreatePersistentVolumes(req ctrl.Request, ctx context.Context, localPV *uhanavmwarev1alpha1.LocalPV) error {
	// log := r.Log.WithValues("localpv", req.NamespacedName)
	for i := 0; int32(i) < localPV.Spec.Instances; i++ {
		pvIndex := strconv.Itoa(i)
		pvNameWithIndex := localPV.Name + "-pv-" + pvIndex
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvNameWithIndex,
			},
			Spec: corev1.PersistentVolumeSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.PersistentVolumeAccessMode(localPV.Spec.AccessMode)},
				Capacity: corev1.ResourceList{
					"storage": resource.MustParse(localPV.Spec.Size),
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimPolicy(localPV.Spec.PVCReclaimPolicy),
				StorageClassName:              localPV.Spec.StorageClass,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					Local: &corev1.LocalVolumeSource{
						Path: HostPvDir + strings.ReplaceAll(pvNameWithIndex, "-", "_"),
					},
				},
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									corev1.NodeSelectorRequirement{
										Key:      localPV.Name,
										Operator: "In",
										Values:   []string{pvNameWithIndex},
									},
								},
							},
						},
					},
				},
			},
		}
		// Set LocalPV instance as the owner and controller
		ctrl.SetControllerReference(localPV, pv, r.Scheme)
		err := r.Create(ctx, pv)
		if err != nil && !errors.IsAlreadyExists(err) {
			// TODO: Fix logging. logr logging is non trivial
			// log.Error(err, "Failed to create PV", pvNameWithIndex)
			return err
		}
	}
	return nil
}

func (r *LocalPVReconciler) CreateJobToCreateFolder(req ctrl.Request, ctx context.Context, localPV *uhanavmwarev1alpha1.LocalPV) error {
	// var jobs = []batchv1.Job{}
	// log := r.Log.WithValues("localpv", req.NamespacedName)
	var ttl int32 = 300
	for i := 0; int32(i) < localPV.Spec.Instances; i++ {
		pvIndex := strconv.Itoa(i)
		pvNameWithIndex := localPV.Name + "-pv-" + pvIndex
		folderPath := HostPvDir + strings.ReplaceAll(pvNameWithIndex, "-", "_")
		createFolderCommand := "mkdir -p " + folderPath + " && chmod 777 " + folderPath
		volumeName := "pv-folder"
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-create-folder-" + pvNameWithIndex,
				Namespace: localPV.Namespace,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{corev1.Volume{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: HostPvDir,
								},
							},
						}},
						Containers: []corev1.Container{corev1.Container{
							Name:    "create-local-pv",
							Image:   "busybox",
							Command: []string{"/bin/sh", "-c", createFolderCommand},
							VolumeMounts: []corev1.VolumeMount{corev1.VolumeMount{
								Name:      volumeName,
								ReadOnly:  false,
								MountPath: HostPvDir,
							}},
						}},
						RestartPolicy: corev1.RestartPolicyOnFailure,
						NodeSelector:  map[string]string{localPV.Name: pvNameWithIndex},
					},
				},
				// Use the TTL Controller to cleanup these jobs
				TTLSecondsAfterFinished: &ttl,
			},
		}
		ctrl.SetControllerReference(localPV, job, r.Scheme)
		err := r.Create(ctx, job)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		// jobs = append(jobs, job)
	}
	return nil
}

func (r *LocalPVReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&uhanavmwarev1alpha1.LocalPV{}).
		Owns(&corev1.PersistentVolume{}).
		Complete(r)
}
