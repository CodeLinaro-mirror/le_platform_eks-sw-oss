/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear

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
    goerrors "errors"
    "fmt"
    "os"

    kmmv1 "github.com/kubernetes-sigs/kernel-module-management/api/v1beta1"
    secv1 "github.com/openshift/api/security/v1"
    nfdk8s "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
    opg "github.com/operator-framework/api/pkg/operators/v1"
    subs "github.com/operator-framework/api/pkg/operators/v1alpha1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    rbacv1 "k8s.io/api/rbac/v1"

    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/log"
    xcardv1 "x100-operator/api/v1"
)

func createServiceAccount(n ControllerState, res corev1.ServiceAccount) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()

    logger := log.Log.WithValues("ServiceAccount", name, "Namespace", namespace)
    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }

        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createSecret(n ControllerState, res corev1.Secret) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()

    logger := log.Log.WithValues("Secret", name, "Namespace", namespace)
    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createRole(n ControllerState, res rbacv1.Role) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("Role", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }
    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createRoleBinding(n ControllerState, res rbacv1.RoleBinding) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("RoleBinding", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createClusterRole(n ControllerState, res rbacv1.ClusterRole) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("ClusterRole", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createClusterRoleBinding(n ControllerState, res rbacv1.ClusterRoleBinding) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("ClusterRoleBinding", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createConfigMap(n ControllerState, res corev1.ConfigMap) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("ConfigMap", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createPersistentVolume(n ControllerState, res corev1.PersistentVolume) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("PersistentVolume", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createPersistentVolumeClaim(n ControllerState, res corev1.PersistentVolumeClaim) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("PersistentVolumeClaim", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }

        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createNamespace(n ControllerState, res corev1.Namespace) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("Namespace", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createSubscription(n ControllerState, res subs.Subscription) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("Subscription", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createOperatorGroup(n ControllerState, res opg.OperatorGroup) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("OperatorGroup", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createNodeFeatureRule(n ControllerState, res nfdk8s.NodeFeatureRule) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("NodeFeatureRule", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createSecurityContextConstraints(n ControllerState, res secv1.SecurityContextConstraints) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("SecurityContextConstraints", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return xcardv1.Operational, nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return xcardv1.Operational, nil
}

func createModule(n ControllerState, res kmmv1.Module) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    log.Log.Info("Preprocessing Module to replace the fields from CR manifest")

    preProcessModule(robj, n)
    logger := log.Log.WithValues("Module", name, "Namespace", namespace)
    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            return isPodReady("kmm.node.kubernetes.io/module.name", "csm-x100-kmodules-kmm", n, "Running"), nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return isPodReady("kmm.node.kubernetes.io/module.name", "csm-x100-kmodules-kmm", n, "Running"), nil
}


func createDaemonSet(n ControllerState, res appsv1.DaemonSet) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    log.Log.Info("Preprocessing Daemonset to replace the fields from CR manifest")

    preProcessDaemonSet(robj, n)
    logger := log.Log.WithValues("DaemonSet", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Found Resource")
            return isDaemonSetReady(name, n), nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }

    return isDaemonSetReady(name, n), nil
}

func createDeployment(n ControllerState, res appsv1.Deployment) (xcardv1.State, error) {
    robj := res.DeepCopy()
    name := robj.GetName()
    namespace := robj.GetNamespace()
    logger := log.Log.WithValues("Deployment", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Found Resource")
            return isDeploymentReady(name, n), nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return isDeploymentReady(name, n), nil
}

// Creation utilities for our resource types
func createKindResource(n ControllerState, kind string, res runtime.Object) (xcardv1.State, error) {
    log.Log.Info("Asserting particular type for asset")
    error_log := ""
    var err error
    var state xcardv1.State

    switch res.(type) {
    case *corev1.ServiceAccount:
        resource := res.(*corev1.ServiceAccount)
        state, err = createServiceAccount(n, *resource)

    case *corev1.Secret:
        resource := res.(*corev1.Secret)
        state, err = createSecret(n, *resource)

    case *rbacv1.Role:
        resource := res.(*rbacv1.Role)
        state, err = createRole(n, *resource)

    case *rbacv1.RoleBinding:
        resource := res.(*rbacv1.RoleBinding)
        state, err = createRoleBinding(n, *resource)

    case *rbacv1.ClusterRole:
        resource := res.(*rbacv1.ClusterRole)
        state, err = createClusterRole(n, *resource)

    case *rbacv1.ClusterRoleBinding:
        resource := res.(*rbacv1.ClusterRoleBinding)
        state, err = createClusterRoleBinding(n, *resource)

    case *corev1.ConfigMap:
        resource := res.(*corev1.ConfigMap)
        state, err = createConfigMap(n, *resource)

    case *appsv1.DaemonSet:
        resource := res.(*appsv1.DaemonSet)
        state, err = createDaemonSet(n, *resource)

    case *appsv1.Deployment:
        resource := res.(*appsv1.Deployment)
        state, err = createDeployment(n, *resource)

    case *corev1.PersistentVolume:
        resource := res.(*corev1.PersistentVolume)
        state, err = createPersistentVolume(n, *resource)

    case *corev1.PersistentVolumeClaim:
        resource := res.(*corev1.PersistentVolumeClaim)
        state, err = createPersistentVolumeClaim(n, *resource)

    case *corev1.Namespace:
        resource := res.(*corev1.Namespace)
        state, err = createNamespace(n, *resource)

    case *subs.Subscription:
        resource := res.(*subs.Subscription)
        state, err = createSubscription(n, *resource)

    case *opg.OperatorGroup:
        resource := res.(*opg.OperatorGroup)
        state, err = createOperatorGroup(n, *resource)

    case *nfdk8s.NodeFeatureRule:
        resource := res.(*nfdk8s.NodeFeatureRule)
        state, err = createNodeFeatureRule(n, *resource)

    case *kmmv1.Module:
        resource := res.(*kmmv1.Module)
        state, err = createModule(n, *resource)

    case *secv1.SecurityContextConstraints:
        resource := res.(*secv1.SecurityContextConstraints)
        state, err = createSecurityContextConstraints(n, *resource)

    default:
        error_log = fmt.Sprintf("Object type doesn't match any of the specified known types")
    }

    if len(error_log) > 0 {
        LogAndExitOnError("", goerrors.New(error_log))
    }
    return state, err
}

// The operator starts two pods in different stages to validate
// the correct working of the DaemonSets (driver and dp). Therefore
// the operator waits until the Pod completes and checks the error status
// to advance to the next state.
func isPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) xcardv1.State {
    opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}

    log.Log.Info("DEBUG: Pod", "LabelSelector", fmt.Sprintf("%s=%s", labelkey, labelvalue))
    list := &corev1.PodList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get PodList", err)
    }
    log.Log.Info("DEBUG: Pod", "NumberOfPods", len(list.Items))
    if len(list.Items) == 0 {
        return xcardv1.NotOperational
    }

    pd := list.Items[0]

    if pd.Status.Phase != phase {
        log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "!=", phase)
        return xcardv1.NotOperational
    }
    log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "==", phase)
    return xcardv1.Operational
}

func isDeploymentReady(name string, n ControllerState) xcardv1.State {
    opts := []client.ListOption{ client.MatchingLabels{"app": name}, }

    log.Log.Info("DEBUG: DaemonSet", "LabelSelector", fmt.Sprintf("app=%s", name))
    list := &appsv1.DeploymentList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get DaemonSetList", err)
    }
    log.Log.Info("DEBUG: DaemonSet", "NumberOfDaemonSets", len(list.Items))
    if len(list.Items) == 0 {
        return xcardv1.NotOperational
    }

    ds := list.Items[0]
    log.Log.Info("DEBUG: DaemonSet", "NumberUnavailable", ds.Status.UnavailableReplicas)

    if ds.Status.UnavailableReplicas != 0 {
        return xcardv1.NotOperational
    }

    return isPodReady("app", name, n, "Running")
}

func isDaemonSetReady(name string, n ControllerState) xcardv1.State {
    opts := []client.ListOption{ client.MatchingLabels{"app": name}, }

    log.Log.Info("DEBUG: DaemonSet", "LabelSelector", fmt.Sprintf("app=%s", name))
    list := &appsv1.DaemonSetList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get DaemonSetList", err)
    }
    log.Log.Info("DEBUG: DaemonSet", "NumberOfDaemonSets", len(list.Items))
    if len(list.Items) == 0 {
        return xcardv1.NotOperational
    }

    ds := list.Items[0]
    log.Log.Info("DEBUG: DaemonSet", "NumberUnavailable", ds.Status.NumberUnavailable)

    if ds.Status.NumberUnavailable != 0 {
        return xcardv1.NotOperational
    }

    return isPodReady("app", name, n, "Running")
}

func preProcessModule(obj *kmmv1.Module, n ControllerState) {
    // Add all daemonsets here to define a mapping
    transformations := map[string]func(*kmmv1.Module, *xcardv1.X100ManagementPolicySpec, ControllerState) error{
        "csm-x100-kmodules-kmm": TransformModule,
    }

    t, ok := transformations[obj.Name]
    if !ok {
        log.Log.Info(fmt.Sprintf("No transformation for Module '%s'", obj.Name))
        return
    } else {
        log.Log.Info(fmt.Sprintf("Transformation applied to Module '%s'", obj.Name))
    }

    err := t(obj, &n.x100Policy.Spec, n)
    if err != nil {
        log.Log.Info(fmt.Sprintf("Failed to apply transformation '%s' with error: '%v'", obj.Name, err))
        os.Exit(1)
    }
}

func preProcessDaemonSet(obj *appsv1.DaemonSet, n ControllerState) {

    // Add all daemonsets here to define a mapping
    transformations := map[string]func(*appsv1.DaemonSet, *xcardv1.X100ManagementPolicySpec, ControllerState) error{
        "csm-x100-firmware-ds": TransformFirmware,
        "csm-x100-hwmanager-ds":   TransformHWManager,
        "csm-x100-deviceplugin-ds":    TransformDevicePlugin,
    }

    t, ok := transformations[obj.Name]

    if !ok {
        log.Log.Info(fmt.Sprintf("No transformation for Daemonset '%s'", obj.Name))
        return
    } else {
        log.Log.Info(fmt.Sprintf("Transformation applied to Daemonset '%s'", obj.Name))
    }

    err := t(obj, &n.x100Policy.Spec, n)
    if err != nil {
        log.Log.Info(fmt.Sprintf("Failed to apply transformation '%s' with error: '%v'", obj.Name, err))
        os.Exit(1)
    }
}

// TransformFirmware transforms k8s-firmware daemonset with required config as per x100ManagementPolicy
func TransformFirmware(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, n ControllerState) error {

    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.Firmware.ImagePath()
    // update image pull policy
    if config.Firmware.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Firmware.ImagePolicy(config.Firmware.ImagePullPolicy)
    }

    if len(config.Firmware.ImagePullSecrets) > 0 {
        for _, secret := range config.Firmware.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }

    log.Log.Info("TranformFirmware: ", "Image :", obj.Spec.Template.Spec.Containers[0].Image)
    log.Log.Info("TranformFirmware: ", "Image pull policy :", obj.Spec.Template.Spec.Containers[0].ImagePullPolicy)
    log.Log.Info("TranformFirmware: ", "Image pull secret :", obj.Spec.Template.Spec.ImagePullSecrets)

    return nil
}

// TransformHWManager transforms HWManager daemonset with required config as per x100ManagementPolicy
func TransformHWManager(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, n ControllerState) error {

    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.HwManager.ImagePath()
    // update image pull policy
    if config.HwManager.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.HwManager.ImagePolicy(config.HwManager.ImagePullPolicy)
    }

    if len(config.HwManager.ImagePullSecrets) > 0 {
        for _, secret := range config.HwManager.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }

    log.Log.Info("TranformHwMgr: ", "Image0 :", obj.Spec.Template.Spec.Containers[0].Image)
    log.Log.Info("TranformHwMgr: ", "Image1 :", obj.Spec.Template.Spec.Containers[1].Image)
    log.Log.Info("TranformHwMgr: ", "Image2 :", obj.Spec.Template.Spec.Containers[2].Image)
    log.Log.Info("TranformHwMgr: ", "Image pull policy :", obj.Spec.Template.Spec.Containers[0].ImagePullPolicy)
    log.Log.Info("TranformHwMgr: ", "Image pull secret :", obj.Spec.Template.Spec.ImagePullSecrets)

    return nil
}

// TransformModule transforms KModule daemonset with required config as per x100ManagementPolicy
func TransformModule(obj *kmmv1.Module, config *xcardv1.X100ManagementPolicySpec, n ControllerState) error {

    // update image
    //obj.Spec.Template.Spec.Containers[0].Image = config.HwManager.ImagePath()
    log.Log.Info("Transform KModule: ENTERED function")
    obj.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage = config.KModule.KModuleImagePath()
    obj.Spec.ModuleLoader.Container.ImagePullPolicy = corev1.PullAlways

    log.Log.Info("Transform KModule: after ContainerImage")
    if len(config.KModule.ImageRepoSecret) > 0 {
        obj.Spec.ImageRepoSecret = &corev1.LocalObjectReference{Name: config.KModule.ImageRepoSecret}
    }

    log.Log.Info("Transform KModule:", "Image : ", obj.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage)
    return nil
}


// TransformCsmDevicePlugin transforms DevicePlugin daemonset with required config as per x100ManagementPolicy
func TransformDevicePlugin(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, n ControllerState) error {

    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.DevicePlugin.ImagePath()
    // update image pull policy

    if config.DevicePlugin.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.DevicePlugin.ImagePolicy(config.DevicePlugin.ImagePullPolicy)
    }

    if len(config.DevicePlugin.ImagePullSecrets) > 0 {
        for _, secret := range config.DevicePlugin.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }

    log.Log.Info("TranformDevicePlugin: ", "Image :", obj.Spec.Template.Spec.Containers[0].Image)
    log.Log.Info("TranformDevicePlugin: ", "Image pull policy :", obj.Spec.Template.Spec.Containers[0].ImagePullPolicy)
    log.Log.Info("TranformDevicePlugin: ", "Image pull secret :", obj.Spec.Template.Spec.ImagePullSecrets)
    return nil

}


func setContainerEnv(c *corev1.Container, key, value string) {
    for i, val := range c.Env {
        if val.Name != key {
            continue
        }
        c.Env[i].Value = value
        return
    }
    c.Env = append(c.Env, corev1.EnvVar{Name: key, Value: value})
}
