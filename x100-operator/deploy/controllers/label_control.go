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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	xcardv1 "x100-operator/api/v1"

	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

func (c *ControllerState) labelX100NodeswithCR(policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {
	/***
		If policy node list is empty
			apply only on nodes that have no activeCR or are new
			Push processing to below loop and handle in else part

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
						Reapply appropraite labels to track new CR
				else
					Must be first time boot up or first time policy application
					fill the labels priorCR, activeCR, SwVersion
					priorCR should be kept empty

		For deletion, we may have to put a condition based on whether priorCR is empty
		After deletion of policy, all labels will be deleted, we could retain priorCr if nonEmpty
		We hit reconciler due to this label change again, but policy instance may be different now
		So how do we ensure that we need to apply priorCR now
		Put an if condition to check if priorCR exists and has value but activeCR is none
		This could indicate coming from deletion fallback path
		Now apply appropriate labels and create new resources
		Fetch policy instance based on the name if needed, and get sw version from it
		We may need to retain sw version as well to avoid the policy pull

		Note : Sequence for creation of resources is maintained through the
		labels created by the dependency and the dependent checks for this
		through a label. Create label on pod ready condition.
		Make sure to modify the podReady calls as well to streamline with This
		new implementation.
		For fwpod, ensure that a file is created by the script.
		Operator checks for this file before flagging fwPodReady condition
		runCommandOnPod() can be used to read the file

		Existing node selectors

		  nodeSelector:
			qualcomm.com/x100.present: "true"
			qualcomm.com/x100.swvdefault: "true"
			qualcomm.com/x100.swversion: default
		New node selectors
		    qualcomm.com/x100.present: "true"
			qualcomm.com/x100.swversion: "v1"
			qualcomm.com/x100.fw.present: "true"

			individual pod related labels can be used
			to control creation and termination.

	***/
	log.Log.Info("Entering labelX100NodeswithCR()")
	nodeInCRSelectorList := policySpec.NodeSelector
	log.Log.Info("labelX100NodeswithCR() ", "Nodes list length from CRD ", len(nodeInCRSelectorList))

	if len(nodeInCRSelectorList) == 0 {
		log.Log.Info(fmt.Sprintf("Selector list is empty for policy %s, applying to all nodes under no policy", policy.ObjectMeta.Name))
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
			labels := node.GetLabels()
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
	// nodesToBeIncludedInPolicy := &corev1.NodeList{}
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
		labels := node.GetLabels()
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
	// We can process all nodes now to apply appropriate policy/software version labels

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
		all nodeSelectorList across nodes.
		For any policy change though, we would have the correct policy instance.
	***/
	nodeInCRSelectorList = policySpec.NodeSelector
	log.Log.Info("Refetching ", "Nodes list length from CRD ", len(nodeInCRSelectorList))

	nodeNeedsRollback := false
	for _, ns := range nodeInCRSelectorList {
		//nodeHasNoX100Card := false
		for _, node := range list.Items {
			// Process only nodes that are in the policy nodeSelectorList
			if node.ObjectMeta.Name == ns {
				log.Log.Info(fmt.Sprintf("Found node object for node %s in selector list", node.ObjectMeta.Name))
				// get node labels
				labels := node.GetLabels()
				hasx100Attached := hasCustomX100Label(labels)
				nodehasNoActiveCRLabel := !hasActiveCRLabel(labels)
				nodeEnablingPodSelectors := hasX100EnablingFirstPolicy(labels)

				if hasx100Attached {
					// Processing worker nodes that have x100 card attached to them
					if nodehasNoActiveCRLabel {
						log.Log.Info(fmt.Sprintf("Processing node %s for no activeCR label", node.ObjectMeta.Name))
						// first time policy application to the node
						labels[x100ActiveCR] = policy.ObjectMeta.Name
						labels[x100SwVersion] = policySpec.SwVersion
						labels[x100PriorCR] = "" // No priorCR
						nodehasOwnerPolicyDeletedLabel := hasX100OwnerPolicyDeletedLabel(labels)
						if nodehasOwnerPolicyDeletedLabel {
							delete(labels, x100OwnerPolicyDeleted)
						}
						// Acts as a flag to identify that we need to reenter this loop
						if !hasX100EnablingFirstPolicy(labels) {
							labels[x100EnablingFirstPolicy] = "true"
						}
						// Enable individual pod selector labels to control deletion sequence
						err = c.enablePodSelectorLabels(&node)
						if err != nil {
							return err
						}
						// Fetch updated node Instance
						log.Log.Info(fmt.Sprintf("Labelling node %s with activeCR label", node.ObjectMeta.Name))
						node.SetLabels(labels)
						err = c.rec.Update(context.TODO(), &node)
						if err != nil {
							return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100ActiveCR, err.Error())
						}
					} else if nodeEnablingPodSelectors {
						// Enable individual pod selector labels to control creation sequence
						log.Log.Info(fmt.Sprintf("Enabling Pod selector labels for node %s under policy %s",
							node.ObjectMeta.Name, policy.ObjectMeta.Name))
						err = c.enablePodSelectorLabels(&node)
						if err != nil {
							// allow the process to move ahead with daemonset creation
							// instead of returning an error which could result in
							// useless looping and no progress
							return nil
						}
						/***
						log.Log.Info("Marking Pod selector enablement as complete...")
						// Remove the label
						rnode := &corev1.Node{}
						err, rnode = c.fetchUpdatedNodeInstance(&node)
						if err != nil {
							return err
						}
						labels = rnode.GetLabels()
						delete(labels, x100EnablingFirstPolicy)

						node.SetLabels(labels)
						err = c.rec.Update(context.TODO(), &node)
						if err != nil {
							return fmt.Errorf("Unable to delete label node %s with %s, err %s",
												node.ObjectMeta.Name, x100EnablingFirstPolicy, err.Error())
						}
						***/
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
								nodeNeedsRollback = true
							}
							break
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
								return err
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
										return fmt.Errorf("Unable to remove node %s from exisiting policy : %s. err %s",
											cr.ObjectMeta.GetName(), node.ObjectMeta.Name, err.Error())
									}
									log.Log.Info(fmt.Sprintf("Removed node %s from %s",
										node.ObjectMeta.Name, cr.ObjectMeta.Name))

									//Fetch updated node instance and label aggregation as blocked on this node
									// Todo What if the label attachment fails, how to fallback
									labels[x100NodeAggregationBlocked] = "true"
									node.SetLabels(labels)
									err = c.rec.Update(context.TODO(), &node)
									if err != nil {
										return fmt.Errorf("Unable to remove label %s from node %s, err %s",
											x100NodeAggregationBlocked, node.ObjectMeta.Name, err.Error())
									}
								} else {
									log.Log.Info(fmt.Sprintf("Node %s will be excluded from aggregation while activeCR doesn't change",
										node.ObjectMeta.Name))
								}
								log.Log.Info(fmt.Sprintf("Running upgrade sequence for node %s", node.GetName()))
								err = c.runUpgradeSequenceForNode(&node)
								if err != nil {
									return err
								}
							} else {
								// Check for upgrading labels here
								hasUpgradingLabel := hasX100UpgradingLabel(labels)
								if hasUpgradingLabel {
									err = c.runUpgradeSequenceForNode(&node)
									if err != nil {
										return err
									}
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
										err = c.rec.Update(context.TODO(), &node)
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
										return err
									}
								}
							}
						}
					}
				} else {
					// Node doesn't have x100 card attached to it
					// Break from here and then the outer loop since no card
					// nodeHasNoX100Card = true
					break
				}
			}
		}
	}
	if nodeNeedsRollback {
		return errors.New(fmt.Sprintf("Node needs a rollback, flagging policy %s as NotOperational", policy.ObjectMeta.Name))
	}
	return nil
}

func (n *ControllerState) labelX100Nodes(policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {

	// fetch all nodes in the cluster
	log.Log.Info("Entering labelX100Nodes() ")
	opts := []client.ListOption{}
	list := &corev1.NodeList{}
	err := n.rec.List(context.TODO(), list, opts...)
	if err != nil {
		return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
	}

	for _, node := range list.Items {
		// get node labels
		labels := node.GetLabels()
		hasCustomLabel := hasCustomX100Label(labels)
		hasPCILabel := hasX100PCILabels(labels)
		isNodeSchedulable := !n.isNodeUnschedulable(&node)

		if !hasCustomLabel && hasPCILabel && isNodeSchedulable {
			// label node with the custom label
			labels[x100LabelKey] = x100LabelValue
			node.SetLabels(labels)
			err = n.rec.Update(context.TODO(), &node)
			if err != nil {
				return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100LabelKey, err.Error())
			}
			log.Log.Info(fmt.Sprintf("Label %s set to true", x100LabelKey))
		} else if hasCustomLabel && !hasPCILabel {
			// previously labelled node and no longer has X100s'
			// reset the custom label as it is not valid
			labels[x100LabelKey] = "false"
			node.SetLabels(labels)
			err = n.rec.Update(context.TODO(), &node)
			if err != nil {
				return fmt.Errorf("Unable to reset node label for %s with %s, err %s", node.ObjectMeta.Name, x100LabelKey, err.Error())
			}
			log.Log.Info(fmt.Sprintf("Stale Label %s set to false", x100LabelKey))
		}
	}
	return nil
}

func (c *ControllerState) labelx100bootupStatusforNodes() xcardv1.State {

	nodesCount := 0
	for {
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
			labels := node.GetLabels()
			if !isX100RunningWithPolicy(labels, c.x100Policy.ObjectMeta.Name) {
				nodesCount--
				log.Log.Info("labelx100bootupStatusforNodes:", "not running with current CR: Removing Node ", node.GetName())
				continue
			} else {
				// Nodes under current policy
				if hasX100BootupSuccessLabel(labels) {
					nodesCount--
					log.Log.Info("labelx100bootupStatusforNodes:", " hasX100BootupSuccess label: Removing Node ", node.GetName())
					continue
				} else if !hasCustomX100Label(labels) {
					nodesCount--
					log.Log.Info("labelx100bootupStatusforNodes:", " doesn't hasCustomX100Label label: Removing Node ", node.GetName())
					continue
				} else if hasX100UpgradeFailedLabel(labels) {
					nodesCount--
					log.Log.Info("labelx100bootupStatusforNodes:", " hasX100UpgradeFailed label: Removing Node ", node.GetName())
					continue
				} else if hasX100BootupStatusMarkedLabel(labels) {
					nodesCount--
					log.Log.Info("labelx100bootupStatusforNodes:", " hasX100BootupStatusMarked label: Removing Node ", node.GetName())
					continue
				}
			}

			x100count, err := c.getX100CardCountOnNode(&node)
			if x100count > 0 {
				labels = node.GetLabels()
				labels[x100CountOnNode] = strconv.Itoa(x100count)
				node.SetLabels(labels)
				err = c.rec.Update(context.TODO(), &node)

				if err != nil {
					log.Log.Info("labelx100bootupStatusforNodes: Unable to label node", node.ObjectMeta.Name, " with ", x100CountOnNode, err.Error())
				} else {
					log.Log.Info("labelx100bootupStatusforNodes: X100 Count on Node", node.ObjectMeta.Name, x100count)
				}

				successBootCount, failedBootCount, err := c.getX100BootupStatusOnNode(&node, x100count)
				labels = node.GetLabels()
				if err != nil {
					log.Log.Info("labelx100bootupStatusforNodes: Unable to get X100 Bootup status for node", node.ObjectMeta.Name, err.Error())
					labels[x100BootupSuccess] = "unknown"
					labels[x100BootupSuccessCount] = strconv.Itoa(successBootCount)
				} else if successBootCount+failedBootCount != x100count {
					log.Log.Info("labelx100bootupStatusforNodes: Success X100 Bootup for node", node.ObjectMeta.Name, successBootCount, ", Failed bootup:", failedBootCount)
					labels[x100BootupSuccess] = "unknown"
					labels[x100BootupSuccessCount] = strconv.Itoa(successBootCount)
				} else {
					nodesCount--
					if successBootCount == x100count {
						labels[x100BootupSuccess] = "true"
					}
					labels[x100BootupSuccessCount] = strconv.Itoa(successBootCount)
					if failedBootCount > 0 {
						labels[x100BootupFailedCount] = strconv.Itoa(failedBootCount)
						labels[x100BootupSuccess] = "false"
					}
				}

				node.SetLabels(labels)
				err = c.rec.Update(context.TODO(), &node)
				if err != nil {
					log.Log.Info("labelx100bootupStatusforNodes: Unable to label node", node.ObjectMeta.Name, " with ", x100BootupSuccess, err.Error())
				} else {
					log.Log.Info("labelx100bootupStatusforNodes: X100 Bootup success Count on Node", node.ObjectMeta.Name, successBootCount)
				}
			}
		}
		if nodesCount <= 0 {
			log.Log.Info("labelx100bootupStatusforNodes: Returning")
			return xcardv1.Operational
		}
		log.Log.Info("labelx100bootupStatusforNodes: Sleeping 10 before retrying to get BootStatus of nodes")
		time.Sleep(time.Second * 10)
	}
}
