/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
*/

package controllers

import (
    "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/client"
    xcardv1 "x100-operator/api/v1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "context"
    "strings"
    "fmt"
)

func LogAndExitOnError(logit string, err error) {
    if err != nil {
        if logit != "" {
            log.Log.Info(logit)
        }
        panic(err)
    }
}

func cleanupStaleLabels(labels map[string]string) map[string]string {
   //Delete all the labels related to x100 on Operator Clean-up(or controller exit)
   return labels
}

func attachDefaultSWLabels(labels map[string]string) map[string]string {
   labels[x100swversionKey] = "default"
   labels[x100swvdefaultKey] = "true"
   labels[x100activecrdKey] = "x100managementpolicy-default"
   return labels
}

func cleanupStaleCRDLabels(labels map[string]string) map[string]string {
    for _, label := range x100crdLabels {
        if _, ok := labels[label]; ok {
           delete(labels, label)
        }
    }
    return labels
}

func isrunningDefaultSW(labels map[string]string) bool {
    if _, ok := labels[x100swvdefaultKey]; ok {
        if labels[x100swvdefaultKey] == x100swvdefaultValue {
            // node is running with defaultSW
            return true
        }
    }
    return false
}

func isrunningNonDefaultSW(labels map[string]string) bool {
    if _, ok := labels[x100swvNondefaultKey]; ok {
        if labels[x100swvNondefaultKey] == x100swvNondefaultValue {
            // node is running with Non-defaultSW
            return true
        }
    }
    return false
}

func hasActiveCRDLabel(labels map[string]string, currentCRName string) bool {
    if _, ok := labels[x100activecrdKey]; ok {
        if labels[x100activecrdKey] == currentCRName {
            return true
        }
    }
    return false
}

func hasCustomX100Label(labels map[string]string) bool {
    if _, ok := labels[x100LabelKey]; ok {
        if labels[x100LabelKey] == x100LabelValue {
            return true
        }
    }
    return false
}

func hasX100PCILabels(labels map[string]string) bool {

    for key, val := range labels {
        if _, ok := x100NodeLabels[key]; ok {
            if x100NodeLabels[key] == val {
                log.Log.Info("Found x100PCILabels")
                return true
            }
        }
    }
    return false
}

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

func getdsLabel(dsname string) string {
    for k, v := range NamePrefixes {
        if strings.Contains(dsname, v)  {
            return PodNamePrefixes[k]
        }
    }
    return ""
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
       //log.Log.Info("DEBUG: isHwMgrPodReady", "Phase", pd.Status.Phase, "==", phase)
       //log.Log.Info("DEBUG: isHwMgrPodReady", "on Node", pd.Spec.NodeName)
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
    isHwMgrPodReady("app", getdsLabel(name), n, "Running")
}

func isModuleReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) xcardv1.State {
    opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}
    log.Log.Info("DEBUG: Pod", "LabelSelector", fmt.Sprintf("%s=%s", labelkey, labelvalue))
    podlist := &corev1.PodList{}
    err := n.rec.List(context.TODO(), podlist, opts...)
    if err != nil {
        log.Log.Info("Could not get PodList", err)
    }
    log.Log.Info("DEBUG: Pod", "NumberOfPods", len(podlist.Items))
    //get number of nodes with currentCR as activeCR
    currentCR := n.x100Policy.ObjectMeta.Name
    opts = []client.ListOption{}
    node_list := &corev1.NodeList{}
    err = n.rec.List(context.TODO(), node_list, opts...)
    countofnodescurrentCR := 0
    for _, node := range node_list.Items {
        labels := node.GetLabels()
	if _, ok := labels[x100activecrdKey]; ok {
            if labels[x100activecrdKey] == currentCR {
                countofnodescurrentCR++
            }
	}
    }

    if len(podlist.Items) != countofnodescurrentCR {
       return xcardv1.NotOperational
    }
    if len(podlist.Items) == 0 && countofnodescurrentCR == 0 {
        return xcardv1.Operational
    }
    if len(podlist.Items) == countofnodescurrentCR {
        for _, pd := range podlist.Items {
            if pd.Status.Phase != phase {
                return xcardv1.NotOperational
            }
        }
    }
    return xcardv1.Operational
}
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
