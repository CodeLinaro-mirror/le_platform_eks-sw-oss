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
var NamePrefixes = map[string]string{
	"firmware":     "csm-x100fw-daemonset",
	"HwManager":    "csm-x100hwmgr-daemonset",
	"kmodules":     "csm-x100kmodules",
	"kmmConfigMap": "csm-x100kmodules-configmap",
	"devicePlugin": "csm-x100-dpds",
}

var PodNamePrefixes = map[string]string{
	"firmware":     "csm-x100fw",
	"HwManager":    "csm-x100hwmgr",
	"kmodules":     "csm-x100kmodules",
	"kmmConfigMap": "csm-x100kmodules-configmap",
	"devicePlugin": "csmx100-dp",
}

const (
	FirmwareDsName     = "csm-x100-firmware-ds"
	HwManagerDsName    = "csm-x100-hwmanager-ds"
	KModulesName       = "csm-x100-kmodules-kmm"
	KModuleCMName      = "csm-x100-kmmdockerfile"
	DevicePluginDsName = "csm-x100-deviceplugin-ds"
)

const (
        x100HwMgrRunning   = "qualcomm.com/x100.hwmgrRunning"
)
var ModuleIdentifierLabelValue string = "KMODULES_NAME"

func setModuleIdentifier(value string) {
	ModuleIdentifierLabelValue = value
}

func getTrimmedSoftwareVersion(version string) string {
	return version
}

func getUniqueNameForResource(res_name string, sw_ver string) string {
	var name string
	sw_ver = getTrimmedSoftwareVersion(sw_ver)

	switch res_name {
	case FirmwareDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["firmware"], SoftwareVersionSeperator, sw_ver)

	case HwManagerDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["HwManager"], SoftwareVersionSeperator, sw_ver)

	case KModulesName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["kmodules"], SoftwareVersionSeperator, sw_ver)
		setModuleIdentifier(name)

	case KModuleCMName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["kmmConfigMap"], SoftwareVersionSeperator, sw_ver)

	case DevicePluginDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["devicePlugin"], SoftwareVersionSeperator, sw_ver)

	default:
		name = sw_ver
	}

	return name
}

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
    tname := getUniqueNameForResource(robj.GetName(), n.x100Policy.Spec.SwVersion)
    robj.ObjectMeta.Name = tname
    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Resource exists from an earlier iteration of reconcile loop")
            if err := n.rec.Update(context.TODO(), robj); err != nil {
                 logger.Info("Update of DS triggered")
            }
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
            logger.Info("Resource exists from an earlier iteration, Updates if any changes are present")
            if err := n.rec.Update(context.TODO(), robj); err != nil {
               logger.Info("Update of Module triggered")
            }
            return isPodReady("kmm.node.kubernetes.io/module.name", ModuleIdentifierLabelValue, n, "Running"), nil
        }
        logger.Info("Couldn't create", "Error", err)
        return xcardv1.NotOperational, err
    }
    return isPodReady("kmm.node.kubernetes.io/module.name", ModuleIdentifierLabelValue, n, "Running"), nil
}
func isHwMgrPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) bool {
    opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}

    list := &corev1.PodList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get PodList", err)
    }
    log.Log.Info("DEBUG: Pod", "NumberOfPods", len(list.Items))
    if len(list.Items) == 0 {
        return false
    }

    pd := list.Items[0]
    if pd.Status.Phase != phase {
        log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "!=", phase)
        return false
    }
    opts = []client.ListOption{}
    node_list := &corev1.NodeList{}
    err = n.rec.List(context.TODO(), node_list, opts...)

    for _, pd := range list.Items {
       log.Log.Info("DEBUG: isHwMgrPodReady", "Phase", pd.Status.Phase, "==", phase)
       log.Log.Info("DEBUG: isHwMgrPodReady", "on Node", pd.Spec.NodeName)
       for _, node := range node_list.Items {
	  if node.ObjectMeta.Name == pd.Spec.NodeName {
              labels := node.GetLabels()
	      labels[x100HwMgrRunning] = "true"
	      node.SetLabels(labels)
              err = n.rec.Update(context.TODO(), &node)
              if err != nil {
                 log.Log.Info("Unable to label node", node.ObjectMeta.Name, " with ",x100HwMgrRunning , err.Error())
		 return false
              }
          }
       }
    }
    return true
}

func labelHwMgrRunningState(n ControllerState, res appsv1.DaemonSet) {
    robj := res.DeepCopy()
    namespace := robj.GetNamespace()
    name := robj.GetName()
    if name != HwManagerDsName {
       return
    }
    name = getUniqueNameForResource(name, n.x100Policy.Spec.SwVersion)
    log.Log.Info("labelHwMgrRunningState ", "-",namespace, "-", name)
    isHwMgrPodReady("app", name, n, "Running")
}

func createDaemonSet(n ControllerState, res appsv1.DaemonSet) (xcardv1.State, error) {
    robj := res.DeepCopy()
    namespace := robj.GetNamespace()
    log.Log.Info("Preprocessing Daemonset to replace the fields from CR manifest")

    preProcessDaemonSet(robj, n)
    name := robj.GetName()
    logger := log.Log.WithValues("DaemonSet", name, "Namespace", namespace)

    if err := controllerutil.SetControllerReference(n.x100Policy, robj, n.rec.Scheme); err != nil {
        return xcardv1.NotOperational, err
    }

    // Create the resource from decoded manifest object
    if err := n.rec.Create(context.TODO(), robj); err != nil {
        if errors.IsAlreadyExists(err) {
            logger.Info("Found Resource")
	    if err := n.rec.Update(context.TODO(), robj); err != nil {
	       logger.Info("Update of DS triggered")
	    }
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
	labelHwMgrRunningState(n, *resource)

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
    transformations := map[string]func(*appsv1.DaemonSet, *xcardv1.X100ManagementPolicySpec, *xcardv1.X100ManagementPolicy, ControllerState) error{
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

    err := t(obj, &n.x100Policy.Spec, n.x100Policy, n)
    if err != nil {
        log.Log.Info(fmt.Sprintf("Failed to apply transformation '%s' with error: '%v'", obj.Name, err))
        os.Exit(1)
    }
}

// TransformFirmware transforms k8s-firmware daemonset with required config as per x100ManagementPolicy
func TransformFirmware(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, objmeta *xcardv1.X100ManagementPolicy, n ControllerState) error {
    dsName := getUniqueNameForResource(obj.GetName(), config.SwVersion)
    obj.ObjectMeta.Name = dsName
    log.Log.Info("TransformFirmware", "Daemonset Name :", obj.ObjectMeta.Name)
    obj.ObjectMeta.Labels["app"] = dsName
    obj.Spec.Selector.MatchLabels["app"] = dsName
    obj.Spec.Template.ObjectMeta.Labels["app"] = dsName
    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.X100Resources.Firmware.ImagePath()
    // update image pull policy
    if config.X100Resources.Firmware.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.X100Resources.Firmware.ImagePolicy(config.X100Resources.Firmware.ImagePullPolicy)
    }

    if len(config.X100Resources.Firmware.ImagePullSecrets) > 0 {
        for _, secret := range config.X100Resources.Firmware.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }
    obj.Spec.Template.Spec.NodeSelector[x100swversionKey] = config.SwVersion
    if len(config.NodeSelectors) == 0 {
       obj.Spec.Template.Spec.NodeSelector[x100swvdefaultKey] = x100swvdefaultValue
    }

    log.Log.Info("TranformFirmware: ", "Image :", obj.Spec.Template.Spec.Containers[0].Image)
    log.Log.Info("TranformFirmware: ", "Image pull policy :", obj.Spec.Template.Spec.Containers[0].ImagePullPolicy)
    log.Log.Info("TranformFirmware: ", "Image pull secret :", obj.Spec.Template.Spec.ImagePullSecrets)

    return nil
}

// TransformHWManager transforms HWManager daemonset with required config as per x100ManagementPolicy
func TransformHWManager(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, objmeta *xcardv1.X100ManagementPolicy, n ControllerState) error {

   dsName := getUniqueNameForResource(obj.GetName(), config.SwVersion)
   obj.ObjectMeta.Name = dsName
   log.Log.Info("TransformHWManager", "Daemonset Name :", obj.ObjectMeta.Name)

   obj.ObjectMeta.Labels["app"] = dsName
   obj.Spec.Selector.MatchLabels["app"] = dsName
   obj.Spec.Template.ObjectMeta.Labels["app"] = dsName

    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.X100Resources.HwManager.ImagePath()
    // update image pull policy
    if config.X100Resources.HwManager.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.X100Resources.HwManager.ImagePolicy(config.X100Resources.HwManager.ImagePullPolicy)
    }

    if len(config.X100Resources.HwManager.ImagePullSecrets) > 0 {
        for _, secret := range config.X100Resources.HwManager.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }
    obj.Spec.Template.Spec.Containers[0].Env[0].Name = "SWVERSION_FROMCRD"
    obj.Spec.Template.Spec.Containers[0].Env[0].Value = config.SwVersion

    obj.Spec.Template.Spec.NodeSelector[x100swversionKey] = config.SwVersion
    if len(config.NodeSelectors) == 0 {
       obj.Spec.Template.Spec.NodeSelector[x100swvdefaultKey] = x100swvdefaultValue
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

    kmoduleName := getUniqueNameForResource(obj.GetName(), config.SwVersion)
    obj.ObjectMeta.Name = kmoduleName
    log.Log.Info("TransformModule", "Daemonset Name :", obj.ObjectMeta.Name)

    configMapName := getUniqueNameForResource(KModuleCMName, config.SwVersion)
    obj.Spec.ModuleLoader.Container.KernelMappings[0].Build.DockerfileConfigMap.Name = configMapName


    // update image
    log.Log.Info("Transform KModule: ENTERED function")
    obj.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage = config.X100Resources.KModule.KModuleImagePath()
    obj.Spec.ModuleLoader.Container.KernelMappings[0].Build.BuildArgs[0].Value = config.X100Resources.KModule.Tag
    obj.Spec.ModuleLoader.Container.KernelMappings[0].Build.BuildArgs[1].Value = config.X100Resources.KModule.DtkAutoImage
    obj.Spec.ModuleLoader.Container.KernelMappings[0].Build.BuildArgs[2].Value = config.X100Resources.KModule.DtkSrcImage

    obj.Spec.ModuleLoader.Container.ImagePullPolicy = corev1.PullAlways

    log.Log.Info("Transform KModule: after ContainerImage")
    if len(config.X100Resources.KModule.ImageRepoSecret) > 0 {
        obj.Spec.ImageRepoSecret = &corev1.LocalObjectReference{Name: config.X100Resources.KModule.ImageRepoSecret}
    }
    obj.Spec.Selector[x100swversionKey] = config.SwVersion
    if len(config.NodeSelectors) == 0 {
       obj.Spec.Selector[x100swvdefaultKey] = x100swvdefaultValue
    }
    obj.Spec.Selector[x100HwMgrRunning] = "true"
    log.Log.Info("Transform KModule:", "Image : ", obj.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage)
    return nil
}


// TransformCsmDevicePlugin transforms DevicePlugin daemonset with required config as per x100ManagementPolicy
func TransformDevicePlugin(obj *appsv1.DaemonSet, config *xcardv1.X100ManagementPolicySpec, objmeta *xcardv1.X100ManagementPolicy, n ControllerState) error {

   //Update name and selector label values
	dsName := getUniqueNameForResource(obj.GetName(), config.SwVersion)
	obj.ObjectMeta.Name = dsName
	log.Log.Info("TransformDevicePlugin", "Daemonset Name :", obj.ObjectMeta.Name)

	obj.ObjectMeta.Labels["app"] = dsName
	obj.Spec.Selector.MatchLabels["app"] = dsName
	obj.Spec.Template.ObjectMeta.Labels["app"] = dsName

    // update image
    obj.Spec.Template.Spec.Containers[0].Image = config.X100Resources.DevicePlugin.ImagePath()
    // update image pull policy

    if config.X100Resources.DevicePlugin.ImagePullPolicy != "" {
        obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.X100Resources.DevicePlugin.ImagePolicy(config.X100Resources.DevicePlugin.ImagePullPolicy)
    }

    if len(config.X100Resources.DevicePlugin.ImagePullSecrets) > 0 {
        for _, secret := range config.X100Resources.DevicePlugin.ImagePullSecrets {
            obj.Spec.Template.Spec.ImagePullSecrets = append(obj.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
        }
    }
    obj.Spec.Template.Spec.NodeSelector[x100swversionKey] = config.SwVersion
    if len(config.NodeSelectors) == 0 {
       obj.Spec.Template.Spec.NodeSelector[x100swvdefaultKey] = x100swvdefaultValue
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
