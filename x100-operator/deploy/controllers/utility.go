/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
*/

package controllers

import (
    kmmv1 "github.com/kubernetes-sigs/kernel-module-management/api/v1beta1"
    kerrors "k8s.io/apimachinery/pkg/api/errors"
    "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "k8s.io/client-go/tools/remotecommand"
    "k8s.io/apimachinery/pkg/runtime"
    xcardv1 "x100-operator/api/v1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "context"
    "strings"
    "slices"
    "regexp"
    "errors"
    "bytes"
    "time"
    "fmt"
    "os"
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
    for _, label := range x100crdLabels {
        if _, ok := labels[label]; ok {
            delete(labels, label)
        }
    }
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

func hasPriorCrLabel(labels map[string]string) (string, bool) {
    if _, ok := labels[x100PriorCr]; ok {
       return labels[x100PriorCr], true
    }
    return "", false
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

func isRunningwithcurrCR(labels map[string]string, currentCRName string) bool {
    if _, ok := labels[x100activecrdKey]; ok {
        if labels[x100activecrdKey] == currentCRName {
            return true
        }
    }
    return false
}

func getActiveCRonNode(labels map[string]string) (string, error) {
    if _, ok := labels[x100activecrdKey]; ok {
        return labels[x100activecrdKey], nil
    }
    return "", fmt.Errorf("Active CR label not found on current Node")
}

func hasHwmgrRunningLabel(labels map[string]string) bool {
    if _, ok := labels[x100HwMgrRunning]; ok {
        if labels[x100HwMgrRunning] == "true" {
            return true
        }
    }
    return false
}

func hasX100BootupSuccessLabel(labels map[string]string) bool {
    if _, ok := labels[x100BootupSuccess]; ok {
        if labels[x100BootupSuccess] == "true" {
            return true
        }
    }
    return false
}

func isX100BootupStatusMarkedLabelTrue(labels map[string]string) bool {
    if _, ok := labels[x100BootupStatusMarked]; ok {
        if labels[x100BootupStatusMarked] == "true" {
            return true
        }
    }
    return false
}

func hasX100BootupStatusMarkedLabel(labels map[string]string) bool {
    if _, ok := labels[x100BootupStatusMarked]; ok {
        return true
    }
    return false
}

func isx100UpgradingLabelTrue(labels map[string]string) bool {
    if _, ok := labels[x100Upgrading]; ok {
        if labels[x100Upgrading] == "true" {
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

func hasX100IsolateLabel(labels map[string]string) bool {
	if _, ok := labels[x100IsolateKey]; ok {
		log.Log.Info("Isolate label present")
		return true
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

func (c *ControllerState) markHealthCheckedforCurrentCR() {
    currentCR := c.x100Policy.ObjectMeta.Name
    opts := []client.ListOption{}
    node_list := &corev1.NodeList{}
    err := c.rec.List(context.TODO(), node_list, opts...)
    for _, node := range node_list.Items {
        labels := node.GetLabels()
        if isRunningwithcurrCR(labels, currentCR) {
            if _, ok := labels[x100BootupSuccess]; ok {
                labels[x100BootupStatusMarked] = "true"
            }
            hasX100BootupSuccess := hasX100BootupSuccessLabel(labels)
            hasx100Upgrading := isx100UpgradingLabelTrue(labels)
            delete(labels, x100Upgrading)
            if hasx100Upgrading && !hasX100BootupSuccess {
                labels[x100UpgradeFailed] = labels[x100swversionKey]
            }
            node.SetLabels(labels)
            err = c.rec.Update(context.TODO(), &node)
            if err != nil {
                log.Log.Info("Unable to label node", node.ObjectMeta.Name, " with ",x100BootupStatusMarked , err.Error())
            }
        }
    }
}

func isHwMgrPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) bool {

    opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}
    list := &corev1.PodList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get PodList", err)
    }
    if len(list.Items) == 0 {
        return false
    }

    opts = []client.ListOption{}
    node_list := &corev1.NodeList{}
    err = n.rec.List(context.TODO(), node_list, opts...)

    for _, pd := range list.Items {
       for _, node := range node_list.Items {
          if node.ObjectMeta.Name == pd.Spec.NodeName &&  pd.Status.Phase == phase{
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

func (c *ControllerState) getKmmPodOnNode(node *corev1.Node, podNamePrefix string) (corev1.Pod, error) {
    returnPod := corev1.Pod{}
    nodeName := node.GetName()

    opts := []client.ListOption{ client.MatchingFields{"spec.nodeName": nodeName},
    }

    list := &corev1.PodList{}

    err := c.rec.List(context.TODO(), list, opts...)
    if err != nil {
        if kerrors.IsNotFound(err) {
            log.Log.Info(fmt.Sprintf("Pod %s has already been deleted on node %v", podNamePrefix, nodeName))
        }
        log.Log.Info(fmt.Sprintf("Unable to retrieve pod on node %s, ERR: %v", nodeName, err))
        return returnPod, err
    }
    if len(list.Items) == 0 {
        return returnPod, errors.New(fmt.Sprintf("Pods not available on node %s", nodeName))
    }
    for _, pod := range list.Items {
        podName := pod.GetName()
        if strings.Contains(podName, podNamePrefix) {
            returnPod = pod
            return returnPod, nil
        }
    }
    return returnPod, errors.New(fmt.Sprintf("Pod with prefix %s is not available on node %s", podNamePrefix, nodeName))
}

func hasKmmReadylabel(labels map[string]string) bool {
    for k, _ := range labels {
        if strings.Contains(k, "kmm.node.kubernetes.io/x100-operator-resources.") {
            return true
        }
    }
    return false
}

func (c *ControllerState) waitForKmmPodTermination(node *corev1.Node) {

    for {
        nopts := []client.ListOption{ client.MatchingFields{"metadata.name": node.ObjectMeta.Name }}
        tempnodes := &corev1.NodeList{}
        err := c.rec.List(context.TODO(), tempnodes, nopts...)
        if err != nil {
            log.Log.Info("Could not get Latest Node", err,"--")
        }
        tempnode := tempnodes.Items[0]
        labels := tempnode.GetLabels()
        isKmmReady := hasKmmReadylabel(labels)
        if !isKmmReady {
            break
        }
        log.Log.Info(fmt.Sprintf("Still running KMM rmmod worker pods on %v", node.GetName()))
        time.Sleep(3 * time.Second)
    }
    log.Log.Info(fmt.Sprintf("Successfully drained all Modules from %v, Returning", node.GetName()))
    return
}

func (c *ControllerState) waitForFwPodTermination(node *corev1.Node) {

    for {
        pod, err := c.getNamedPodOnNode(node, PodNamePrefixes["firmware"])
        if err != nil {
            break
        }
        if pod.DeletionTimestamp == nil {
           log.Log.Info(fmt.Sprintf("Firmware pod Deletion time is null on node %s ?", node.GetName()))
        }
        log.Log.Info(fmt.Sprintf("Still terminating %v pods on %v", pod.GetName(), node.GetName()))
        time.Sleep(3 * time.Second)
    }
    log.Log.Info(fmt.Sprintf("Successfully drained all Fw pods from %v, Returning", node.GetName()))
    return
}

func (c *ControllerState) waitForHwPodTermination(node *corev1.Node) {
    for {
        pod, err := c.getNamedPodOnNode(node, PodNamePrefixes["HwManager"])
        if err != nil {
            break
        }
        if pod.DeletionTimestamp == nil {
           log.Log.Info(fmt.Sprintf("HwManager pod Deletion time is null on node %s ?", node.GetName()))
        }
        log.Log.Info(fmt.Sprintf("Still terminating %v pods on %v", pod.GetName(), node.GetName()))
        time.Sleep(3 * time.Second)
    }
    log.Log.Info(fmt.Sprintf("Successfully drained HwMgr pods from %v, Returning", node.GetName()))
    return
}

func isModuleReady(moduleName string, n ControllerState) xcardv1.State {
    log.Log.Info("Module Object query","Name: ",moduleName)
    opts := []client.ListOption{client.MatchingFields{"metadata.name": moduleName}}
    moduleList := &kmmv1.ModuleList{}
    err := n.rec.List(context.TODO(), moduleList, opts...)
    if err != nil {
        log.Log.Info("Could not get ModuleList", err)
        return xcardv1.NotOperational
    }
    log.Log.Info("ModuleList", "length", len(moduleList.Items))
    if len(moduleList.Items) == 0 {
        return xcardv1.NotOperational
    }
    //get number of nodes with currentCR as activeCR
    module := kmmv1.Module{}
    if len(moduleList.Items) > 0 {
       for _, md := range moduleList.Items {
            if md.ObjectMeta.Name == moduleName {
                module = md
                log.Log.Info("ModuleList Fetched the current Module")
                break
            }
        }
    }
    log.Log.Info("Module Object Values","[module.Status.ModuleLoader.AvailableNumber]: ", module.Status.ModuleLoader.AvailableNumber)
    log.Log.Info("Module Object Values", "[module.Status.ModuleLoader.DesiredNumber]: ", module.Status.ModuleLoader.DesiredNumber)
    currentCR := n.x100Policy.ObjectMeta.Name
    opts = []client.ListOption{}
    node_list := &corev1.NodeList{}
    err = n.rec.List(context.TODO(), node_list, opts...)
    var countofnodescurrentCR int32 = 0
    for _, node := range node_list.Items {
        labels := node.GetLabels()
        if _, ok := labels[x100activecrdKey]; ok {
            if labels[x100activecrdKey] == currentCR {
                countofnodescurrentCR++
            }
        }
    }
    if module.Status.ModuleLoader.AvailableNumber != countofnodescurrentCR {
       return xcardv1.NotOperational
    }
    if module.Status.ModuleLoader.AvailableNumber == 0 && countofnodescurrentCR == 0 {
        return xcardv1.Operational
    }
    return xcardv1.Operational
}

func isPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) xcardv1.State {
    opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}

    list := &corev1.PodList{}
    err := n.rec.List(context.TODO(), list, opts...)
    if err != nil {
        log.Log.Info("Could not get PodList", err)
    }
    if len(list.Items) == 0 {
        return xcardv1.NotOperational
    }

    for _, pd := range list.Items {
        if pd.Status.Phase != phase {
            log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "!=", phase)
            return xcardv1.NotOperational
        }
    }
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

func (c *ControllerState) runCommandOnPod(podName string, containerName string, command string) ([]byte, error) {
    /***
        Execute supplied command on given pod/contianer
        Return the output of the command to caller
    ***/
    req := c.rec.RESTClient.
                 Post().
                 Namespace(assetsNamespace).
                 Resource("pods").
                 Name(podName).
                 SubResource("exec").
                 VersionedParams(&corev1.PodExecOptions{
                    Container: containerName,
                    Command:   []string{"/bin/bash", "-c", command},
                    Stdin:     true,
                    Stdout:    true,
                    Stderr:    true,
                 },
                 runtime.NewParameterCodec(c.rec.Scheme))
    exec, err := remotecommand.NewSPDYExecutor(c.rec.RESTConfig, "POST", req.URL())
    if err != nil {
        return nil, fmt.Errorf("error while creating remote command executor: %v", err)
    }

    var stdout, stderr bytes.Buffer
    err = exec.Stream(remotecommand.StreamOptions{
               Stdin:  os.Stdin,
               Stdout: &stdout,
               Stderr: &stderr,
               Tty:    false,
          })
    if err != nil {
        return []byte{}, err
    }
    //log.Log.Info(fmt.Sprintf("Command (%s) result on pod %s: %s", command, podName, string(stdout.Bytes())))
    return stdout.Bytes(), nil
}

func (c *ControllerState) getNamedPodOnNode(node *corev1.Node, podNamePrefix string) (corev1.Pod, error) {

    returnPod := corev1.Pod{}
    nodeName := node.GetName()

    opts := []client.ListOption{ client.MatchingFields{"spec.nodeName": nodeName},
    }

    list := &corev1.PodList{}

    err := c.rec.List(context.TODO(), list, opts...)
    if err != nil {
        if kerrors.IsNotFound(err) {
            log.Log.Info(fmt.Sprintf("Pod %s has already been deleted on node %v", podNamePrefix, nodeName))
        }
        log.Log.Info(fmt.Sprintf("Unable to retrieve pod on node %s, ERR: %v", nodeName, err))
        return returnPod, err
    }
    if len(list.Items) == 0 {
        return returnPod, errors.New(fmt.Sprintf("Pods not available on node %s", nodeName))
    }
    for _, pod := range list.Items {
        podName := pod.GetName()
        if strings.Contains(podName, podNamePrefix) {
            returnPod = pod
            return returnPod, nil
        }
    }
    return returnPod, errors.New(fmt.Sprintf("Pod with prefix %s is not available on node %s", podNamePrefix, nodeName))
}

func (c *ControllerState) getX100CardCountOnNode(node *corev1.Node) (int, error) {
    count := 0
    pod := corev1.Pod{}
    pod, err := c.getNamedPodOnNode(node, PodNamePrefixes["HwManager"])
    if err != nil {
        log.Log.Info(fmt.Sprintf("HwManager pod on node %s not running", node.GetName()))
        return count, err
    }

    command := "lspci |grep Qualcomm |rev |cut -d: -f3 |rev | uniq"
    output, err := c.runCommandOnPod(pod.GetName(), pod.Spec.Containers[0].Name, command)
    if err != nil {
        log.Log.Info(fmt.Sprintf("Failed to fetch the x100 cards count on node %s", node.GetName()))
        return count, err
    }

    count = len(strings.Split(string(output), "\n"))
    count--
    log.Log.Info(fmt.Sprintf("X100 Card count from HwMgr pod on node %s is %v", node.GetName(), count))
    return count, nil
}

func (c *ControllerState) readHealthLogFromHwMgrPod(pod *corev1.Pod) ([]byte, error) {

    containerName := pod.Spec.Containers[0].Name
    podName := pod.GetName()
    command := fmt.Sprintf("cat %s", healthLogPath)
    //log.Log.Info(fmt.Sprintf("Reading the health log from %s pod - %s container", podName, containerName))
    output,err := c.runCommandOnPod(podName, containerName, command)
    if err != nil {
       log.Log.Info(fmt.Sprintf("Failed to read healthlog from pod %s, \n ERR:%v", podName,err))
       return output, err
    }
    log.Log.Info(fmt.Sprintf("Fetched health log from pod %s,\nLog:%v", podName, string(output)))
    return output, nil
}

func (c *ControllerState) getX100BootupStatusOnNode(node *corev1.Node, x100count int) (int, int, error) {
    x100fetchedcount := 0
    x100bootupcount := 0
    x100failedbootupcount := 0
    pod := corev1.Pod{}
    pod, err := c.getNamedPodOnNode(node, PodNamePrefixes["HwManager"])
    if err != nil {
        log.Log.Info(fmt.Sprintf(" getX100BootupStatusOnNode: HwManager pod on node %s not running", node.GetName()))
        return x100bootupcount, x100failedbootupcount, err
    }
    output,err := c.readHealthLogFromHwMgrPod(&pod)
    //Compare the strings with success cases
    if err != nil {
        log.Log.Info(fmt.Sprintf("getX100BootupStatusOnNode: Failed to fetch the Health Status on node %s", node.GetName()))
        return x100bootupcount, x100failedbootupcount, err
    }
    res_lines := strings.Split(string(output), "\n")
    for _, line := range res_lines {
        re := regexp.MustCompile(`Device boot (success|failed) for (\w+)`)
        match := re.FindStringSubmatch(line)
        if len(match) >= 3 {
            deviceStatus := match[1]
            deviceChannel := match[2]
            x100fetchedcount += 1
            log.Log.Info(fmt.Sprintf("Bootup %s on channel %s", deviceStatus, deviceChannel))
            if deviceStatus == "failed" {
                x100failedbootupcount += 1
            } else if deviceStatus == "success" {
                x100bootupcount += 1
            }
        }
    }

    if x100fetchedcount != x100count {
        // Missing health status for some cards
        log.Log.Info(fmt.Sprintf("Node %s has missing health status for one or more cards", node.GetName()))
        return x100bootupcount, x100failedbootupcount, errors.New("Node %s has missing health status for one or more cards")
    }
    return x100bootupcount, x100failedbootupcount, nil
}

func (c *ControllerState) removeInvalidNodesfromCR(policy *xcardv1.X100ManagementPolicy) error {
    log.Log.Info("---removeInvalidNodesfromCR---")
    opts := []client.ListOption{}
    nlist := &corev1.NodeList{}
    err := c.rec.List(context.TODO(), nlist, opts...)
    if err != nil {
        return fmt.Errorf("Unable to list nodes, err %s", err.Error())
    }
    nlist_names := []string{}
    for _, node := range nlist.Items {
        nlist_names = append(nlist_names, node.ObjectMeta.Name)
    }
    cropts := []client.ListOption{}
    crlist := &xcardv1.X100ManagementPolicyList{}
    nodeListfromcr := policy.Spec.NodeSelector
    if len(nodeListfromcr) == 0 && policy.Spec.SwVersion == "default" {
        return nil
    } else {
        for _, ns := range nodeListfromcr {
            if slices.Contains(nlist_names, ns) {
                continue
            }
            err  := c.rec.List(context.TODO(), crlist, cropts...)
            if err != nil {
               log.Log.Error(err, "removeInvalidNodesfromCR()- Unable to list X100ManagementPolicy CRs")
            }
            for _, policyItr := range crlist.Items {
                if policyItr.ObjectMeta.GetName() == policy.ObjectMeta.GetName() {
                    var ind int
                    for i, pnode := range policyItr.Spec.NodeSelector {
                        if pnode == ns {
                            ind = i
                            break
                        }
                    }
                    policyItr.Spec.NodeSelector = append(policyItr.Spec.NodeSelector[:ind], policyItr.Spec.NodeSelector[ind+1:]...)
                    c.rec.Update(context.TODO(), &policyItr)
                }
            }
        }
        return nil
    }
}
