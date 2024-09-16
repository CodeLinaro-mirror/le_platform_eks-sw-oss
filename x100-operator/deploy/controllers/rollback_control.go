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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	xcardv1 "x100-operator/api/v1"
)

func (c *ControllerState) rollbackHandler(policy *xcardv1.X100ManagementPolicy, rnode *corev1.Node) (xcardv1.State, error) {
	/***
		For given node if rollback is enabled
			Proceed if node belongs to current policy and has a priorCR
				Fetch policyInstance for the priorCR
				Attach rollingBackUpgrade label and remove status labels from node
				teminateX100PolicyOwnerPodsOnNode
				Move the node from activeCR list to priorCR list
				**add handling to avoid procesing nodes in label
				  calls that have this rollingbackupgrade label
				put priorCR into activeCR, make priorCR empty, add swVersion using priorCR instance
				Enable Pod selector labels
				Remove the rollingbackupgrade and put rolledBack

	***/
	log.Log.Info(fmt.Sprintf("Executing rollbackHandler on nodes in %s policy", policy.ObjectMeta.Name))

	if policy.Spec.EnableRollback != true {

		log.Log.Info(fmt.Sprintf("Rollback disabled on %s policy", policy.ObjectMeta.Name))
		return xcardv1.Operational, nil

	} else {
		labels := c.getNodeLabels(*rnode)
		isNodeUnderCurrentPolicy := isX100RunningWithPolicy(labels, policy.ObjectMeta.Name)
		upgradeFailedOnNode := hasX100UpgradeFailedLabel(labels)
		nodeHasRollingBackUpgradeLabel := hasX100RollingBackUpgradeLabel(labels)

		if isNodeUnderCurrentPolicy && (upgradeFailedOnNode || nodeHasRollingBackUpgradeLabel) {
			log.Log.Info(fmt.Sprintf("Rolling back upgrade on node: %s...", rnode.ObjectMeta.Name))

			priorCR, priorCRAvailable := hasPriorCRLabel(labels)
			if !priorCRAvailable {
				log.Log.Info(fmt.Sprintf("Rollback needed but PriorCR is not available for node: %s, Skipping...", rnode.ObjectMeta.Name))
			} else {
				// List all x100 policies
				cropts := []client.ListOption{}
				crlist := &xcardv1.X100ManagementPolicyList{}
				err := c.rec.List(context.TODO(), crlist, cropts...)
				if err != nil {
					log.Log.Error(err, "Unable to list x100ManagementPolicy during rollbackHandler execution.")
					return xcardv1.NotOperational, err
				}
				priorCRInstanceAvailable := false
				priorCRInstance := &xcardv1.X100ManagementPolicy{}
				for _, pcr := range crlist.Items {
					if pcr.ObjectMeta.GetName() == priorCR {
						priorCRInstanceAvailable = true
						priorCRInstance = &pcr
						log.Log.Info("PriorCR instance found during rollbackHandler execution.")
						break
					}
				}

				if priorCRInstanceAvailable {
					node := &corev1.Node{}
					err = c.transitionToRollingBackUpgradeState(rnode)
					if err != nil {
						log.Log.Info(fmt.Sprintf("Unable to transition to rolling back upgrade, err %s", err.Error()))
						return xcardv1.NotOperational, err
					}

					err, node = c.fetchUpdatedNodeInstance(rnode)
					if err != nil {
						return xcardv1.NotOperational, err
					}

					labels = c.getNodeLabels(*node)
					if !hasX100TeardownCompletedLabel(labels) {
						err = c.teardownX100ManagementPolicyOwnedPodsOnNode(node)
						if err != nil {
							log.Log.Info(fmt.Sprintf("Tearing down of x100ManagementPolicy owned pods is incomplete..., err %s", err.Error()))
							return xcardv1.NotOperational, err
						}
						// Mark teardown sequence completion
						labels[x100TeardownCompleted] = "true"
						node.SetLabels(labels)
						//err := c.rec.Update(context.TODO(), node)
						err := c.setX100NodeLabels(node, labels, x100SwVersion, LabelUpdateAdditionType)
						if err != nil {
							return xcardv1.NotOperational, fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name,
								x100TeardownCompleted, err.Error())
						}
					}

					err, node = c.fetchUpdatedNodeInstance(node)
					if err != nil {
						return xcardv1.NotOperational, err
					}

					// remove PlaceHolderNode node
					pnodes := priorCRInstance.Spec.NodeSelector
					var index int
					for i, pnode := range pnodes {
						if pnode == PlaceHolderNode {
							log.Log.Info(fmt.Sprintf("PlaceHolderNode node found in %s", priorCRInstance.GetName()))
							index = i
							break
						}
					}
					priorCRInstance.Spec.NodeSelector = append(priorCRInstance.Spec.NodeSelector[:index], priorCRInstance.Spec.NodeSelector[index+1:]...)

					priorCRInstance.Spec.NodeSelector = append(priorCRInstance.Spec.NodeSelector, node.ObjectMeta.Name)
					err = c.rec.Update(context.TODO(), priorCRInstance)
					if err != nil {
						log.Log.Info(fmt.Sprintf("Error encountered during rollback while updating node selector list for %s", priorCRInstance.ObjectMeta.Name))
						return xcardv1.NotOperational, err
					}
					log.Log.Info(fmt.Sprintf("Added node %s to policy %s's selector list", node.ObjectMeta.Name, priorCRInstance.ObjectMeta.Name))
					//delete the Node from current policy
					pnodes = policy.Spec.NodeSelector
					var ind int
					for i, pnode := range pnodes {
						if pnode == node.ObjectMeta.Name {
							ind = i
							break
						}
					}
					policy.Spec.NodeSelector = append(policy.Spec.NodeSelector[:ind], policy.Spec.NodeSelector[ind+1:]...)
					if len(policy.Spec.NodeSelector) == 0 {
						policy.Spec.NodeSelector = append(policy.Spec.NodeSelector, PlaceHolderNode)
					}
					c.rec.Update(context.TODO(), policy)
					if err != nil {
						log.Log.Info(fmt.Sprintf("Error encountered during rollback while updating node selector list for %s", policy.ObjectMeta.Name))
						return xcardv1.NotOperational, err
					}
					log.Log.Info(fmt.Sprintf("Removed node %s from policy %s's selector list", node.ObjectMeta.Name, policy.ObjectMeta.Name))

					err, node = c.fetchUpdatedNodeInstance(node)
					if err != nil {
						return xcardv1.NotOperational, err
					}
					labels = c.getNodeLabels(*node)

					labels[x100ActiveCR] = priorCR
					labels[x100SwVersion] = priorCRInstance.Spec.SwVersion
					labels[x100PriorCR] = ""

					node.SetLabels(labels)
					//err = c.rec.Update(context.TODO(), node)
					err = c.setX100NodeLabels(node, labels, x100ActiveCR, LabelUpdateAdditionType)
					if err != nil {
						log.Log.Info("Unable to update activeCR during rollback on node %s , err %s", node.ObjectMeta.Name, err.Error())
						return xcardv1.NotOperational, err
					}
					log.Log.Info(fmt.Sprintf("Updated activeCR to priorCR for rollback on node %s for policy %s", node.ObjectMeta.Name, policy.ObjectMeta.Name))
					if hasX100RollingBackUpgradeLabel(labels) {
						delete(labels, x100RollingBackUpgrade)
						labels[x100RolledBack] = "true"
						node.SetLabels(labels)
						//err = c.rec.Update(context.TODO(), node)
						err = c.setX100NodeLabels(node, labels, x100RolledBack, LabelUpdateAdditionType)
						if err != nil {
							log.Log.Info("Unable to label node %s with rollback completion, err %s", node.ObjectMeta.Name, err.Error())
						}
						log.Log.Info(fmt.Sprintf("Rollback completed on node %s", node.ObjectMeta.Name))
					}

				} else {
					log.Log.Info("priorCrInstance is not available, no policy to rollback to.")
				}
			}
		}
	}
	return xcardv1.Operational, nil
}
