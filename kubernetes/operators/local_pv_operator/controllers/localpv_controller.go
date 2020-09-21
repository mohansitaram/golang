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
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	uhanavmwarev1alpha1 "gitlab.eng.vmware.com/core-build/uhana_piran/api/v1alpha1"
)

const HostPvDir string = "/mnt/kubernetes/persistent_volumes/"
const localPvFinalizer string = "finalizer.localpv.uhana.vmware.com"

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

// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=uhana.vmware.my.domain,resources=localpvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list;patch;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=list;watch;create;delete

func (r *LocalPVReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("localpv", req.NamespacedName)
	ctx := context.Background()

	// Fetch the LocalPVInstance(s)
	localpv := &uhanavmwarev1alpha1.LocalPV{}
	err := r.Get(ctx, req.NamespacedName, localpv)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			log.Info("LocalPV resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
	}
	// Check if the instance was deleted
	if localpv.GetDeletionTimestamp() != nil {
		// If yes, handle deletion logic including running finalizer
		return r.deleteLocalPVHandler(localpv, log)
	}
	if !controllerutil.ContainsFinalizer(localpv, localPvFinalizer) {
		// This usually means the localPV just got created and it doesn't have the finalizer added. Add it now
		err = r.updateFinalizer(controllerutil.AddFinalizer, localpv)
		if err != nil {
			log.Error(err, "Failed to add finalizer to LocalPV instnace")
			return ctrl.Result{}, err
		}
	}
	return r.createLocalPVhandler(localpv, log)
}

func (r *LocalPVReconciler) createLocalPVhandler(localpv *uhanavmwarev1alpha1.LocalPV, log logr.Logger) (ctrl.Result, error) {
	// Check if the node labels are created, if not create them
	listOpts := []client.ListOption{}
	nodes, err := r.listNodes(log, listOpts)
	if err != nil {
		return ctrl.Result{}, err
	}
	if int32(len(nodes)) < localpv.Spec.Instances {
		errMsg := fmt.Sprintf("Number of nodes %d is smaller than the required number of instances %d for %s", len(nodes), localpv.Spec.Instances, localpv.Name)
		err = errors.NewBadRequest(errMsg)
		return ctrl.Result{}, err
	}
	var pvIndices []string
	var nodesWithoutLabel []*corev1.Node

	for _, node := range nodes {
		// Check localpv.Name on every node and create if not present
		if value, ok := node.Labels[localpv.Name]; ok {
			log.V(1).Info("Node", node.Name, "already has", "label", localpv.Name)
			pvIndices = append(pvIndices, strings.Split(value, "-")[2])
		} else {
			log.V(1).Info("Node", node.Name, "doesn't have", "label", localpv.Name)
			nodesWithoutLabel = append(nodesWithoutLabel, node)
		}
	}
	if int32(len(pvIndices)) < localpv.Spec.Instances {
		// Randomize the node list. Ideally we should take into account
		// the nodes' current free disk
		r.randomizeNodes(&nodesWithoutLabel)
		// Assign labels to nodes
		labelValuePrefix := localpv.Name + "-" + "pv" + "-"
		remainingLabelIndices := r.getDiff(pvIndices, localpv.Spec.Instances)
		for i, labelIndex := range remainingLabelIndices {
			nodeToLabel := nodesWithoutLabel[i]
			// newLabels := nodeToLabel.Labels
			label := labelValuePrefix + labelIndex
			nodeToLabel.Labels[localpv.Name] = label
			err = r.patchNodeLabels(nodeToLabel, nodeToLabel.Labels, log)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	err = r.createPersistentVolumes(localpv, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.launchFolderJobs(localpv, log, "create")
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *LocalPVReconciler) deleteLocalPVHandler(localpv *uhanavmwarev1alpha1.LocalPV, log logr.Logger) (ctrl.Result, error) {
	log.Info("Cleaning up after LocalPV instance deletion")
	err := r.finalizeLocalPV(localpv, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Finally, remove finalizer. This is required so that the CR can finally be deleted from the API server
	err = r.updateFinalizer(controllerutil.RemoveFinalizer, localpv)
	if err != nil {
		log.Error(err, "Failed to remove finalizer from LocalPV instance")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *LocalPVReconciler) finalizeLocalPV(localPV *uhanavmwarev1alpha1.LocalPV, log logr.Logger) error {
	// 1. Delete PVs and their corresponding PVCs
	var pvList = &corev1.PersistentVolumeList{}
	listOpts := []client.ListOption{
		client.MatchingLabels{"localpv": localPV.Name},
	}
	ctx := context.TODO()
	err := r.List(ctx, pvList, listOpts...)
	pvc := &corev1.PersistentVolumeClaim{}
	for _, pv := range pvList.Items {
		pvClaimRef := pv.Spec.ClaimRef
		// Sometimes the PV may not have a PVC which usually happens when there is no statefulset created to use the PV
		if pvClaimRef != nil {
			err = r.Get(ctx, types.NamespacedName{Name: pvClaimRef.Name, Namespace: pvClaimRef.Namespace}, pvc)
			if err != nil {
				log.Error(err, "Failed to get PVC")
				return err
			}
			log.Info("Deleting PVC")
			err = r.Delete(ctx, pvc)
			if err != nil {
				log.Error(err, "Failed to delete PVC.")
				return err
			}
		}
		log.Info("Deleting PV")
		err = r.Delete(ctx, &pv)
		if err != nil {
			log.Error(err, "Failed to delete PV.")
			return err
		}
	}
	// 2. Delete folders via Jobs. Check their completion status
	log.Info("Deleting folders")
	err = r.launchFolderJobs(localPV, log, "delete")
	if err != nil {
		return err
	}
	// 4. Delete labels on nodes
	err = r.deleteNodeLabels(localPV, log)
	if err != nil {
		return err
	}
	return nil
}

func (r *LocalPVReconciler) deleteNodeLabels(localpv *uhanavmwarev1alpha1.LocalPV, log logr.Logger) error {
	listOpts := []client.ListOption{}
	nodes, err := r.listNodes(log, listOpts)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, ok := node.Labels[localpv.Name]; ok {
			delete(node.Labels, localpv.Name)
			err = r.patchNodeLabels(node, node.Labels, log)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *LocalPVReconciler) createPersistentVolumes(localPV *uhanavmwarev1alpha1.LocalPV, log logr.Logger) error {
	for i := 0; int32(i) < localPV.Spec.Instances; i++ {
		pvIndex := strconv.Itoa(i)
		pvNameWithIndex := localPV.Name + "-pv-" + pvIndex
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   pvNameWithIndex,
				Labels: map[string]string{"localpv": localPV.Name},
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
		// Set LocalPV instance as the owner
		controllerutil.SetOwnerReference(localPV, pv, r.Scheme)
		err := r.Create(context.TODO(), pv)
		if err != nil && !errors.IsAlreadyExists(err) {
			// TODO: Fix logging. logr logging is non trivial
			// log.Error(err, "Failed to create PV", pvNameWithIndex)
			return err
		}
	}
	return nil
}

func (r *LocalPVReconciler) launchFolderJobs(localPV *uhanavmwarev1alpha1.LocalPV, log logr.Logger, task string) error {
	var jobs = []batchv1.Job{}
	var ttl int32 = 300
	for i := 0; int32(i) < localPV.Spec.Instances; i++ {
		pvIndex := strconv.Itoa(i)
		pvNameWithIndex := localPV.Name + "-pv-" + pvIndex
		var folderCommand string
		folderPath := HostPvDir + strings.ReplaceAll(pvNameWithIndex, "-", "_")
		if task == "create" {
			folderCommand = "mkdir -p " + folderPath + " && chmod 744 " + folderPath
		} else {
			folderCommand = "rm -rf " + folderPath
		}
		volumeName := "pv-folder"
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-" + task + "-folder-" + pvNameWithIndex,
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
							Name:    task + "-local-pv",
							Image:   "busybox",
							Command: []string{"/bin/sh", "-c", folderCommand},
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
				// Use the TTL Controller to automatically clean up these jobs
				TTLSecondsAfterFinished: &ttl,
			},
		}
		controllerutil.SetOwnerReference(localPV, job, r.Scheme)
		err := r.Create(context.TODO(), job)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		jobs = append(jobs, *job)
	}
	err := r.checkJobsStatus(jobs, log)
	if err != nil {
		return err
	}
	return nil
}

func (r *LocalPVReconciler) checkJobsStatus(jobs []batchv1.Job, log logr.Logger) error {
	timeout := time.After(120 * time.Second)
	ticker := time.Tick(1 * time.Second)
	for {
		select {
		case <-timeout:
			return errors.NewTimeoutError("Timed out waiting for folder creation Jobs to complete. Next reconcile loop might succeed", 2)
		case <-ticker:
			if len(jobs) == 0 {
				log.Info("All jobs completed successfully")
				return nil
			}
			completedJobs := []batchv1.Job{}
			for _, job := range jobs {
				foundJob := &batchv1.Job{}
				err := r.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob)
				if err != nil {
					if errors.IsNotFound(err) {
						log.Info("Job Not found. Maybe it completed successfully and got deleted. Ignoring..")
						completedJobs = append(completedJobs, job)
						continue
					}
					return err
				}
				if foundJob.Status.Succeeded > 0 {
					completedJobs = append(completedJobs, job)
				}
			}
			for _, job := range completedJobs {
				jobs = deleteJobFromJoblist(job, jobs)
			}
		}
	}
}

func deleteJobFromJoblist(jobToDelete batchv1.Job, jobs []batchv1.Job) []batchv1.Job {
	newJobList := []batchv1.Job{}
	for _, job := range jobs {
		if job.Name != jobToDelete.Name {
			newJobList = append(newJobList, job)
		}
	}
	return newJobList
}

func (r *LocalPVReconciler) updateFinalizer(f func(localPV controllerutil.Object, finalizerName string), localPV *uhanavmwarev1alpha1.LocalPV) error {
	f(localPV, localPvFinalizer)
	// Update the custom resource
	err := r.Update(context.TODO(), localPV)
	if err != nil {
		return err
	}
	return nil
}

func (r *LocalPVReconciler) randomizeNodes(nodes *[]*corev1.Node) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(*nodes), func(i, j int) {
		(*nodes)[i], (*nodes)[j] = (*nodes)[j], (*nodes)[i]
	})
}

func (r *LocalPVReconciler) listNodes(log logr.Logger, listOpts []client.ListOption) ([]*corev1.Node, error) {
	var nonCustomerNodes []*corev1.Node
	nodeList := &corev1.NodeList{}
	err := r.List(context.TODO(), nodeList, listOpts...)
	if err != nil {
		log.Error(err, "Failed to list nodes")
		return nonCustomerNodes, err
	}
	// Exclude customer/test nodes
	for _, node := range nodeList.Items {
		if _, ok := node.Labels["customer"]; !ok {
			nonCustomerNodes = append(nonCustomerNodes, &node)
			log.V(1).Info("Node", node.Name, "is not a customer node")
		}
	}

	var nodeNames []string
	for _, node := range nonCustomerNodes {
		nodeNames = append(nodeNames, node.Name)
	}
	log.V(1).Info("Unchecked nodes are", "nodes", nodeNames)

	return nonCustomerNodes, nil
}

func (r *LocalPVReconciler) patchNodeLabels(node *corev1.Node, labels map[string]string, log logr.Logger) error {
	log.V(1).Info("Updating labels on", "node", node.Name)
	newLabels := make([]LabelReplace, 1)
	newLabels[0].Op = "replace"
	newLabels[0].Path = "/metadata/labels"
	newLabels[0].Value = labels
	patchBytes, _ := json.Marshal(newLabels)
	patch := client.RawPatch(types.JSONPatchType, patchBytes)
	err := r.Patch(context.TODO(), node, patch)
	if err != nil {
		log.Error(err, "Failed to patch node", node.Name, "with labels", labels)
		return err
	}
	return nil
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
