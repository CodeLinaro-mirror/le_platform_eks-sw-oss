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
	"k8s.io/apimachinery/pkg/types"
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

func resetBootHealthTimer(annotations map[string]string) map[string]string {
	if _, ok := annotations[x100HealthCheckStartTime]; ok {
		delete(annotations, x100HealthCheckStartTime)
	}
	return annotations
}

func cleanupStaleAnnotations(annotations map[string]string) map[string]string {
	if _, ok := annotations[nodeProcessingStartTime]; ok {
		delete(annotations, nodeProcessingStartTime)
	}

	if _, ok := annotations[x100HealthCheckStartTime]; ok {
		delete(annotations, x100HealthCheckStartTime)
	}
	return annotations
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
	for _, label := range x100SelectorLabelsForDeletion {
		if _, ok := labels[label]; ok {
			delete(labels, label)
		}
	}
	return labels
}

func cleanupLabelsOnReboot(labels map[string]string) map[string]string {
	/***
	       Selector labels are not removed as they are terminated in
	       sequence.Add node to params to identify if pods were terminated
	       on this node
	***/

	// Remove Reboot labels related labels
	for _, label := range x100DeleteOnRebootLabels {
		if _, ok := labels[label]; ok {
			delete(labels, label)
		}
	}
	return labels
}

func (c *ControllerState) cleanupOnNodeIsolation(node *corev1.Node) error {
	labels := c.getNodeLabels(*node)

	//cleanup labels
	labels = cleanupStaleCRLabels(labels)
	labels = cleanupStaleSelectorLabels(labels)

	// Retain x100.present label for consistency
	labels[x100LabelKey] = "false"

	err := c.setX100NodeLabels(node, labels, x100ActiveCR, LabelUpdateDeletionType)
	if err != nil {
		return err
	}
	// Fetch updated node instance
	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return err
	}

	//cleanup annotations
	annotations := node.GetAnnotations()
	annotations = cleanupStaleAnnotations(annotations)
	node.SetAnnotations(annotations)
	err = c.setX100NodeAnnotations(node, annotations, nodeProcessingStartTime, LabelUpdateDeletionType)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Unable to reset node annotation %s for %s",
			nodeProcessingStartTime, node.ObjectMeta.Name))
		return err
	}
	return nil
}

func hasX100AggregationBlockedLabel(labels map[string]string) bool {
	if _, ok := labels[x100NodeAggregationBlocked]; ok {
		if labels[x100NodeAggregationBlocked] != "" {
			return true
		}
	}
	return false
}

func hasX100ComingUpAfterRebootLabel(labels map[string]string) bool {
	if _, ok := labels[X100ComingUpAfterReboot]; ok {
		if labels[X100ComingUpAfterReboot] == "true" {
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

func hasX100RollingBackUpgradeLabel(labels map[string]string) bool {
	if _, ok := labels[x100RollingBackUpgrade]; ok {
		if labels[x100RollingBackUpgrade] == "true" {
			return true
		}
	}
	return false
}

func hasX100RolledBackLabel(labels map[string]string) bool {
	if _, ok := labels[x100RolledBack]; ok {
		if labels[x100RolledBack] == "true" {
			return true
		}
	}
	return false
}

func hasX100EnablingFirstPolicy(labels map[string]string) bool {
	if _, ok := labels[x100EnablingFirstPolicy]; ok {
		if labels[x100EnablingFirstPolicy] == "true" {
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

func hasX100IsolateLabel(labels map[string]string) bool {
	if _, ok := labels[x100IsolateKey]; ok {
		if labels[x100IsolateKey] == "true" {
			return true
		}
	}
	return false
}

func hasKmmReadylabel(labels map[string]string) bool {
	for k, _ := range labels {
		if strings.Contains(k, kmmReadyLabel) {
			return true
		}
	}
	return false
}

func hasNodeProcessingStartTimeAnnotation(annotations map[string]string) bool {
	for k, _ := range annotations {
		if strings.Contains(k, nodeProcessingStartTime) {
			return true
		}
	}
	return false
}

func hasX100HealthCheckStartTimeAnnotation(annotations map[string]string) bool {
	for k, _ := range annotations {
		if strings.Contains(k, x100HealthCheckStartTime) {
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
	/***
		Either activeCR reflects correct policy
		Or aggregationBlocked label indicates the correct policy
		aggregationBlocked is transient state and would reflect
		the true policy
	***/
	if !hasX100AggregationBlockedLabel(labels) {
		if _, ok := labels[x100ActiveCR]; ok {
			if labels[x100ActiveCR] == policyName {
				return true
			}
		}
	} else {
		if policyName == labels[x100NodeAggregationBlocked] {
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
				labels := n.getNodeLabels(node)
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

func (c *ControllerState) getNodeLabels(node corev1.Node) map[string]string {
	updatedNode := &corev1.Node{}
	maxRetryCount := 5
	for count := 0; count < maxRetryCount; count++ {
		err := c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, updatedNode)
		if err != nil {
			continue
		}
		log.Log.Info(fmt.Sprintf("Fetched labels for node %s from API server",
			updatedNode.GetName()))
		return updatedNode.GetLabels()
	}
	return node.GetLabels()
}

func (c *ControllerState) setX100NodeLabels(node *corev1.Node, labels map[string]string,
	verifyLabel string, opType string) error {
	updatedNode := &corev1.Node{}
	maxRetryCount := 5

	for count := 0; count < maxRetryCount; count++ {
		node.SetLabels(labels)
		err := c.rec.Update(context.TODO(), node)
		if err != nil {
			if kerrors.IsConflict(err) {
				log.Log.Info(fmt.Sprintf("API server has new version for node %s, refetching",
					node.GetName()))
				err := c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, updatedNode)
				if err != nil {
					continue
				}
				node = updatedNode
			}
			// Could be a temporary glitch, retry
			log.Log.Info(fmt.Sprintf("Unable to set label %s on node %s at the moment due to error, retrying",
				verifyLabel, node.GetName()))
			time.Sleep(time.Second * 1)
			continue
		}

		// force the update propagation to API server
		err = c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, node)
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		log.Log.Info(fmt.Sprintf("Fetching latest version of node %s from API server",
			node.GetName()))

		// Successful update
		updatedLabels := c.getNodeLabels(*node)
		if opType == LabelUpdateAdditionType {
			if val, ok := updatedLabels[verifyLabel]; ok {
				if val == labels[verifyLabel] {
					log.Log.Info(fmt.Sprintf("Successfully applied label %s on node %s",
						verifyLabel, node.GetName()))
					return nil
				}
			}
		} else if opType == LabelUpdateDeletionType {
			if _, ok := updatedLabels[verifyLabel]; !ok {
				log.Log.Info(fmt.Sprintf("Successfully deleted label %s on node %s",
					verifyLabel, node.GetName()))
				return nil
			}
		}

		log.Log.Info(fmt.Sprintf("Unable to verify label %s for node %s at the moment, retrying",
			verifyLabel, node.GetName()))
	}
	return errors.New("Failed to update node labels")
}

func (c *ControllerState) setX100NodeAnnotations(node *corev1.Node, annotations map[string]string,
	verifyAnnotation string, opType string) error {
	updatedNode := &corev1.Node{}
	maxRetryCount := 5

	for count := 0; count < maxRetryCount; count++ {
		node.SetAnnotations(annotations)
		err := c.rec.Update(context.TODO(), node)
		if err != nil {
			if kerrors.IsConflict(err) {
				log.Log.Info(fmt.Sprintf("API server has new version for node %s, refetching",
					node.GetName()))
				err := c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, updatedNode)
				if err != nil {
					continue
				}
				node = updatedNode
			}
			// Could be a temporary glitch, retry
			log.Log.Info(fmt.Sprintf("Unable to set annotation %s on node %s at the moment due to error, retrying",
				verifyAnnotation, node.GetName()))
			time.Sleep(time.Second * 1)
		}

		// force the update propagation to API server
		err = c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, node)
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		log.Log.Info(fmt.Sprintf("Fetching latest version of node %s from API server",
			node.GetName()))

		// Successful update
		updatedAnnotations := node.GetAnnotations()
		if opType == LabelUpdateAdditionType {
			if val, ok := updatedAnnotations[verifyAnnotation]; ok {
				if val == updatedAnnotations[verifyAnnotation] {
					log.Log.Info(fmt.Sprintf("Successfully applied annotation %s on node %s",
						verifyAnnotation, node.GetName()))
					return nil
				}
			}
		} else if opType == LabelUpdateDeletionType {
			if _, ok := updatedAnnotations[verifyAnnotation]; !ok {
				log.Log.Info(fmt.Sprintf("Successfully deleted annotation %s on node %s",
					verifyAnnotation, node.GetName()))
				return nil
			}
		}

		log.Log.Info(fmt.Sprintf("Unable to verify annotation %s for node %s at the moment, retrying",
			verifyAnnotation, node.GetName()))
	}
	return errors.New("Failed to update node annotations")
}

func (c *ControllerState) testAndSetX100NodeAsIsolated(node *corev1.Node) bool {
	annotations := node.GetAnnotations()
	if timeStamp, ok := annotations[nodeProcessingStartTime]; ok {
		startTime, err := time.Parse(timeFormat, timeStamp)
		if err != nil {
			return false
		}
		if time.Since(startTime) > time.Minute*gracePeriodNodeCompletion {
			labels := c.getNodeLabels(*node)
			labels[x100IsolateKey] = "true"
			log.Log.Info(fmt.Sprintf("Node %s processing start time was %v, grace period elapsed, timing out",
				node.GetName(), startTime))

			// Clean labels
			labels[x100LabelKey] = "false"
			labels = cleanupStaleCRLabels(labels)
			labels = cleanupStaleSelectorLabels(labels)

			err = c.setX100NodeLabels(node, labels, x100IsolateKey, LabelUpdateAdditionType)
			if err != nil {
				return false
			}

			err, node = c.fetchUpdatedNodeInstance(node)
			if err != nil {
				return false
			}

			annotations = node.GetAnnotations()
			delete(annotations, nodeProcessingStartTime)
			delete(annotations, x100HealthCheckStartTime)
			node.SetAnnotations(annotations)
			err = c.setX100NodeAnnotations(node, annotations, nodeProcessingStartTime, LabelUpdateDeletionType)
			if err != nil {
				log.Log.Info(fmt.Sprintf("Unable to reset node annotation %s for %s",
					nodeProcessingStartTime, node.ObjectMeta.Name))
				return false
			}
			return true
		}
	}

	return false
}

func (c *ControllerState) hasX100DeviceHealthCheckTimedout(node *corev1.Node) bool {
	annotations := node.GetAnnotations()
	if timeStamp, ok := annotations[x100HealthCheckStartTime]; ok {
		startTime, err := time.Parse(timeFormat, timeStamp)
		if err != nil {
			return false
		}
		if time.Since(startTime) > time.Minute*gracePeriodHealthCheck {
			log.Log.Info(fmt.Sprintf("Health status check for x100 card on node %s has timed out",
				node.GetName()))
			return true
		}
	}
	return false
}

func (c *ControllerState) createNFDResources() error {
	/***
		NFD resources need to be available beforehand
		Since NFD attaches labels to the nodes that
		labelX100NodeswithCR() then processes
	***/
	log.Log.Info("Creating NFD resources to label nodes with appropriate pci labels")
	allowSleep := true
	_ = createAssetMap(nfdResourcePath)

	// Create the resources using  resources from AssetMap and creation callbacks
	asset := AssetMap[nfdResourcePath]

	for _, robj := range asset.objectMappings {
		_, err := createKindResource(*c, robj.key, robj.value)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				allowSleep = false
			} else {
				return err
			}
		}
	}
	if allowSleep {
		time.Sleep(10 * time.Second)
	}
	return nil
}

func (c *ControllerState) getPodCompletionStatus(node *corev1.Node, podNamePrefix string, completionPath string) (bool, error) {
	status := false
	log.Log.Info(fmt.Sprintf("Getting completion status for %s from %s", podNamePrefix, completionPath))
	pod, err := c.getNamedPodOnNode(node, podNamePrefix)
	if err != nil {
		log.Log.Info(fmt.Sprintf("Failed to fetch pod %s on node %s",
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
		log.Log.Info(fmt.Sprintf("Failed to read pod completion status on node %s", node.GetName()))
		return status, err
	}

	//check for unavailability of daemonset
	opts := []client.ListOption{client.MatchingLabels{"app": podNamePrefix}}
	list := &appsv1.DaemonSetList{}
	err = c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		log.Log.Info("Could not get DaemonSetList", err)
	}
	for _, ds := range list.Items {
		if ds.Status.NumberUnavailable != 0{
			log.Log.Info(fmt.Sprintf("Daemonset with prefix %s is not available yet", podNamePrefix))
			return status, nil
		}
	}

	out := string(output)
	if strings.Contains(out, "completed") {
		log.Log.Info(fmt.Sprintf("Completion status as read from pod on node %s is %s", node.GetName(), out))
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
				labels := n.getNodeLabels(node)
				if hasKmmReadylabel(labels) {
					nodesUnderCurrentPolicy += 1
				}
			}
		}
	}

	if nodesUnderCurrentPolicy == 0 {
		// No node under policy yet has a module pod resource
		return xcardv1.NotOperational
	}

	return xcardv1.Operational
}

func isPodReady(labelkey string, labelvalue string, n ControllerState, phase corev1.PodPhase) xcardv1.State {
	/***
		Pod ready only needs to check for existence of atleast one Pod
		It doesn't need to wait for pods to come up in all nodes
		under current policy.

	***/
	list := &corev1.PodList{}

	opts := []client.ListOption{&client.MatchingLabels{labelkey: labelvalue}}

	err := n.rec.List(context.TODO(), list, opts...)
	if err != nil {
		log.Log.Info("Could not get PodList", err)
		return xcardv1.NotOperational
	}

	log.Log.Info(fmt.Sprintf("%v - %s pods in the cluster", len(list.Items), labelvalue))

	if len(list.Items) == 0 {
		return xcardv1.NotOperational
	}

	pd := list.Items[0]
	if pd.Status.Phase != phase {
		log.Log.Info("DEBUG: Pod", "Phase", pd.Status.Phase, "!=", phase)
		return xcardv1.NotOperational
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

func (c *ControllerState) isFirmwareDSCreated() bool {
	ds := &appsv1.DaemonSet{}
	dsName := FirmwareDsName + SoftwareVersionSeperator + c.x100Policy.Spec.SwVersion
	log.Log.Info(fmt.Sprintf("Checking if %s daemonSet exists", dsName))
	maxRetryCount := 3
	for count := 0; count < maxRetryCount; count++ {
		err := c.rec.Get(context.TODO(), types.NamespacedName{Name: FirmwareDsName}, ds)
		if err != nil {
			if kerrors.IsNotFound(err) {
				break
			}
		} else {
			return true
		}
	}
	return false
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
		labels := c.getNodeLabels(node)
		/***
			We need to check if the time since healthcheck started has passed
			300 secs. If yes, we need mark the status as false
			If boot status is unknown still, we can skip processing the node
		***/
		if isX100RunningWithPolicy(labels, currentCR) {
			timedout := c.hasX100DeviceHealthCheckTimedout(&node)
			hasX100BootupSuccess := true
			if _, ok := labels[x100BootupSuccess]; ok {
				if labels[x100BootupSuccess] != "unknown" || timedout {
					if labels[x100BootupSuccess] != "true" {
						hasX100BootupSuccess = false
						labels[x100BootupSuccess] = "false"
					}
					labels[x100BootupStatusMarked] = "true"
					log.Log.Info(fmt.Sprintf("Marking bootSuccess label as %s on node %s",
						labels[x100BootupSuccess], node.GetName()))

					//cleanup
					nodeHasRolledBackLabel := hasX100RolledBackLabel(labels)
					nodeHasEnablingFirstPolicyLabel := hasX100EnablingFirstPolicy(labels)
					hasX100RebootedLabel := hasX100ComingUpAfterRebootLabel(labels)
					hasx100TeardownCompleted := hasX100TeardownCompletedLabel(labels)
					hasx100Upgrading := hasX100UpgradingLabel(labels)

					if hasx100Upgrading {
						delete(labels, x100Upgrading)
					}
					if nodeHasRolledBackLabel {
						delete(labels, x100RolledBack)
					}
					if nodeHasEnablingFirstPolicyLabel {
						delete(labels, x100EnablingFirstPolicy)
					}
					if hasX100RebootedLabel {
						delete(labels, X100ComingUpAfterReboot)
					}
					if hasx100TeardownCompleted {
						delete(labels, x100TeardownCompleted)
					}

					if hasx100Upgrading && !hasX100BootupSuccess {
						labels[x100UpgradeFailed] = labels[x100SwVersion]
						log.Log.Info(fmt.Sprintf("Marking upgrade as failed on node %s as bootSuccess is %v",
							node.GetName(), hasX100BootupSuccess))
					}

					err = c.setX100NodeLabels(&node, labels, x100BootupStatusMarked, LabelUpdateAdditionType)
					if err != nil {
						log.Log.Info("Unable to label node", node.ObjectMeta.Name, " with ", x100BootupStatusMarked, err.Error())
					}

					// Delete timing related annotations from the node as boot status is marked
					annotations := node.GetAnnotations()
					annotations = cleanupStaleAnnotations(annotations)
					node.SetAnnotations(annotations)
					err := c.setX100NodeAnnotations(&node, annotations, x100HealthCheckStartTime, LabelUpdateDeletionType)
					if err != nil {
						log.Log.Info(fmt.Sprintf("Failed to delete x100HealthCheckStartTime annotation from node %s",
							node.GetName()))
					}
				}
			}
		}
	}
}

func (c *ControllerState) attachX100Annotation(node *corev1.Node, annotation string) (*corev1.Node, error) {
	annotations := node.GetAnnotations()
	timeStamp := time.Now().UTC().Format(timeFormat)
	annotations[annotation] = timeStamp
	node.SetAnnotations(annotations)

	//err := c.rec.Update(context.TODO(), node)
	err := c.setX100NodeAnnotations(node, annotations, annotation, LabelUpdateAdditionType)
	if err != nil {
		return node, fmt.Errorf("Unable to update node %s with %s annotation, err %s",
			node.ObjectMeta.Name, annotation, err.Error())
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return node, err
	}
	return node, nil
}

func (c *ControllerState) deleteX100Annotation(node *corev1.Node, annotation string) (*corev1.Node, error) {
	annotations := node.GetAnnotations()

	delete(annotations, annotation)
	node.SetAnnotations(annotations)
	//err := c.rec.Update(context.TODO(), node)
	err := c.setX100NodeAnnotations(node, annotations, annotation, LabelUpdateDeletionType)
	if err != nil {
		return node, fmt.Errorf("Unable to delete %s annotation from node %s, err %s",
			annotation, node.GetName(), err.Error())
	}

	err, node = c.fetchUpdatedNodeInstance(node)
	if err != nil {
		return node, err
	}

	return node, nil
}

func (c *ControllerState) runUpgradeSequenceForNode(node *corev1.Node) error {
	// Transition the node policy state to upgrading

	err := c.transitionToUpgradingState(node)
	if err != nil {
		return err
	}
	log.Log.Info(fmt.Sprintf("Node %s transitioned to upgrading state", node.ObjectMeta.Name))

	// Wait for existing pods to terminate
	labels := c.getNodeLabels(*node)
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
		return err
	}

	log.Log.Info(fmt.Sprintf("Enabled pod selector labels for node %s", node.GetName()))
	return nil
}

func (c *ControllerState) transitionToUpgradingState(node *corev1.Node) error {
	labels := c.getNodeLabels(*node)

	if !hasX100UpgradingLabel(labels) {
		//Remove stale labels if any
		for _, label := range x100StatusLabels {
			if _, ok := labels[label]; ok {
				delete(labels, label)
			}
		}
		labels[x100Upgrading] = "true"

		node.SetLabels(labels)
		err := c.setX100NodeLabels(node, labels, x100Upgrading, LabelUpdateAdditionType)
		//err := c.rec.Update(context.TODO(), node)
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
	labels := c.getNodeLabels(*node)
	if _, ok := labels[DevicePluginSelectorLabelKey]; ok {
		delete(labels, DevicePluginSelectorLabelKey)
		node.SetLabels(labels)
		//err = c.rec.Update(context.TODO(), node)
		err = c.setX100NodeLabels(node, labels, DevicePluginSelectorLabelKey, LabelUpdateDeletionType)
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

	/*** KMM Related Pod Deletion ***/
	labels = c.getNodeLabels(*node)
	if _, ok := labels[KModuleSelectorLabelKey]; ok {
		delete(labels, KModuleSelectorLabelKey)
		node.SetLabels(labels)
		err = c.setX100NodeLabels(node, labels, KModuleSelectorLabelKey, LabelUpdateDeletionType)
		//err = c.rec.Update(context.TODO(), node)
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

	/*** HW Manager Pod Deletion ***/
	labels = c.getNodeLabels(*node)
	if _, ok := labels[HwMgrDsSelectorLabelKey]; ok {
		/***
		//Test code to cause upgrade stuck on a node
		if node.GetName() == "psi-lassen-lh3" {
			log.Log.Info(fmt.Sprintf("HW Manager pod %s can't be terminated, treat as stuck", hwManagerPodName))
			return errors.New("Hw manager pod is stuck")
		}
		***/

		delete(labels, HwMgrDsSelectorLabelKey)
		node.SetLabels(labels)
		err = c.setX100NodeLabels(node, labels, HwMgrDsSelectorLabelKey, LabelUpdateDeletionType)
		//err = c.rec.Update(context.TODO(), node)
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
	
	/*** Firmware Pod Deletion ***/
	labels = c.getNodeLabels(*node)
	if _, ok := labels[FirmwareDsSelectorLabelKey]; ok {
		delete(labels, FirmwareDsSelectorLabelKey)
		node.SetLabels(labels)
		//err = c.rec.Update(context.TODO(), node)
		err = c.setX100NodeLabels(node, labels, FirmwareDsSelectorLabelKey, LabelUpdateDeletionType)
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
	labels := c.getNodeLabels(*node)

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
		err := c.setX100NodeLabels(node, labels, x100TeardownCompleted, LabelUpdateAdditionType)
		//err := c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name,
				x100TeardownCompleted, err.Error())
		}
	}
	return nil
}

func (c *ControllerState) enablePodSelectorLabels(node *corev1.Node) error {
	// Apply the pod selector labels
	labels := c.getNodeLabels(*node)
	updateLabels := false
	currentIndex := 0
	/***
		Detect which labels are present
		check in sequence fw -> hwmgr -> kmodule --> dplugin
		Enable each label only after the completion flag is present
		/var/podcheck/fw_pod_running,
		/var/podcheck/hwmgr_pod_running,
		for kmm check the ready label if present
	***/
	for index, label := range x100SelectorLabelsForCreation {
		currentIndex = index
		if _, ok := labels[label]; !ok {
			// Skip the ones already applied
			if index == 0 {
				log.Log.Info("Skipping the processing for index 0 during enablePodSelectorLabels()")

			} else if index < 3 {
				// index = 1 (HwManager) and index = 2 (KModules)
				if index == 1 || index == 2 {
					podName := getPodNamePrefixFromIndex(index - 1)
					log.Log.Info(fmt.Sprintf("podname fetched for index %v is %s", index-1, podName))
					// Check if previous pod has signalled completion
					isComplete, err := c.getPodCompletionStatus(node, podName, PodCompletionPath[index-1])
					if err != nil {
						return err
					}
					if !isComplete {
						//return errors.New(fmt.Sprintf("PodCompletionStatus is incomplete for %s on node %s",
						//	podName, node.GetName()))
						log.Log.Info(fmt.Sprintf("PodCompletionStatus is incomplete for %s on node %s", podName, node.GetName()))
						return errors.New(fmt.Sprintf("PodCompletionStatus is incomplete for %s on node %s", podName, node.GetName()))
					}
				}
			} else {
				// index == 3 (devicePlugin)
				podName := getPodNamePrefixFromIndex(index - 1)

				isKmmReady := hasKmmReadylabel(labels)
				if !isKmmReady {
					log.Log.Info(fmt.Sprintf("Pod %s is not ready on node %s", podName, node.GetName()))
					return errors.New(fmt.Sprintf("PodCompletionStatus is incomplete for %s on node %s", podName, node.GetName()))
				}
			}
			log.Log.Info(fmt.Sprintf("Setting %s to true", label))
			labels[label] = "true"
			updateLabels = true
			break
		}
	}

	if updateLabels {
		node.SetLabels(labels)
		log.Log.Info(fmt.Sprintf("Applying the updated labels %s from inside enablePodSelectorLabels to %s", labels, node.GetName()))
		//err := c.rec.Update(context.TODO(), node)
		err := c.setX100NodeLabels(node, labels, x100SelectorLabelsForCreation[currentIndex], LabelUpdateAdditionType)
		if err != nil {
			return fmt.Errorf("Unable to label node %s with %v, err %s", node.ObjectMeta.Name,
				x100SelectorLabelsForCreation, err.Error())
		}
	} else {
		log.Log.Info(fmt.Sprintf("All relevant labels already enabled in enablePodSelectorLabels(), returning..."))
	}
	return nil
}

func (c *ControllerState) transitionToRollingBackUpgradeState(node *corev1.Node) error {
	labels := c.getNodeLabels(*node)

	if !hasX100RollingBackUpgradeLabel(labels) {
		for _, label := range x100StatusLabels {
			if _, ok := labels[label]; ok {
				delete(labels, label)
			}
		}
		labels[x100RollingBackUpgrade] = "true"

		node.SetLabels(labels)
		err := c.setX100NodeLabels(node, labels, x100RollingBackUpgrade, LabelUpdateAdditionType)
		//err := c.rec.Update(context.TODO(), node)
		if err != nil {
			return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name,
				x100RollingBackUpgrade, err.Error())
		}
	}
	return nil
}

func (c *ControllerState) waitForKmmPodTermination(node *corev1.Node) error {
	labels := c.getNodeLabels(*node)
	isKmmReady := hasKmmReadylabel(labels)
	if isKmmReady {
		log.Log.Info(fmt.Sprintf("Waiting for KMM Pod termination on %s",
			node.GetName()))

		// Unable to terminate pod
		return errors.New(fmt.Sprintf("Waiting for KMM Pod termination on %s",
			node.GetName()))
	}
	log.Log.Info(fmt.Sprintf("Successfully drained all Modules from %v, Returning", node.GetName()))
	return nil
}

func (c *ControllerState) waitForPodTermination(node *corev1.Node, podType string, podNamePrefix string) error {
	pod, err := c.getNamedPodOnNode(node, podNamePrefix)

	if err != nil {
		log.Log.Info(fmt.Sprintf("Waiting for Pod termination on %s", node.GetName()))

		// Unable to fetch the pod
		return errors.New(fmt.Sprintf("Waiting to terminate %s pod successfully", podType))
	}

	if pod.Name == "" || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		// Pod terminated
		log.Log.Info(fmt.Sprintf("Successfully drained %s pod from %s, Returning",
			podType, node.GetName()))
		return nil
	}
	log.Log.Info(fmt.Sprintf("Waiting to terminate %s pod successfully", podType))
	return errors.New(fmt.Sprintf("Waiting to terminate %s pod successfully", podType))
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

func (c * ControllerState) isNodeReady(node* corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}


func (c *ControllerState) isX100NodeEligibleForBootStatusCheck(node *corev1.Node) (bool, int) {
	/***
		Ignore Node if
			a. Not running with current policy
			b. Has no X100 card attached
			c. Already has bootup success label or status marked attached
			d. UpgradeFailed or RollingBack upgrade
			e. Upgrading/Rebooting but Teardown not completed
			f. DevicePlugin Pod is not available(last pod in creation sequence)

			certain cases do not warrant a node decrement as they are responsibility
			of current policy but can not be processed right now
			e.g Node is upgrading but Teardown not completed.
	***/
	labels := c.getNodeLabels(*node)
	if !isX100RunningWithPolicy(labels, c.x100Policy.ObjectMeta.Name) {
		log.Log.Info(fmt.Sprintf("Node %s not running with current policy %s, skipping bootStatusCheck",
			node.GetName(), c.x100Policy.GetName()))
		return false, 1

	} else if !hasCustomX100Label(labels) {
		log.Log.Info(fmt.Sprintf("Node %s has no X100 card attached, skipping bootStatusCheck",
			node.GetName()))
		return false, 1

	} else if hasX100BootupSuccessLabel(labels) {
		log.Log.Info(fmt.Sprintf("Node %s has bootup success label true, skipping bootStatusCheck",
			node.GetName()))
		return false, 1

	} else if hasX100BootupStatusMarkedLabel(labels) {
		log.Log.Info(fmt.Sprintf("Node %s has bootup status marked label, skipping bootStatusCheck",
			node.GetName()))
		return false, 1

	} else if hasX100UpgradeFailedLabel(labels) || hasX100RollingBackUpgradeLabel(labels) {
		log.Log.Info(fmt.Sprintf("Node %s is on the downgrade/rollback path, skipping bootStatusCheck",
			node.GetName()))
		return false, 0

	} else if hasX100ComingUpAfterRebootLabel(labels) && !hasX100TeardownCompletedLabel(labels) {
		log.Log.Info(fmt.Sprintf("Node %s has coming up after reboot label but termination pending, skipping bootStatusCheck",
			node.GetName()))
		return false, 0

	} else if hasX100UpgradingLabel(labels) && !hasX100TeardownCompletedLabel(labels) {
		log.Log.Info(fmt.Sprintf("Node %s is on upgrade path but teardown is not completed, skipping bootStatusCheck",
			node.GetName()))
		return false, 0

	} else {
		pod, err := c.getNamedPodOnNode(node, PodNamePrefixes["devicePlugin"])
		if err != nil || (pod.Status.Phase != "Running") {
			log.Log.Info(fmt.Sprintf("DevicePlugin pod is not running on node %s, skipping bootStatusCheck",
				node.GetName()))
			return false, 0
		}
	}
	log.Log.Info(fmt.Sprintf("Node %s is eligible for bootStatusCheck, processing...",
		node.GetName()))
	// We can process the node further
	return true, 0
}
