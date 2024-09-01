/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear

Copyright 2023.

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
	"fmt"
	kmmv1 "github.com/kubernetes-sigs/kernel-module-management/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
	xcardv1 "x100-operator/api/v1"
)

var x100Ctrl ControllerState

// X100ManagementPolicyReconciler reconciles a X100ManagementPolicy(CRD) object
type X100ManagementPolicyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTClient rest.Interface
	RESTConfig *rest.Config
}

func (r *X100ManagementPolicyReconciler) clearLabelsOnCrDeletion(policyInstance *xcardv1.X100ManagementPolicy) error {
	/***
		Deletion supports only deletion of policy and no upgrade/downgrade.
		All nodes under the policy would have the policy owned resources
		deleted from respective nodes.An additional label (x100OwnerPolicyDeleted)
		is attached	to affected nodes so that new policy is not attached to them
		unless user explicitly expresses the intent to do so by putting the node
		in the selector list of some other policy.

	***/
	policyopts := []client.ListOption{}
	policylist := &xcardv1.X100ManagementPolicyList{}
	err := r.List(context.TODO(), policylist, policyopts...)
	if err != nil {
		log.Log.Error(err, "clearLabelsOnCrDeletion()- Unable to list X100ManagementPolicy CRs")
	}

	log.Log.Info(fmt.Sprintf("Deleting policy %s, found %v x100managementpolicies in the cluster",
		policyInstance.ObjectMeta.Name, len(policylist.Items)))

	opts := []client.ListOption{}
	list := &corev1.NodeList{}
	err = r.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
	}
	for _, node := range list.Items {
		labels := node.GetLabels()
		if isX100RunningWithPolicy(labels, policyInstance.ObjectMeta.Name) {
			log.Log.Info("Deleting policy which is running on current node")
			labels = cleanupStaleCRLabels(labels)
			labels = cleanupStaleSelectorLabels(labels)
			if len(policylist.Items) > 1 {
				labels[x100OwnerPolicyDeleted] = "true"
			}
			node.SetLabels(labels)
			err = x100Ctrl.setX100NodeLabels(&node, labels, x100SwVersion, LabelUpdateDeletionType)
			//err = r.Update(context.TODO(), &node)
			if err != nil {
				return fmt.Errorf("Unable to delete node label for %s , err %s", node.ObjectMeta.Name, err.Error())
			}
		} else {
			// If the only policy available is being deleted
			if len(policylist.Items) == 1 {
				log.Log.Info("Deleting last X100ManagementPolicy in the cluster")
				if hasX100OwnerPolicyDeletedLabel(labels) {
					delete(labels, x100OwnerPolicyDeleted)
				}
				if hasCustomX100Label(labels) {
					delete(labels, x100LabelKey)
				}
				node.SetLabels(labels)
				//err = r.Update(context.TODO(), &node)
				err = x100Ctrl.setX100NodeLabels(&node, labels, x100LabelKey, LabelUpdateDeletionType)
				if err != nil {
					return fmt.Errorf("Unable to delete node label for %s , err %s", node.ObjectMeta.Name, err.Error())
				}
			}
		}
	}
	return nil
}

func (r *X100ManagementPolicyReconciler) deletePolicyOwnedResources(policyInstance *xcardv1.X100ManagementPolicy) error {
	log.Log.Info("qualcomm.com/finalizer invoked the cleanup logic", policyInstance.ObjectMeta.Name, policyInstance.Spec.NodeSelector)
	err := r.clearLabelsOnCrDeletion(policyInstance)
	return err
}

// +kubebuilder:rbac:groups=qualcomm.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=qualcomm.com,resources=x100managementpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;pods/exec;pods/attach;services;services/finalizers;endpoints;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets;nodes;persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=privileged,verbs=get;list;watch;create;update;patch;delete;use
// +kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nfd.k8s-sigs.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=*

func (r *X100ManagementPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Loop iterates over the assets that are being managed by the policy
	// Maintains the overall state of the CR to be Operational or NotOperational

	logger := log.Log.WithValues("Reconciling: ", req.NamespacedName)

	// Fetch the CRD instance
	policyInstance := &xcardv1.X100ManagementPolicy{}

	logger.Info("X100ManagementPolicy reconcile for CR: ", "-", policyInstance.ObjectMeta.Name)
	err := r.Get(ctx, req.NamespacedName, policyInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	x100finalizer := "qualcomm.com/finalizer"
	if policyInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(policyInstance, x100finalizer) {
			controllerutil.AddFinalizer(policyInstance, x100finalizer)
			if err := r.Update(ctx, policyInstance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		// Delete policy and related resources from all nodes under this policy
		if controllerutil.ContainsFinalizer(policyInstance, x100finalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deletePolicyOwnedResources(policyInstance); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(policyInstance, x100finalizer)
			if err := r.Update(ctx, policyInstance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Reboot path handling
	opts := []client.ListOption{}
	list := &corev1.NodeList{}
	err = r.List(context.TODO(), list, opts...)
	if err != nil {
		return reconcile.Result{}, err
	}

	reconcilerTriggeredOnReboot := false
	for _, node := range list.Items {
		for _, ns := range policyInstance.Spec.NodeSelector {
			if node.ObjectMeta.Name == ns {
				labels := node.GetLabels()
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionUnknown {
						reconcilerTriggeredOnReboot = true
						if hasX100UpgradeFailedLabel(labels) {
							// Prioritize upgrade rollback over reboot
							log.Log.Info(fmt.Sprintf("Not setting reboot label to let pending downgrade resume later"))
							return reconcile.Result{}, nil
						}

						if !hasX100ComingUpAfterRebootLabel(labels) {
							log.Log.Info(fmt.Sprintf("Reconciler loop hit on reboot detection, setting reboot label"))
							// We entered reconcile due to node entering reboot path
							labels = cleanupLabelsOnReboot(labels)
							// Label to indicate that we were in reboot path
							labels[X100ComingUpAfterReboot] = "true"
							node.SetLabels(labels)
							//err = r.Update(context.TODO(), &node)
							err = x100Ctrl.setX100NodeLabels(&node, labels, X100ComingUpAfterReboot, LabelUpdateAdditionType)
							if err != nil {
								log.Log.Info(fmt.Sprintf("Unable to reset node labels for %s in reboot path, err %s",
									node.ObjectMeta.Name, err.Error()))
								return reconcile.Result{}, err
							}
						}
					}
				}
			}
		}
	}

	if reconcilerTriggeredOnReboot {
		log.Log.Info("Exiting reconciler, waiting for one or more nodes to reboot")
		return reconcile.Result{}, nil
	}

	// State machine takes care of analyzing the state
	overallStatus := xcardv1.Operational

	// Init the state machine
	err = x100Ctrl.start(r, policyInstance, &policyInstance.Spec)
	if err != nil {
		log.Log.Info("Failed to initialize X100ManagementPolicy controller during start()")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	//If Policy already exists, has no nodes , no need to Reconcile
	//will this affect the logic to have empty node selector list for
	//selection of all nodes
	if len(policyInstance.Spec.NodeSelector) == 0 {
		logger.Info(fmt.Sprintf("Policy %s has no nodes in selector list, not reconciling...",
			policyInstance.ObjectMeta.Name))
		return ctrl.Result{}, nil
	} else {
		logger.Info(fmt.Sprintf("Policy %s has %d nodes in selector list at reconcile start",
			policyInstance.ObjectMeta.Name, len(policyInstance.Spec.NodeSelector)))
	}

	retryCount := 100
	for retryCount > 0 {
		// Trigger the state machine
		status, err := x100Ctrl.triggerStateMachine(r, policyInstance)
		if err != nil {
			log.Log.Info("Failed to complete X100ManagementPolicy controller state machine during triggerStateMachine()")
			return ctrl.Result{RequeueAfter: time.Second * 20}, err
		}

		if status == xcardv1.NotOperational {
			if len(policyInstance.Spec.NodeSelector) == 0 {
				// We are looping over create resources where pods won't come up
				// Because the policy has no nodes in the selector List
				// Break this infinite loop and trigger reconciler again for clean up
				logger.Info(fmt.Sprintf("Policy %s has no nodes in selector list, quit trying to create pods", policyInstance.ObjectMeta.Name))
				return ctrl.Result{}, nil
			}

			// Avoid looping when we have a pending deletion Trigger
			if !policyInstance.ObjectMeta.DeletionTimestamp.IsZero() {
				if controllerutil.ContainsFinalizer(policyInstance, x100finalizer) {
					// our finalizer is present, so lets handle any external dependency
					if err := r.deletePolicyOwnedResources(policyInstance); err != nil {
						// if fail to delete the external dependency here, return with error
						// so that it can be retried
						return ctrl.Result{}, err
					}

					// remove our finalizer from the list and update it.
					controllerutil.RemoveFinalizer(policyInstance, x100finalizer)
					if err := r.Update(ctx, policyInstance); err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
			}

			// if CR was previously set to Operational(prior reboot etc), reset it to current state
			if policyInstance.Status.State == xcardv1.Operational {
				updateCRState(r, ctx, req.NamespacedName, xcardv1.NotOperational)
			}
			// If the resource is not Operational, log status and proceed with other components
			logger.Info("X100ManagementPolicy step wasn't Operational", "State:", status)
			overallStatus = xcardv1.NotOperational

			//Cases where labelling needs to be redone
			err = x100Ctrl.labelX100Nodes(policyInstance, &policyInstance.Spec)
			if err != nil {
				return ctrl.Result{RequeueAfter: time.Second * 10}, err
			}
			err = x100Ctrl.labelX100NodeswithCR(policyInstance, &policyInstance.Spec)
			if err != nil {
				//return ctrl.Result{RequeueAfter: time.Second * 10}, err
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
			// introduce an artificial sleep to avoid too much looping here
			// Give time to pods to come up
			time.Sleep(5 * time.Second)
		}

		retryCount--
		if x100Ctrl.stateMachineCompleted() {
			break
		}
	}

	// if any asset state is not Operational, requeue for reconcile after 5 seconds
	if overallStatus != xcardv1.Operational {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	// Success case, no need to requeue the request
	updateCRState(r, ctx, req.NamespacedName, overallStatus)
	return ctrl.Result{}, nil
}

func updateCRState(r *X100ManagementPolicyReconciler, ctx context.Context,
	namespacedName types.NamespacedName, state xcardv1.State) error {
	// Update the state to reflect the current state of the CR
	policyInstance := &xcardv1.X100ManagementPolicy{}
	err := r.Get(ctx, namespacedName, policyInstance)
	if err != nil {
		log.Log.Error(err, "Failed to get X100ManagementPolicy instance for status update")
		return err
	}
	// Update the CR state
	policyInstance.SetState(state)
	err = r.Status().Update(ctx, policyInstance)
	if err != nil {
		log.Log.Error(err, "Failed to update X100ManagementPolicy status")
		return err
	}
	return nil
}

// Create custom event and handler for custom event to watch nodes for label changes

func watchx100NodeLabelChanges(r *X100ManagementPolicyReconciler, c controller.Controller, mgr manager.Manager) error {
	mapFn := func(ctx context.Context, a client.Object) []reconcile.Request {
		opts := []client.ListOption{}
		list := &xcardv1.X100ManagementPolicyList{}

		err := mgr.GetClient().List(context.TODO(), list, opts...)
		if err != nil {
			log.Log.Error(err, "Unable to list X100ManagementPolicy")
			return []reconcile.Request{}
		}

		requests := []reconcile.Request{}

		for _, policy := range list.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      policy.ObjectMeta.GetName(),
				Namespace: policy.ObjectMeta.GetNamespace(),
			}})
		}
		log.Log.Info("Reconcile policies after node label update", "count:", len(requests))
		return requests
	}

	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			hasPCILabel := hasX100PCILabels(labels)
			hasCustomLabel := hasCustomX100Label(labels)

			x100LabelMissing := hasPCILabel && !hasCustomLabel

			log.Log.Info("Labels->", "hasX100PCILabels:", hasPCILabel, "hasCustomX100Label:", hasCustomLabel, "x100LabelMissing:", x100LabelMissing)
			if x100LabelMissing {
				log.Log.Info("[Create Event] New node needs an update, x100 custom label is missing", "name", e.Object.GetName())
			}
			return x100LabelMissing
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newnode, ok := e.ObjectNew.(*corev1.Node)
			if ok {
				if newnode.Spec.Unschedulable {
					log.Log.Info("Node is Unschedulable, don't hit the reconciler for this")
					return false
				}
			}

			newLabels := e.ObjectNew.GetLabels()
			newNode := e.ObjectNew.(*corev1.Node)

			//log.Log.Info("Node Labels Updated - Enque reconcile requests on available CRs")
			//log.Log.Info("Node Labels ->", "newLabels: ", newLabels)
			// Trigger reconcile loop on transition to notReady state
			// Detect the same in reconcile loop
			// Enter this only if we are coming from the reboot path
			// as we cleanup labels in this path
			isX100Rebooting := hasX100ComingUpAfterRebootLabel(newLabels)
			if !isX100Rebooting {
				for _, condition := range newNode.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionUnknown {
						return true
					}
				}
			} else {
				log.Log.Info("Detected reboot label while handling update event, triggering reconciler")
				return true
			}

			hasPCILabel := hasX100PCILabels(newLabels)
			hasCustomLabel := hasCustomX100Label(newLabels)
			hasIsolateLabel := hasX100IsolateLabel(newLabels)

			hasRolledBackLabel := hasX100RolledBackLabel(newLabels)

			hasUpgradeFailedLabel := hasX100UpgradeFailedLabel(newLabels)
			hasUpgradingLabel := hasX100UpgradingLabel(newLabels)

			x100LabelMissing := hasPCILabel && !hasCustomLabel
			x100LabelOutdated := (hasIsolateLabel || !hasPCILabel) && hasCustomLabel

			inUpgradePath := hasUpgradeFailedLabel || hasUpgradingLabel || hasRolledBackLabel

			needsUpdate := x100LabelMissing || x100LabelOutdated || inUpgradePath

			if needsUpdate {
				log.Log.Info("[Update Event] Node needs an update", "name", e.ObjectNew.GetName(),
					"x100LabelMissing", x100LabelMissing,
					"x100LabelOutdated", x100LabelOutdated,
					"inUpgradePath", inUpgradePath)
			}
			return needsUpdate
		},
	}

	err := c.Watch(source.Kind(mgr.GetCache(), &corev1.Node{}), handler.EnqueueRequestsFromMapFunc(mapFn), p)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *X100ManagementPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	c, err := controller.New("X100ManagementPolicy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.Node{}, "metadata.name", func(rawObj client.Object) []string {
		node := rawObj.(*corev1.Node)
		return []string{node.ObjectMeta.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &kmmv1.Module{}, "metadata.name", func(rawObj client.Object) []string {
		module := rawObj.(*kmmv1.Module)
		return []string{module.ObjectMeta.Name}
	}); err != nil {
		return err
	}

	// Watch for changes to primary resource X100ManagementPolicy
	err = c.Watch(source.Kind(mgr.GetCache(), &xcardv1.X100ManagementPolicy{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Add custom implementation of mapFunction for other resources like NFD label changes to watch
	// Watch for changes to Node labels and requeue the owner policy
	err = watchx100NodeLabelChanges(r, c, mgr)
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	err = c.Watch(
		source.Kind(mgr.GetCache(), &corev1.Pod{}),
		handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &xcardv1.X100ManagementPolicy{}, handler.OnlyControllerOwner()),
	)

	if err != nil {
		return err
	}

	return nil
}
