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

const (
    x100LabelKey   = "qualcomm.com/x100.present"
    x100LabelValue = "true"
)

var x100NodeLabels = map[string]string{
    "feature.node.kubernetes.io/pci-1200_17cb_0600.present": "true",
    "feature.node.kubernetes.io/pci-1200_17cb_0601.present": "true",
}

// States for the controller state machine
const (
    startState = iota
    iterateAssetsState
    createResourcesState
    endState
)

// Sequence below is in terms of dependencies
// Modify accordingly when adding new assets
var assets = []string{
    "/opt/x100-operator/init-tasks",
    "/opt/x100-operator/nfd-configuration",
    "/opt/x100-operator/firmware",
    "/opt/x100-operator/hw-manager",
    "/opt/x100-operator/kernel-modules",
    "/opt/x100-operator/device-plugin",
}

type ControllerState struct {
    currentState      int
    desiredState      int
    currentAsset      string
    workerNeedsReboot bool
    assets            []string
    x100Policy        *xcardv1.X100ManagementPolicy
    rec               *X100ManagementPolicyReconciler
}

func hasCustomX100Label(labels map[string]string) bool {
    if _, ok := labels[x100LabelKey]; ok {
        //log.Log.Info("Label key matched for ", "key: ", x100LabelKey, "Value: ", labels[x100LabelKey], "Expected:", x100LabelValue)
        if labels[x100LabelKey] == x100LabelValue {
            // node is already labelled
            return true
        }
    }
    return false
}

func hasX100PCILabels(labels map[string]string) bool {

    for key, val := range labels {
        if _, ok := x100NodeLabels[key]; ok {
            //log.Log.Info("Label key matched for ", "key: ", key, "Value: ", val, "Expected: ", x100NodeLabels[key])
            if x100NodeLabels[key] == val {
                //log.Log.Info("Found x100PCILabels")
                return true
            }
        }
    }
    return false
}

func (n *ControllerState) labelX100Nodes() error {

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

// State Machine
func (c *ControllerState) start(reconciler *X100ManagementPolicyReconciler, policy *xcardv1.X100ManagementPolicy) error {

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
    err := c.labelX100Nodes()
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
