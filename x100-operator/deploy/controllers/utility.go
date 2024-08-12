/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
*/

package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	kmmv1 "github.com/kubernetes-sigs/kernel-module-management/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"time"
	xcardv1 "x100-operator/api/v1"
)

func LogAndExitOnError(logit string, err error) {
	if err != nil {
		if logit != "" {
			log.Log.Info(logit)
		}
		panic(err)
	}
}

func cleanupStaleCRLabels(labels map[string]string) map[string]string {
	for _, label := range x100CrdLabels {
		if _, ok := labels[label]; ok {
			delete(labels, label)
		}
	}
	return labels
}

func cleanupStaleSelectorLabels(labels map[string]string) map[string]string {
	for _, label := range x100SelectorLabels {
		if _, ok := labels[label]; ok {
			delete(labels, label)
		}
	}
	return labels
}

func hasX100AggregationBlockedLabel(labels map[string]string) bool {
	if _, ok := labels[x100NodeAggregationBlocked]; ok {
		if labels[x100NodeAggregationBlocked] == "true" {
			return true
		}
	}
	return false
}

func getActiveCRonNode(labels map[string]string) (string, error) {
	if _, ok := labels[x100ActiveCR]; ok {
		return labels[x100ActiveCR], nil
	}
	return "", fmt.Errorf("Active CR label not found on current Node")
}

func hasActiveCRLabel(labels map[string]string) bool {
	if _, ok := labels[x100ActiveCR]; ok {
		if labels[x100ActiveCR] != "" {
			return true
		}
	}
	return false
}

func hasPriorCRLabel(labels map[string]string) (string, bool) {
	if _, ok := labels[x100PriorCR]; ok {
		if labels[x100PriorCR] != "" {
			return labels[x100PriorCR], true
		}
	}
	return "", false
}

func hasX100OwnerPolicyDeletedLabel(labels map[string]string) bool {
	if _, ok := labels[x100OwnerPolicyDeleted]; ok {
		if labels[x100OwnerPolicyDeleted] == "true" {
			return true
		}
	}
	return false
}

func hasX100TeardownCompletedLabel(labels map[string]string) bool {
	if _, ok := labels[x100TeardownCompleted]; ok {
		if labels[x100TeardownCompleted] == "true" {
			return true
		}
	}
	return false
}

func hasX100UpgradeFailedLabel(labels map[string]string) bool {
	if _, ok := labels[x100UpgradeFailed]; ok {
		if labels[x100UpgradeFailed] != "" {
			return true
		}
	}
	return false
}

func hasX100UpgradingLabel(labels map[string]string) bool {
	if _, ok := labels[x100Upgrading]; ok {
		if labels[x100Upgrading] == "true" {
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

func hasX100BootupFailedLabel(labels map[string]string) bool {
	if _, ok := labels[x100BootupSuccess]; ok {
		if labels[x100BootupSuccess] == "false" {
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

func hasCustomX100Label(labels map[string]string) bool {
	if _, ok := labels[x100LabelKey]; ok {
		if labels[x100LabelKey] == x100LabelValue {
			return true
		}
	}
	return false
}

func hasX100PCILabel(labels map[string]string) bool {
	for key, val := range labels {
		if _, ok := x100NodeLabels[key]; ok {
			if x100NodeLabels[key] == val {
				return true
			}
		}
	}
	return false
}

func hasX100PCILabels(labels map[string]string) bool {
	result := hasX100PCILabel(labels)
	if !result && hasCustomX100Label(labels) {
		time.Sleep(5 * time.Second)
	} else {
		return result
	}
	return hasX100PCILabel(labels)
}

func hasKmmReadylabel(labels map[string]string) bool {
	for k, _ := range labels {
		if strings.Contains(k, kmmReadyLabel) {
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

func isX100RunningWithPolicy(labels map[string]string, policyName string) bool {
	if _, ok := labels[x100ActiveCR]; ok {
		if labels[x100ActiveCR] == policyName {
			return true
		}
	}
	return false
}

func getTrimmedSoftwareVersion(version string) string {
	return version
}

func getUniqueNameForResource(res_name string, sw_ver string) string {
	var name string
	sw_ver = getTrimmedSoftwareVersion(sw_ver)

	log.Log.Info(fmt.Sprintf("getUniqueNameForResource received (name - %s, sw version - %s)", res_name, sw_ver))

	switch res_name {
	case FirmwareDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["firmware"], SoftwareVersionSeperator, sw_ver)

	case HwManagerDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["HwManager"], SoftwareVersionSeperator, sw_ver)

	case KModulesName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["kmodules"], SoftwareVersionSeperator, sw_ver)
		//setModuleIdentifier(name)

	case KModuleCMName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["kmmConfigMap"], SoftwareVersionSeperator, sw_ver)

	case DevicePluginDsName:
		name = fmt.Sprintf("%s%s%s", NamePrefixes["devicePlugin"], SoftwareVersionSeperator, sw_ver)

	default:
		name = sw_ver
	}

	return name
}

func getDSLabel(dsname string) string {
	for k, v := range NamePrefixes {
		if strings.Contains(dsname, v) {
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

func setEnvVariablesForHwManagerPod(c *corev1.Container, config *xcardv1.X100ManagementPolicySpec) {
	// Capture values for environment variable SWVERSION_FROMCRD
	key := "SWVERSION_FROMCRD"
	sv := config.SwVersion
	fv := config.X100Resources.Firmware.Version
	hv := config.X100Resources.HwManager.Version
	ev := config.X100Resources.HwManager.ExpVersion
	kv := config.X100Resources.KModule.Version
	dv := config.X100Resources.DevicePlugin.Version
	value := fmt.Sprintf("%s:{Firmware:%s HwManager:%s Exporter:%s KModules:%s DevicePlugin:%s}",
		sv, fv, hv, ev, kv, dv)
	setContainerEnv(c, key, value)

	// Capture values for environment variable VfCount
	if config.VfCount > 0 {
		key = "VFCOUNT"
		value = strconv.Itoa(config.VfCount)
	}
	setContainerEnv(c, key, value)
}

func getX100EnabledNodesCount(n ControllerState) (int, *corev1.NodeList) {
	/***
		Return the node count for nodes that have
		x100 card attached and are under the current
		policies selector list

	***/
	count := 0
	nodeList := &corev1.NodeList{}
	policy := n.x100Policy

	nodesInCRSelectorList := policy.Spec.NodeSelector

	opts := []client.ListOption{}
	list := &corev1.NodeList{}

	err := n.rec.List(context.TODO(), list, opts...)
	if err != nil {
		log.Log.Info("Could not get node list of x100 enabled nodes", err)
		return -1, nodeList
	}

	for _, ns := range nodesInCRSelectorList {
		for _, node := range list.Items {
			if node.ObjectMeta.Name == ns {
				labels := node.GetLabels()
				if hasCustomX100Label(labels) {
					count += 1
					nodeList.Items = append(nodeList.Items, node)
				}
			}
		}
	}
	return count, nodeList
}

func getPodNamePrefixFromIndex(index int) string {
	var name string

	switch index {
	case 0:
		name = PodNamePrefixes["firmware"]
	case 1:
		name = PodNamePrefixes["HwManager"]
	case 2:
		name = PodNamePrefixes["kmodules"]
	case 3:
		name = PodNamePrefixes["devicePlugin"]
	default:
		name = ""
	}
	return name
}

func (c *ControllerState) createNFDResources() error {
	/***
		NFD resources need to be available beforehand
		Since NFD attaches labels to the nodes that
		labelX100NodeswithCR() then processes
	***/
	log.Log.Info("Creating NFD resources to label nodes with appropriate pci labels")
	_ = createAssetMap(nfdResourcePath)

	// Create the resources using  resources from AssetMap and creation callbacks
	asset := AssetMap[nfdResourcePath]

	for _, robj := range asset.objectMappings {
		_, err := createKindResource(*c, robj.key, robj.value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ControllerState) getPodCompletionStatus(node *corev1.Node, podNamePrefix string, completionPath string) (bool, error) {
	status := false
	log.Log.Info(fmt.Sprintf("Getting completion status for %s from %s", podNamePrefix, completionPath))
	pod, err := c.getNamedPodOnNode(node, podNamePrefix)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Failed to fetch firmware pod %s on node %s",
			pod.GetName(), node.GetName()))
		return status, err
	}
	command := fmt.Sprintf("cat %s", completionPath)
	log.Log.Info(fmt.Sprintf("Running command (%s) on pod %s", command, pod.GetName()))

	if len(pod.Spec.Containers) == 0 {
		//If previous pod has not even been created, like in case of fw pod
		return status, nil
	}

	output, err := c.runCommandOnPod(pod.GetName(), pod.Spec.Containers[0].Name, command)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Failed to read fw pod completion status on node %s", node.GetName()))
		return status, err
	}

	out := string(output)
	//log.Log.Info(fmt.Sprintf("Completion status as read from firmware pod on node %s is %s", node.GetName(), out))
	if strings.Contains(out, "completed") {
		log.Log.Info(fmt.Sprintf("Completion status as read from firmware pod on node %s is %s", node.GetName(), out))
		return true, nil
	}
	return status, nil
}
func isModuleReady(moduleName string, n ControllerState) xcardv1.State {
	log.Log.Info(fmt.Sprintf("Checking isModuleReady for %s module", moduleName))

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

	log.Log.Info("Module Object Values", "[module.Status.ModuleLoader.AvailableNumber]: ",
		module.Status.ModuleLoader.AvailableNumber)
	log.Log.Info("Module Object Values", "[module.Status.ModuleLoader.DesiredNumber]: ",
		module.Status.ModuleLoader.DesiredNumber)

	opts = []client.ListOption{}
	node_list := &corev1.NodeList{}
	err = n.rec.List(context.TODO(), node_list, opts...)
	if err != nil {
		log.Log.Info("Could not get nodelist", err)
		return xcardv1.NotOperational
	}

	var nodesUnderCurrentPolicy int32 = 0

	nodesInCRSelectorList := n.x100Policy.Spec.NodeSelector
	for _, ns := range nodesInCRSelectorList {
		for _, node := range node_list.Items {
			if node.ObjectMeta.Name == ns {
				labels := node.GetLabels()
				if hasKmmReadylabel(labels) {
					nodesUnderCurrentPolicy += 1
				}
			}
		}
	}

	if module.Status.ModuleLoader.AvailableNumber != nodesUnderCurrentPolicy {
		return xcardv1.NotOperational
	}
	if module.Status.ModuleLoader.AvailableNumber == 0 && nodesUnderCurrentPolicy == 0 {
		return xcardv1.Operational
	}
	return xcardv1.Operational
}

func isPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) xcardv1.State {
	nodelist := &corev1.NodeList{}
	list := &corev1.PodList{}
	podsUnderCurrentCR := &corev1.PodList{}

	opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}

	err := n.rec.List(context.TODO(), list, opts...)
	if err != nil {
		log.Log.Info("Could not get PodList", err)
	}

	/***
		We can check for pods bring up on all eligible nodes under currentCR
		Match the pods with label selector against nodes with a particular running version
		TBD func areDsOwnedPodsReady(selector string, version string)
	***/
	log.Log.Info(fmt.Sprintf("%v - %s pods in the cluster", len(list.Items), labelvalue))

	if len(list.Items) == 0 {
		return xcardv1.NotOperational
	}

	x100NodeCount, nodelist := getX100EnabledNodesCount(n)
	if x100NodeCount == -1 {
		return xcardv1.NotOperational
	}

	x100PodCount := 0
	for _, pod := range list.Items {
		for _, node := range nodelist.Items {
			if pod.Spec.NodeName == node.ObjectMeta.Name {
				x100PodCount += 1
				podsUnderCurrentCR.Items = append(podsUnderCurrentCR.Items, pod)
			}
		}
	}
	log.Log.Info(fmt.Sprintf("%v - %s pods under policy %s",
		len(list.Items), labelvalue, n.x100Policy.ObjectMeta.Name))
	log.Log.Info(fmt.Sprintf("Need %v - %s pods under policy %s",
		x100NodeCount, labelvalue, n.x100Policy.ObjectMeta.Name))

	if x100PodCount < x100NodeCount {
		return xcardv1.NotOperational
	}

	//if len(list.Items) < x100NodeCount {
	// Each x100 enabled node should have a pod
	//	return xcardv1.NotOperational
	//}

	for _, pd := range podsUnderCurrentCR.Items {
		if pd.Status.Phase != phase {
			log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "!=", phase)
			return xcardv1.NotOperational
		}
	}
	return xcardv1.Operational
}

func isDeploymentReady(name string, n ControllerState) xcardv1.State {
	opts := []client.ListOption{client.MatchingLabels{"app": name}}

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
	opts := []client.ListOption{client.MatchingLabels{"app": name}}

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

func (c *ControllerState) getKmmPodOnNode(node *corev1.Node, podNamePrefix string) (corev1.Pod, error) {
	returnPod := corev1.Pod{}
	nodeName := node.GetName()

	opts := []client.ListOption{client.MatchingFields{"spec.nodeName": nodeName}}

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

func (c *ControllerState) markHealthCheckedforCurrentCR() {
	currentCR := c.x100Policy.ObjectMeta.Name
	opts := []client.ListOption{}
	node_list := &corev1.NodeList{}
	err := c.rec.List(context.TODO(), node_list, opts...)
	for _, node := range node_list.Items {
		labels := node.GetLabels()
		if isX100RunningWithPolicy(labels, currentCR) {
			if _, ok := labels[x100BootupSuccess]; ok {
				labels[x100BootupStatusMarked] = "true"
				if labels[x100BootupSuccess] != "true" {
					labels[x100BootupSuccess] = "false"
				}
			}
			hasX100BootupSuccess := hasX100BootupSuccessLabel(labels)
			hasx100Upgrading := hasX100UpgradingLabel(labels)
			hasx100TeardownCompleted := hasX100TeardownCompletedLabel(labels)
			if hasx100Upgrading {
				delete(labels, x100Upgrading)
			}
			if hasx100TeardownCompleted {
				delete(labels, x100TeardownCompleted)
			}

			if hasx100Upgrading && !hasX100BootupSuccess {
				labels[x100UpgradeFailed] = labels[x100SwVersion]
			}
			node.SetLabels(labels)
			err = c.rec.Update(context.TODO(), &node)
			if err != nil {
				log.Log.Info("Unable to label node", node.ObjectMeta.Name, " with ", x100BootupStatusMarked, err.Error())
			}
		}
	}
}

func (c *ControllerState) runUpgradeSequenceForNode(node *corev1.Node) error {
	// Transition the node policy state to upgrading

	err := c.transitionToUpgradingState(node)
	if err != nil {
		return err
	}
	log.Log.Info(fmt.Sprintf("Node %s transitioned to upgrading state", node.ObjectMeta.Name))

	// Wait for existing pods to terminate
	labels := node.GetLabels()
	if !hasX100TeardownCompletedLabel(labels) {
		err = c.teardownX100ManagementPolicyOwnedPodsOnNode(node)
		if err != nil {
			return err
		}
	}
	log.Log.Info(fmt.Sprintf("Node %s completed tearing down existing state", node.ObjectMeta.Name))

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	//Update the sw version related labels
	err = c.updateSwVersionLabels(node)
	if err != nil {
		return err
	}
	log.Log.Info(fmt.Sprintf("Node %s updated SwVersion labels", node.ObjectMeta.Name))

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	// Force pod bringup sequence here
	err = c.enablePodSelectorLabels(node)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Unable to label node %s with pod selector labels, err %s", node.GetName(), err.Error()))
		//return err
		return nil
	}
	/***
	labels = node.GetLabels()
	delete(labels, x100EnablingFirstPolicy)
	node.SetLabels(labels)
	err = c.rec.Update(context.TODO(), node)
	if err != nil {
		return fmt.Errorf("Unable to delete label node %s with %s, err %s",
			node.ObjectMeta.Name, x100EnablingFirstPolicy, err.Error())
	}
	***/
	log.Log.Info(fmt.Sprintf("Enabled pod selector labels for node %s", node.GetName()))
	return nil
}

func (c *ControllerState) transitionToUpgradingState(node *corev1.Node) error {
	labels := node.GetLabels()

	if !hasX100UpgradingLabel(labels) {
		//Remove stale labels if any
		for _, label := range x100StatusLabels {
			if _, ok := labels[label]; ok {
				delete(labels, label)
			}
		}
		labels[x100Upgrading] = "true"

		node.SetLabels(labels)
		err := c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name,
				x100Upgrading, err.Error())
		}
	}
	return nil
}

func (c *ControllerState) teardownX100ManagementPolicyOwnedPodsOnNode(node *corev1.Node) error {
	// Maintain a sequence to delete the pods
	// Preferably observe status of current resource deletion before sequencing further
	// FirmwareDsSelectorLabelKey,HwMgrDsSelectorLabelKey,KModuleSelectorLabelKey
	log.Log.Info(fmt.Sprintf(" -----teardownX100ManagementPolicyOwnedPods Entered-----"))

	firmwarePodName := ""
	hwManagerPodName := ""
	kModulePodName := ""
	kmmConfigMapName := ""
	devicePluginPodName := ""

	nodeName := node.GetName()

	// Filter all pods running on given node and
	// Match names for pods of interest to x100ManagementPolicy
	opts := []client.ListOption{
		client.MatchingFields{"spec.nodeName": nodeName},
	}

	list := &corev1.PodList{}

	err := c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		if kerrors.IsNotFound(err) {
			log.Log.Info(fmt.Sprintf("Node %s not found during teardown sequence", nodeName))
			return nil
		}
		return err
	}
	count := 0
	for _, pod := range list.Items {
		podName := pod.GetName()
		if strings.Contains(podName, NamePrefixes["firmware"]) {
			firmwarePodName = podName
			count += 1
		} else if strings.Contains(podName, NamePrefixes["HwManager"]) {
			hwManagerPodName = podName
			count += 1
		} else if strings.Contains(podName, NamePrefixes["kmodules"]) {
			kModulePodName = podName
			count += 1
		} else if strings.Contains(podName, NamePrefixes["kmmConfigMap"]) {
			kmmConfigMapName = podName
		} else if strings.Contains(podName, NamePrefixes["devicePlugin"]) {
			devicePluginPodName = podName
			count += 1
		}
	}

	if count == 0 {
		// All pods have already been terminated
		log.Log.Info("All pods have been terminated successfully")
		return nil
	}

	log.Log.Info(
		fmt.Sprintf("Pods found for deletion during teardown -> \nFirmware Pod: %s\nHW Manager Pod: %s\nModule: %s\nDevicePlugin Pod: %s\n",
			firmwarePodName, hwManagerPodName, kModulePodName, devicePluginPodName))

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	/*** Device Plugin Pod Deletion ***/
	labels := node.GetLabels()
	if _, ok := labels[DevicePluginSelectorLabelKey]; ok {
		delete(labels, DevicePluginSelectorLabelKey)
		node.SetLabels(labels)
		err = c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to remove label %s from node %s, err %s", DevicePluginSelectorLabelKey,
				node.ObjectMeta.Name, err.Error())
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	if devicePluginPodName != "" {
		err = c.waitForPodTermination(node, "DevicePlugin", PodNamePrefixes["devicePlugin"])
		if err != nil {
			return err
		} else {
			log.Log.Info(fmt.Sprintf("DevicePlugin pod %s has been terminated", devicePluginPodName))
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	/*** HW Manager Pod Deletion ***/
	labels = node.GetLabels()
	if _, ok := labels[HwMgrDsSelectorLabelKey]; ok {
		delete(labels, HwMgrDsSelectorLabelKey)
		node.SetLabels(labels)
		err = c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to remove label %s from node %s, err %s", HwMgrDsSelectorLabelKey,
				node.ObjectMeta.Name, err.Error())
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	if hwManagerPodName != "" {
		err = c.waitForPodTermination(node, "HwManager", PodNamePrefixes["HwManager"])
		if err != nil {
			return err
		} else {
			log.Log.Info(fmt.Sprintf("HW Manager pod %s has been terminated", hwManagerPodName))
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	/*** KMM Related Pod Deletion ***/
	labels = node.GetLabels()
	if _, ok := labels[KModuleSelectorLabelKey]; ok {
		delete(labels, KModuleSelectorLabelKey)
		node.SetLabels(labels)
		err = c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to remove label %s from node %s, err %s", KModuleSelectorLabelKey, node.ObjectMeta.Name, err.Error())
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	err = c.waitForKmmPodTermination(node)
	if err != nil {
		return err
	} else {
		log.Log.Info(fmt.Sprintf("KModule related pod %s has been terminated", kModulePodName))
	}

	/*** May delete configMap as well ***/
	log.Log.Info("Not deleting configMap", "Name", kmmConfigMapName)

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	/*** Firmware Pod Deletion ***/
	labels = node.GetLabels()
	if _, ok := labels[FirmwareDsSelectorLabelKey]; ok {
		delete(labels, FirmwareDsSelectorLabelKey)
		node.SetLabels(labels)
		err = c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to remove label %s from node %s, err %s", FirmwareDsSelectorLabelKey,
				node.ObjectMeta.Name, err.Error())
		}
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	if firmwarePodName != "" {
		err = c.waitForPodTermination(node, "firmware", PodNamePrefixes["firmware"])
		if err != nil {
			return err
		}
	} else {
		log.Log.Info(fmt.Sprintf("Firmware manager pod %s has been terminated", firmwarePodName))
	}

	log.Log.Info(fmt.Sprintf("Successfully removed existing pods from node %s", node.GetName()))
	return nil
}

func (c *ControllerState) updateSwVersionLabels(node *corev1.Node) error {
	labels := node.GetLabels()

	activeCR, err := getActiveCRonNode(labels)
	if err != nil {
		return err
	}

	log.Log.Info(fmt.Sprintf("Node %s has activeCR %v and reconciler policy is %s",
		node.ObjectMeta.Name, activeCR, c.x100Policy.ObjectMeta.Name))

	if activeCR == c.x100Policy.ObjectMeta.Name {
		if hasX100TeardownCompletedLabel(labels) {
			//Correct policy
			log.Log.Info(fmt.Sprintf("Teardown has already completed on node %s, enabling pod selectors",
				node.ObjectMeta.Name))
			return nil
		} else {
			// Incorrect policy
			return errors.New("Nodes activeCR is current policy so can't proceed to upgrade activeCR label")
		}

	}

	if !hasX100TeardownCompletedLabel(labels) {
		/***
			If node has successfully teared down resources, then it also has updated activeCR
			This means that we can avoid going into this path.
		***/
		log.Log.Info(fmt.Sprintf("Upgrading SwVersion Labels for node %s", node.ObjectMeta.Name))
		// Mark teardown sequence completion
		labels[x100TeardownCompleted] = "true"

		// Bookkeeping to maintain software version labels before and after upgrade
		labels[x100PriorCR] = labels[x100ActiveCR]
		labels[x100ActiveCR] = c.x100Policy.ObjectMeta.Name
		labels[x100SwVersion] = c.x100Policy.Spec.SwVersion
		// Once activeCR is updated, we don't need to block node aggregation
		if hasX100AggregationBlockedLabel(labels) {
			delete(labels, x100NodeAggregationBlocked)
		}
		node.SetLabels(labels)
		err := c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name,
				x100SwVersion, err.Error())
		}
	}
	return nil
}

func (c *ControllerState) enablePodSelectorLabels(node *corev1.Node) error {
	// Apply the pod selector labels
	labels := node.GetLabels()
	for _, label := range x100SelectorLabels {
		labels[label] = "true"
	}

	node.SetLabels(labels)
	err := c.rec.Update(context.TODO(), node)
	if err != nil {
		return fmt.Errorf("Unable to update labels on node %s , err %s", node.ObjectMeta.Name, err.Error())
	}
	return nil

}


func (c *ControllerState) waitForKmmPodTermination(node *corev1.Node) error {

	for {
		tnode := &corev1.Node{}
		_, tnode = c.fetchUpdatedNodeInstance(node)

		labels := tnode.GetLabels()
		isKmmReady := hasKmmReadylabel(labels)
		if !isKmmReady {
			break
		}
		//log.Log.Info(fmt.Sprintf("Still running KMM rmmod worker pods on %v", tnode.GetName()))
		time.Sleep(3 * time.Second)
	}
	log.Log.Info(fmt.Sprintf("Successfully drained all Modules from %v, Returning", node.GetName()))
	return nil
}

func (c *ControllerState) waitForPodTermination(node *corev1.Node, podType string, podNamePrefix string) error {

	for {
		pod, err := c.getNamedPodOnNode(node, podNamePrefix)
		// It is not necessarily a termination signal, may need additional checks
		if pod.Name == "" {
			// Pod was not found on the node
			break
		}
		if err != nil {
			return err
		}
		//if pod.DeletionTimestamp != nil {
		//	break
		//}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			break
		}
		//log.Log.Info(fmt.Sprintf("Still terminating %v pod on %v", pod.GetName(), node.GetName()))
	}
	log.Log.Info(fmt.Sprintf("Successfully drained %s pod from %v, Returning", podType, node.GetName()))
	return nil
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
	output, err := c.readHealthLogFromHwMgrPod(&pod)
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

func (c *ControllerState) getNamedPodOnNode(node *corev1.Node, podNamePrefix string) (corev1.Pod, error) {

	returnPod := corev1.Pod{}
	nodeName := node.GetName()

	opts := []client.ListOption{client.MatchingFields{"spec.nodeName": nodeName}}

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

func (c *ControllerState) fetchUpdatedNodeInstance(node *corev1.Node) (error, *corev1.Node) {
	// Fetches the latest updated instance of the given node
	opts := []client.ListOption{client.MatchingFields{"metadata.name": node.ObjectMeta.Name}}
	nodes := &corev1.NodeList{}
	err := c.rec.List(context.TODO(), nodes, opts...)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Could not fetch latest instance for node %s", node.ObjectMeta.Name))
		return err, nil
	}
	return nil, &nodes.Items[0]
}

func (c *ControllerState) readHealthLogFromHwMgrPod(pod *corev1.Pod) ([]byte, error) {

	containerName := pod.Spec.Containers[0].Name
	podName := pod.GetName()
	command := fmt.Sprintf("cat %s", healthLogPath)
	//log.Log.Info(fmt.Sprintf("Reading the health log from %s pod - %s container", podName, containerName))
	output, err := c.runCommandOnPod(podName, containerName, command)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Failed to read healthlog from pod %s, \n ERR:%v", podName, err))
		return output, err
	}
	log.Log.Info(fmt.Sprintf("Fetched health log from pod %s,\nLog:%v", podName, string(output)))
	return output, nil
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

func (c *ControllerState) isNodeUnschedulable(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		log.Log.Info(fmt.Sprintf("Node %s is unschedulable", node.GetName()))
		return true
	}
	return false
}
