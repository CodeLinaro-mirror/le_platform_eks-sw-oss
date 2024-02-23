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
    "time"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
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
    xcardv1 "x100-operator/api/v1"
    "k8s.io/client-go/rest"
)

var x100Ctrl ControllerState


// X100ManagementPolicyReconciler reconciles a X100ManagementPolicy(CRD) object
type X100ManagementPolicyReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    RESTClient rest.Interface
    RESTConfig *rest.Config
}

func (r *X100ManagementPolicyReconciler) clearLabelsOnCrDeletion(policyInstance *xcardv1.X100ManagementPolicy) error {
    opts := []client.ListOption{}
    list := &corev1.NodeList{}
    err := r.List(context.TODO(), list, opts...)
    if err != nil {
        return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
    }
    for _, node := range list.Items {
        labels := node.GetLabels()
        if hasActiveCRDLabel(labels, policyInstance.ObjectMeta.Name) {
            labels = cleanupStaleCRDLabels(labels)
            node.SetLabels(labels)
            err = r.Update(context.TODO(), &node)
            if err != nil {
                return fmt.Errorf("Unable to Delete node label for %s , err %s", node.ObjectMeta.Name, err.Error())
            }
        }
    }
    return nil
}

func (r *X100ManagementPolicyReconciler) deleteExternalResources(policyInstance *xcardv1.X100ManagementPolicy) error {

    // delete any external resources associated with the CR
    // Ensure that delete implementation is idempotent and safe to invoke
    // multiple times for same object.
    log.Log.Info("FINALIZER invoked the clean-up logic",policyInstance.ObjectMeta.Name, policyInstance.Spec.NodeSelectors)
    //Fetch all the nodes in cluster. For each node, see if the activecrd is current policyInstance,
    //then delete all x100 related labels(if the crd is not current, skip the node)
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
        if controllerutil.ContainsFinalizer(policyInstance, x100finalizer) {
            // our finalizer is present, so lets handle any external dependency
            if err := r.deleteExternalResources(policyInstance); err != nil {
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
    // State machine takes care of analyzing the state
    overallStatus := xcardv1.Operational

    // Init the state machine
    err = x100Ctrl.start(r, policyInstance, &policyInstance.Spec)
    if err != nil {
        log.Log.Error(err, "Failed to initialize X100ManagementPolicy controller")
        return ctrl.Result{}, err
    }

    for {
        // Trigger the state machine
        status, err := x100Ctrl.triggerStateMachine(r, policyInstance)
        if err != nil {
            logger.Error(err, "Failed to initialize X100ManagementPolicy controller and dependencies")
            return ctrl.Result{RequeueAfter: time.Second * 20}, err
        }

        if status == xcardv1.NotOperational {
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
            err = x100Ctrl.labelX100NodeswithCRDFields(policyInstance, &policyInstance.Spec)
            if err != nil {
                return ctrl.Result{RequeueAfter: time.Second * 10}, err
            }
            // introduce an artificial sleep to avoid too much looping here
            // Give time to pods to come up
            time.Sleep(5 * time.Second)
        }
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
                log.Log.Info("[Create Event] New node needs an update, x100 custom label is missing","name", e.Object.GetName())
            }
            return x100LabelMissing
        },
        UpdateFunc: func(e event.UpdateEvent) bool {
            newLabels := e.ObjectNew.GetLabels()
            log.Log.Info("Node Labels Updated - Enque reconcile requests on available CRs")
            //log.Log.Info("Node Labels ->", "newLabels: ", newLabels)

            hasPCILabel := hasX100PCILabels(newLabels)
            hasCustomLabel := hasCustomX100Label(newLabels)

            x100LabelMissing := hasPCILabel && !hasCustomLabel
            x100LabelOutdated := !hasPCILabel && hasCustomLabel

            needsUpdate := x100LabelMissing || x100LabelOutdated

            if needsUpdate {
                log.Log.Info("[Update Event] Node needs an update","name", e.ObjectNew.GetName(),
                    "x100LabelMissing", x100LabelMissing,
                    "x100LabelOutdated", x100LabelOutdated)
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
        handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &xcardv1.X100ManagementPolicy{}, handler.OnlyControllerOwner(),),
    )

    if err != nil {
        return err
    }

    return nil
}
