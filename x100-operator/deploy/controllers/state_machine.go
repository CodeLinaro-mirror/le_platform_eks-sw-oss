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
        if !hasCustomLabel && hasPCILabel {
            // label node with the custom label
            labels[x100LabelKey] = x100LabelValue
            node.SetLabels(labels)
            err = n.rec.Update(context.TODO(), &node)
            if err != nil {
                return fmt.Errorf("Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100LabelKey, err.Error())
            }
        } else if hasCustomLabel && !hasPCILabel {
            // previously labelled node and no longer has GPU's
            // reset the custom label as it is not valid
            labels[x100LabelKey] = "false"
            node.SetLabels(labels)
            err = n.rec.Update(context.TODO(), &node)
            if err != nil {
                return fmt.Errorf("Unable to reset node label for %s with %s, err %s", node.ObjectMeta.Name, x100LabelKey, err.Error())
            }
        }
    }
    return nil
}

func (c *ControllerState) labelX100NodeswithCRDFields(policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {

    // fetch all nodes in the cluster
    log.Log.Info("Entering labelX100NodeswithCRDFields() ")
    opts := []client.ListOption{}
    list := &corev1.NodeList{}
    err := c.rec.List(context.TODO(), list, opts...)
    if err != nil {
        return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
    }
    nodeSelectorsListfromcrd := policySpec.NodeSelectors
    log.Log.Info("labelX100NodeswithCRDFields()- ","Nodes list length from CRD- ",len(nodeSelectorsListfromcrd))
    if len(nodeSelectorsListfromcrd) == 0 {
        for _, node := range list.Items {
            // get node labels
            labels := node.GetLabels()
            hasCustomLabel := hasCustomX100Label(labels)
	    //Below fn call will return true if a defaultSW version(label present and value is true) is running on the node.
	    isrunningNonDefaultSW_flag := isrunningNonDefaultSW(labels)
            if hasCustomLabel && !isrunningNonDefaultSW_flag {
                // label node with the custom label
                labels[x100activecrdKey] = policy.ObjectMeta.Name
                labels[x100swversionKey] = policySpec.SwVersion
	        if len(nodeSelectorsListfromcrd) == 0 {
                    labels[x100swvdefaultKey] = x100swvdefaultValue
	        }
                node.SetLabels(labels)
                err = c.rec.Update(context.TODO(), &node)
                if err != nil {
                    return fmt.Errorf("Default- Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100activecrdKey, err.Error())
                }
            }
        }
    } else {
        log.Log.Info("labelX100NodeswithCRDFields()- Non Default CR which has nodeSelector entries")
        for _, node := range list.Items {
            for _, ns := range nodeSelectorsListfromcrd {
	       if node.ObjectMeta.Name == ns {
                    labels := node.GetLabels()
		    hasCustomLabel := hasCustomX100Label(labels)
		    isrunningDefaultSW_flag := isrunningDefaultSW(labels)
		    isrunningNonDefaultSW_flag := isrunningNonDefaultSW(labels)
		    if hasCustomLabel && !isrunningNonDefaultSW_flag {
		        if isrunningDefaultSW_flag {
			   delete(labels, x100swvdefaultKey)
			}
                        labels[x100activecrdKey] = policy.ObjectMeta.Name
			labels[x100swversionKey] = policySpec.SwVersion
			labels[x100swvNondefaultKey] = x100swvNondefaultValue
			node.SetLabels(labels)
			err = c.rec.Update(context.TODO(), &node)
			if err != nil {
                            return fmt.Errorf("Non-Default- Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100activecrdKey, err.Error())
                        }
		    }
	       }
	    }
	}
    }
    return nil
}


// State Machine
func (c *ControllerState) start(reconciler *X100ManagementPolicyReconciler, policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {

    // Initializate the ControllerState
    c.currentState = startState
    c.currentAsset = ""
    c.assets = assets
    c.x100Policy = policy
    c.rec = reconciler
    c.workerNeedsReboot = false

    // Initialize the AssetMap
    AssetMap = map[string]Asset{}

    //fetch all nodes and label x100 nodes
    err := c.labelX100Nodes(policy, policySpec)
    if err != nil {
        return err
    }

    // Transition to the Iterate Assets state
    c.desiredState = iterateAssetsState
    return nil
}

// Iterate over the assets
func (c *ControllerState) iterateAssets() (int, error) {
    assetLength := len(c.assets)
    // Check if there are any more assets to iterate over
    if assetLength == 0 {
        // No assets left, transition to end state
        return endState, nil
    } else {
        // More assets, set the current asset and transition to Create Resources
        c.currentAsset = c.assets[0]
        // Trim assets to exclude the assets before current asset
        c.assets = c.assets[1:]
        return createResourcesState, nil
    }
}

func (c *ControllerState) stateMachineCompleted() bool {
    if len(c.assets) == 0 {
        return true
    }
    return false
}

// Create resources for the current asset
func (c *ControllerState) createResources() (int, xcardv1.State, error) {
    // Create the resources for the current asset
    // Errors in this part should be handled inside createAssetMap by early exit
    _ = createAssetMap(c.currentAsset)

    // Create the resources using  resources from AssetMap and creation callbacks
    asset := AssetMap[c.currentAsset]

    result := xcardv1.Operational
    for _, robj := range asset.objectMappings {
        isHwMgrPodReady("app", "csm-x100hwmgr", *c, "Running")
        cardstate, err := createKindResource(*c, robj.key, robj.value)
        if err != nil {
            return endState, cardstate, err
        }
        if cardstate != xcardv1.Operational {
            result = xcardv1.NotOperational
        }
    }

    // Transition to the iterate assets state
    return iterateAssetsState, result, nil
}

func (c *ControllerState) endState() (xcardv1.State, error) {
    // Terminate, cleanup
    return xcardv1.Operational, nil
}

func (c *ControllerState) triggerStateMachine(reconciler *X100ManagementPolicyReconciler, policy *xcardv1.X100ManagementPolicy) (xcardv1.State, error) {
    var err error
    var desiredState int
    var xcardStatus xcardv1.State
    var cardState xcardv1.State

    for {
        switch c.desiredState {
        case iterateAssetsState:
            desiredState, err = c.iterateAssets()
            if err != nil {
                return xcardv1.NotOperational, err
            }
            c.desiredState = desiredState
        case createResourcesState:
            desiredState, cardState, err = c.createResources()
            if err != nil {
                return cardState, err
            }

            if cardState != xcardv1.Operational {
                return cardState, nil
            }
            c.desiredState = desiredState
        case endState:
            xcardStatus, err = c.endState()
            if err != nil {
                return xcardv1.NotOperational, err
            } else {
                return xcardStatus, nil
            }
        }
    }
}
