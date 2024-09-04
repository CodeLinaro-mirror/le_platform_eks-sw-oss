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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	xcardv1 "x100-operator/api/v1"

	"context"
	"errors"
	"fmt"
	"strconv"
)

func (c *ControllerState) labelX100NodesIsolated(policy *xcardv1.X100ManagementPolicy) error {
	// fetch all nodes in the cluster
	log.Log.Info("Entering labelX100NodesIsolated()")

	opts := []client.ListOption{}
	list := &corev1.NodeList{}

	err := c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check annotations, err %s", err.Error())
	}

	for _, node := range list.Items {
		labels := c.getNodeLabels(node)

		if isX100RunningWithPolicy(labels, policy.GetName()) {
			// Don't process the nodes that don't have x100 cards
			if !hasCustomX100Label(labels) {
				continue
			}
			nodeIsolated := c.testAndSetX100NodeAsIsolated(&node)
			if nodeIsolated {
				log.Log.Info(fmt.Sprintf("Isolated node %s from further processing, needs manual intervention",
					node.GetName()))
			}
		}
	}

	return nil
}

func (c *ControllerState) labelX100Nodes(policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {
	// Check nodes for isolation
	err := c.labelX100NodesIsolated(policy)
	if err != nil {
		return err
	}

	// fetch all nodes in the cluster
	log.Log.Info("Entering labelX100Nodes()")
	opts := []client.ListOption{}
	list := &corev1.NodeList{}
	err = c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
	}

	for _, node := range list.Items {
		// get node labels
		labels := c.getNodeLabels(node)
		hasCustomLabel := hasCustomX100Label(labels)
		hasPCILabel := hasX100PCILabels(labels)
		hasIsolateLabel := hasX100IsolateLabel(labels)
		isNodeSchedulable := !c.isNodeUnschedulable(&node)
		hasBootStatusMarked := hasX100BootupStatusMarkedLabel(labels)

		if !hasCustomLabel && hasPCILabel && isNodeSchedulable {
			// Check for Isolate Label before applying Custom Label
			if hasIsolateLabel {
				labels[x100LabelKey] = "false"
			} else {
				labels[x100LabelKey] = x100LabelValue
			}
			err = c.setX100NodeLabels(&node, labels, x100LabelKey, LabelUpdateAdditionType)
			if err != nil {
				return fmt.Errorf("Unable to label node %s with %s, err %s",
					node.ObjectMeta.Name, x100LabelKey, err.Error())
			}
			log.Log.Info(fmt.Sprintf("Label %s set to true", x100LabelKey))

		} else if hasCustomLabel && (hasIsolateLabel || !hasPCILabel) {
			// previously labelled node and no longer has X100
			// reset the custom label as it is not valid
			if hasIsolateLabel {
				log.Log.Info(fmt.Sprintf("Node %s has been isolated, removed from further processing.",
					node.ObjectMeta.Name))
			}
			labels[x100LabelKey] = "false"
			node.SetLabels(labels)
			err = c.setX100NodeLabels(&node, labels, x100LabelKey, LabelUpdateAdditionType)
			if err != nil {
				return fmt.Errorf("Unable to reset node label for %s with %s, err %s", node.ObjectMeta.Name, x100LabelKey, err.Error())
			}
			log.Log.Info(fmt.Sprintf("Stale Label %s set to false", x100LabelKey))
		} else {
			log.Log.Info(fmt.Sprintf("X100 Labelling conditions => hasCustomLabel=%v,hasPCILabel=%v,hasIsolateLabel=%v,isNodeSchedulable=%v",
				hasCustomLabel, hasPCILabel, hasIsolateLabel, isNodeSchedulable))
		}

		labels = c.getNodeLabels(node)
		if isX100RunningWithPolicy(labels, policy.GetName()) {
			annotations := node.GetAnnotations()
			// x100 card is present and processing time annotation is not present
			if hasCustomLabel && !hasIsolateLabel && !hasBootStatusMarked && !hasNodeProcessingStartTimeAnnotation(annotations) {
				_, err := c.attachX100Annotation(&node, nodeProcessingStartTime)
				if err != nil {
					return err
				}
				log.Log.Info(fmt.Sprintf("Attached nodeProcessingStartTime annotation to the node %s during labelX100Nodes()",
					node.GetName()))
			}
		}
	}
	return nil
}

func (c *ControllerState) labelX100NodeswithCR(policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {
	/***
		If policy node list is empty
			apply only on nodes that have no activeCR or are new

		If there are nodes that are not part of policy selectorlist
		But are having the activeCR as current policy
		We need to aggregate these into policy nodeSelectorList
		This is the case where we apply policy to nodes piece by piece
		e.g. 10 * 10 if 100 nodes, apply v1 to 10 at a time
		current implementation loses track of the earlier 10 Nodes

		Then
			Iterate over all worker nodes and rectify labels

			For each Node, only if node has x100 label, process further
				If node has an activeCR
						Fetch policy that has this node in NodeSelector List
						Remove the node from that policy
						Remove the labels from the node to initiate termination
						Save the labels for priorCR
						Wait for pod termination
						Reapply appropriate labels to track new CR
				else
					Must be first time boot up or first time policy application
					fill the labels priorCR, activeCR, SwVersion
					priorCR should be kept empty

		Note : Sequence for creation of resources is maintained through the
		labels created by the dependency and the dependent checks for this
		through a label. Create label on pod ready condition.

		Node selectors
		    qualcomm.com/x100.present: "true"
			qualcomm.com/x100.swversion: "v1"
			qualcomm.com/x100.fw.present: "true"

			individual pod related labels can be used
			to control creation and termination.

	***/
	log.Log.Info("Entering labelX100NodeswithCR()")
	nodeInCRSelectorList := policySpec.NodeSelector
	hasNodeQualifiedForResourceCreationStage := false

	log.Log.Info("labelX100NodeswithCR() ", "Nodes list length from CRD ", len(nodeInCRSelectorList))

	if len(nodeInCRSelectorList) == 0 {
		log.Log.Info(fmt.Sprintf("Selector list is empty for policy %s, applying to all nodes under no policy",
			policy.ObjectMeta.Name))
		// fetch all nodes and filter ones that have no active policy associated with them
		opts := []client.ListOption{}
		list := &corev1.NodeList{}
		err := c.rec.List(context.TODO(), list, opts...)
		if err != nil {
			return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
		}

		nodesWithNoActiveCRLabel := []string{}
		for _, node := range list.Items {
			// get node labels
			err := c.rec.Get(context.TODO(), types.NamespacedName{Name: node.GetName()}, &node)
			if err != nil {
				return err
			}

			labels := c.getNodeLabels(node)
			hasx100Attached := hasCustomX100Label(labels)
			nodehasNoActiveCRLabel := !hasActiveCRLabel(labels)
			nodehasOwnerPolicyDeletedLabel := hasX100OwnerPolicyDeletedLabel(labels)
			log.Log.Info(fmt.Sprintf("Node %s has x100=%v, noActiveCR=%v, ownerPolicyDeleted=%v",
				node.ObjectMeta.Name, hasx100Attached, nodehasNoActiveCRLabel, nodehasOwnerPolicyDeletedLabel))
			if hasx100Attached && nodehasNoActiveCRLabel && !nodehasOwnerPolicyDeletedLabel {
				// node has no policy applied to it yet, first time application of policy to the node
				log.Log.Info(fmt.Sprintf("Node %s has no activeCR label", node.GetName()))
				nodesWithNoActiveCRLabel = append(nodesWithNoActiveCRLabel, node.ObjectMeta.Name)
			}
		}
		//Modify the policy to update the policyObject.Spec.NodeSelector[] with nodesWithNoActiveCRLabel
		policy.Spec.NodeSelector = append(policy.Spec.NodeSelector, nodesWithNoActiveCRLabel...)
		c.rec.Update(context.TODO(), policy)
		if err != nil {
			return fmt.Errorf("Unable to update nodeselector list on policy %s , err %s",
				policy.ObjectMeta.Name, err.Error())
		}
	}

	// Aggregate nodelist for current policy
	// Irrespective of node selector list being empty
	nodesToBeIncludedInPolicy := false

	opts := []client.ListOption{&client.MatchingLabels{x100ActiveCR: policy.ObjectMeta.Name}}
	log.Log.Info(fmt.Sprintf("Node LabelSelector %s=%s", x100ActiveCR, policy.ObjectMeta.Name))

	list := &corev1.NodeList{}
	err := c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
	}

	log.Log.Info(fmt.Sprintf("Found %d nodes with node selector %s=%s", len(list.Items), x100ActiveCR, policy.ObjectMeta.Name))

	for _, node := range list.Items {
		includeNode := true
		labels := c.getNodeLabels(node)
		for _, ns := range nodeInCRSelectorList {
			// Nodes that are not part of selector list
			if node.ObjectMeta.Name == ns {
				// Node already part of policy node selector list
				// Node is being moved to a different policy but activeCR
				// still indicates that it is under current policy
				// aggregation block guards against this condition
				includeNode = false
				break
			}
		}
		if includeNode {
			// activeCR is the policy CR on this Node
			// this needs to be added
			//nodesToBeIncludedInPolicy.Items = append(nodesToBeIncludedInPolicy.Items, node)
			if hasX100AggregationBlockedLabel(labels) {
				break
			}
			policy.Spec.NodeSelector = append(policy.Spec.NodeSelector, node.ObjectMeta.Name)
			nodesToBeIncludedInPolicy = true
		}
	}

	if nodesToBeIncludedInPolicy {
		log.Log.Info(fmt.Sprintf("Adding additional nodes that had activeCR as current policy %s to node selector list",
			policy.ObjectMeta.Name))
		err = c.rec.Update(context.TODO(), policy)
		if err != nil {
			return fmt.Errorf("Unable to update nodeselector list on policy %s , err %s", policy.ObjectMeta.Name, err.Error())
		}
	}

	// Beyond this point, every policy has a node list associated with it
	// We can process all nodes now to applying appropriate policy/software version labels

	opts = []client.ListOption{}
	list = &corev1.NodeList{}
	err = c.rec.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
	}

	/***
		To avoid excessively large number of nodes, we are processing based on
		nodeSelectorList of current policy as we don't expect reconciler to be triggered
		on any label changes except the nfd labels going off resulting on x100 label going
		off. Requests for all policies are generated though which means we need to handle
		all nodeSelectorList across policies.
		For any policy change though, we would have the correct policy instance.
	***/
	updatedPolicy := &xcardv1.X100ManagementPolicy{}
	err = c.rec.Get(context.TODO(), types.NamespacedName{Name: policy.GetName()}, updatedPolicy)
	if err != nil {
		return err
	}
	log.Log.Info(fmt.Sprintf("Fetched updated %s after node aggregation", policy.GetName()))
	policy = updatedPolicy
	nodeInCRSelectorList = policy.Spec.NodeSelector

	log.Log.Info("After aggregation", "Nodes list length from CRD ", len(nodeInCRSelectorList))

	for _, ns := range nodeInCRSelectorList {
		for _, node := range list.Items {
			// Process only nodes that are in the policy nodeSelectorList
			if node.ObjectMeta.Name == ns {
				log.Log.Info(fmt.Sprintf("Found node object for node %s from selector list", node.ObjectMeta.Name))
				// get node labels
				labels := c.getNodeLabels(node)
				hasx100Attached := hasCustomX100Label(labels)
				nodehasNoActiveCRLabel := !hasActiveCRLabel(labels)
				nodeEnablingPodSelectors := hasX100EnablingFirstPolicy(labels)
				nodeHasRebooted := hasX100ComingUpAfterRebootLabel(labels)
				log.Log.Info(fmt.Sprintf("Node %s has x100Attached=%v, noActiveCR=%v, enablingPodSelectors=%v, comingFromReboot=%v",
					node.GetName(), hasx100Attached, nodehasNoActiveCRLabel, nodeEnablingPodSelectors, nodeHasRebooted))
				if hasx100Attached {
					// Processing worker nodes that have x100 card attached to them
					if nodehasNoActiveCRLabel {
						log.Log.Info(fmt.Sprintf("Processing node %s for no activeCR label", node.ObjectMeta.Name))
						// first time policy application to the node
						labels[x100ActiveCR] = policy.ObjectMeta.Name
						labels[x100SwVersion] = policySpec.SwVersion
						labels[x100PriorCR] = "" // No priorCR

						if hasX100OwnerPolicyDeletedLabel(labels) {
							delete(labels, x100OwnerPolicyDeleted)
						}
						// Acts as a flag to identify that we need to reenter this loop
						if !hasX100EnablingFirstPolicy(labels) {
							labels[x100EnablingFirstPolicy] = "true"
						}
						// Enable individual pod selector labels to control deletion sequence
						err = c.enablePodSelectorLabels(&node)
						if err != nil {
							// If we are unable to set labels in next step and reenter
							// enablePodSelectorLabels() will return error for index 1
							// as it already processed index 0 and now looks for fw pod
							// If DS is not created yet, we need to move ahead
							if c.isFirmwareDSCreated() {
								break
							} else {
								hasNodeQualifiedForResourceCreationStage = true
							}
						}

						log.Log.Info(fmt.Sprintf("Labelling node %s with activeCR label", node.ObjectMeta.Name))
						node.SetLabels(labels)
						err = c.setX100NodeLabels(&node, labels, x100ActiveCR, LabelUpdateAdditionType)
						if err != nil {
							return fmt.Errorf("Unable to label node %s with %s, err %s",
								node.ObjectMeta.Name, x100ActiveCR, err.Error())
						}
						hasNodeQualifiedForResourceCreationStage = true
					} else if nodeEnablingPodSelectors {
						// Enable individual pod selector labels to control creation sequence
						log.Log.Info(fmt.Sprintf("Enabling Pod selector labels for node %s under policy %s",
							node.ObjectMeta.Name, policy.ObjectMeta.Name))
						err = c.enablePodSelectorLabels(&node)
						if err != nil {
							break
						}
						hasNodeQualifiedForResourceCreationStage = true
					} else if nodeHasRebooted {
						log.Log.Info(fmt.Sprintf("[Reboot] Terminating pods for node %s on reboot", node.ObjectMeta.Name))

						if !hasX100TeardownCompletedLabel(labels) {
							err = c.teardownX100ManagementPolicyOwnedPodsOnNode(&node)
							if err != nil {
								break
							}

							err, rnode := c.fetchUpdatedNodeInstance(&node)
							if err != nil {
								return err
							}
							node = *rnode
							labels = c.getNodeLabels(node)

							log.Log.Info(fmt.Sprintf("[Reboot] Termination completed, setting %s to true", x100TeardownCompleted))
							// Mark teardown sequence completion
							labels[x100TeardownCompleted] = "true"

							node.SetLabels(labels)

							err = c.setX100NodeLabels(&node, labels, x100TeardownCompleted, LabelUpdateAdditionType)
							if err != nil {
								log.Log.Info("Unable to label node %s with %s", node.ObjectMeta.Name, x100TeardownCompleted)
								break
							}
						}

						log.Log.Info(fmt.Sprintf("[Reboot] Enabling Pod selector labels for node %s under policy %s",
							node.ObjectMeta.Name, policy.ObjectMeta.Name))
						err = c.enablePodSelectorLabels(&node)
						if err != nil {
							break
						}
						log.Log.Info(fmt.Sprintf("[Reboot] Enabled pod selector labels for node %s after reboot", node.ObjectMeta.Name))
						hasNodeQualifiedForResourceCreationStage = true
					} else {
						/***
							Handle policy deletion triggers here
							Use priorCR values
								If empty, no policy to fallback to
								Else, fall back to priorCR
							Policy deletion would trigger cleanup of labels
							How do we decide that we need to keep priorCR
							Give a field to indicate complete deletion of all policies

							For deletion case, all labels except priorCR will be deleted if non Empty
							For upgrade/downgrade case, we need to delete older resources
							and create new ones as per new policy
							For upgrade
								Delete the node from existing policy that is not current
								Wait for existing pod termination
								enablePodSelectorLabels and other labels for new pods creation
						***/
						log.Log.Info(fmt.Sprintf("Node %s has an activeCR label", node.ObjectMeta.Name))
						if hasX100UpgradeFailedLabel(labels) || hasX100RollingBackUpgradeLabel(labels) {
							// handle only during rollback handler
							cardState, _ := c.rollbackHandler(policy, &node)
							if cardState == xcardv1.NotOperational {
								break
							}
						}
						policyopts := []client.ListOption{}
						policylist := &xcardv1.X100ManagementPolicyList{}
						err := c.rec.List(context.TODO(), policylist, policyopts...)
						if err != nil {
							log.Log.Error(err, "labelX100NodeswithCR()- Unable to list X100ManagementPolicy CRs")
						}

						for _, cr := range policylist.Items {
							activeCR, err := getActiveCRonNode(labels)
							if err != nil {
								log.Log.Info(fmt.Sprintf("Active CR not found on node %s", node.ObjectMeta.Name))
								break
							}
							if cr.ObjectMeta.GetName() == activeCR && policy.ObjectMeta.Name != cr.ObjectMeta.GetName() {
								//Fetch relevant policy for node and see if its not current policy
								//delete the Node from cr.Spec.NodeSelector[]
								//because it is part of current policyInstance that triggered reconciler
								if !hasX100AggregationBlockedLabel(labels) {
									pnodes := cr.Spec.NodeSelector
									var index int
									for i, pnode := range pnodes {
										if pnode == node.ObjectMeta.Name {
											index = i
											break
										}
									}
									cr.Spec.NodeSelector = append(cr.Spec.NodeSelector[:index], cr.Spec.NodeSelector[index+1:]...)
									c.rec.Update(context.TODO(), &cr)
									if err != nil {
										log.Log.Info(fmt.Sprintf("Unable to remove node %s from exisiting policy : %s",
											cr.ObjectMeta.GetName(), node.ObjectMeta.Name))
										break
									}
									log.Log.Info(fmt.Sprintf("Removed node %s from %s",
										node.ObjectMeta.Name, cr.ObjectMeta.Name))

									//Mark label aggregation as blocked on this node
									//Store the correct policy name as value
									labels[x100NodeAggregationBlocked] = policy.GetName()
									node.SetLabels(labels)
									err = c.setX100NodeLabels(&node, labels, x100NodeAggregationBlocked, LabelUpdateAdditionType)
									if err != nil {
										log.Log.Info(fmt.Sprintf("Unable to add label %s to node %s",
											x100NodeAggregationBlocked, node.ObjectMeta.Name))
										break
									}
								} else {
									log.Log.Info(fmt.Sprintf("Node %s will be excluded from aggregation while activeCR doesn't change",
										node.ObjectMeta.Name))
								}
								log.Log.Info(fmt.Sprintf("Running upgrade sequence for node %s", node.GetName()))
								err = c.runUpgradeSequenceForNode(&node)
								if err != nil {
									break
								}
								hasNodeQualifiedForResourceCreationStage = true
							} else {
								// Check for upgrading labels here
								hasUpgradingLabel := hasX100UpgradingLabel(labels)
								if hasUpgradingLabel {
									err = c.runUpgradeSequenceForNode(&node)
									if err != nil {
										break
									}
									hasNodeQualifiedForResourceCreationStage = true
								}
								hasRolledBackLabel := hasX100RolledBackLabel(labels)
								if hasRolledBackLabel {
									rnode := &corev1.Node{}
									nodeUpdated := false
									// Remove stale labels, keep rolledback label to fall back on enable selectors
									if hasX100BootupSuccessLabel(labels) {
										for _, label := range x100StatusLabels {
											if _, ok := labels[label]; ok {
												delete(labels, label)
											}
										}
										node.SetLabels(labels)
										//err = c.rec.Update(context.TODO(), &node)
										err = c.setX100NodeLabels(&node, labels, x100RolledBack, LabelUpdateDeletionType)
										if err != nil {
											return fmt.Errorf("Unable to remove label %s from node %s, err %s", x100RolledBack, node.ObjectMeta.Name, err.Error())
										}
										err, rnode = c.fetchUpdatedNodeInstance(&node)
										if err != nil {
											return err
										}
										nodeUpdated = true
									}

									if !nodeUpdated {
										rnode = &node
									}
									// Enable pod selector labels
									err = c.enablePodSelectorLabels(rnode)
									if err != nil {
										break
									}
									hasNodeQualifiedForResourceCreationStage = true
								}
							}
						}
						hasNodeQualifiedForResourceCreationStage = true
					}
				} else {
					// Node doesn't have x100 card attached to it
					// Break from here since no card
					break
				}
			}
		}
	}

	if len(nodeInCRSelectorList) <= 1 {
		// allow the empty policies or single node policies to move ahead to completion
		hasNodeQualifiedForResourceCreationStage = true
	}

	if !hasNodeQualifiedForResourceCreationStage {
		return errors.New(fmt.Sprintf("Policy has no node that can proceed to create resources, flagging policy %s as NotOperational",
			policy.ObjectMeta.Name))
	}
	return nil
}

func (c *ControllerState) labelx100bootupStatusforNodes() xcardv1.State {

	nodesCount := 0
	opts := []client.ListOption{}
	nodes_list := &corev1.NodeList{}
	err := c.rec.List(context.TODO(), nodes_list, opts...)
	if err != nil {
		if kerrors.IsNotFound(err) {
			log.Log.Info("labelx100bootupStatusforNodes: Reached getx100bootupStatus but unable to list any nodes", "Error : ", err.Error())
			return xcardv1.NotOperational
		}
	}
	nodesCount = len(nodes_list.Items)
	log.Log.Info("labelx100bootupStatusforNodes:", "NodesCount", nodesCount)
	for _, node := range nodes_list.Items {
		labels := c.getNodeLabels(node)
		annotations := node.GetAnnotations()

		if isEligible, decrement := c.isX100NodeEligibleForBootStatusCheck(&node); !isEligible {
			nodesCount = nodesCount - decrement
			continue
		}

		if !hasX100HealthCheckStartTimeAnnotation(annotations) {
			tnode, err := c.attachX100Annotation(&node, x100HealthCheckStartTime)
			if err != nil {
				continue
			}
			node = *tnode
			log.Log.Info(fmt.Sprintf("Attached x100HealthCheckStartTime annotation to the node %s while entering labelx100bootupStatusforNodes()",
				node.GetName()))
		}

		x100count, _ := c.getX100CardCountOnNode(&node)
		if x100count > 0 {
			labels = c.getNodeLabels(node)
			labels[x100CountOnNode] = strconv.Itoa(x100count)
			log.Log.Info("labelx100bootupStatusforNodes: X100 Count on Node",
				node.GetName(), x100count)

			successBootCount, failedBootCount, err := c.getX100BootupStatusOnNode(&node, x100count)
			labels[x100BootupSuccessCount] = strconv.Itoa(successBootCount)
			if err != nil {
				log.Log.Info("labelx100bootupStatusforNodes: Unable to get X100 Bootup status for node",
					node.ObjectMeta.Name, err.Error())
				labels[x100BootupSuccess] = "unknown"
			} else {
				log.Log.Info("labelx100bootupStatusforNodes: Bootup success count for node",
					node.ObjectMeta.Name, successBootCount,
					", Failed bootup:", failedBootCount)
				if successBootCount+failedBootCount != x100count {
					labels[x100BootupSuccess] = "unknown"
					log.Log.Info(fmt.Sprintf("Boot status not available for all cards on node %s",
						node.GetName()))
				} else {
					nodesCount--
					if successBootCount == x100count {
						labels[x100BootupSuccess] = "true"
						log.Log.Info(fmt.Sprintf("All cards booted successfully on node %s", node.GetName()))
					} else {
						if failedBootCount > 0 {
							labels[x100BootupFailedCount] = strconv.Itoa(failedBootCount)
							log.Log.Info(fmt.Sprintf("%v cards failed to boot successfully on node %s",
								labels[x100BootupFailedCount], node.GetName()))
						}
						labels[x100BootupSuccess] = "false"
					}
				}
			}

			node.SetLabels(labels)
			err = c.setX100NodeLabels(&node, labels, x100BootupSuccess, LabelUpdateAdditionType)
			//err = c.rec.Update(context.TODO(), &node)
			if err != nil {
				log.Log.Info("labelx100bootupStatusforNodes: Unable to label node", node.ObjectMeta.Name, " with ", x100BootupSuccess, err.Error())
			} else {
				log.Log.Info("labelx100bootupStatusforNodes: X100 Bootup success Count on Node", node.ObjectMeta.Name, successBootCount)
			}
		}
	}
	if nodesCount <= 0 {
		log.Log.Info("labelx100bootupStatusforNodes: All policy nodes processed successfully")
		return xcardv1.Operational
	}
	log.Log.Info("labelx100bootupStatusforNodes: Unable to get bootStatus of all nodes at the moment")
	return xcardv1.NotOperational
}
