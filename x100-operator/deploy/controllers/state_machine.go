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

    "sigs.k8s.io/controller-runtime/pkg/log"
    xcardv1 "x100-operator/api/v1"

    "time"
)

// State Machine
func (c *ControllerState) start(reconciler *X100ManagementPolicyReconciler, policy *xcardv1.X100ManagementPolicy, policySpec *xcardv1.X100ManagementPolicySpec) error {

    log.Log.Info("c *ControllerState start() Entered() for", " CR: ", policy.ObjectMeta.Name)
    // Initializate the ControllerState
    c.currentState = startState
    c.currentAsset = ""
    c.assets = assets
    c.x100Policy = policy
    c.rec = reconciler
    // Initialize the AssetMap
    AssetMap = map[string]Asset{}

    //fetch all nodes and label x100 nodes
    err := c.labelX100Nodes(policy, policySpec)
    if err != nil {
        return err
    }
    //Remove the invalid nodes from NodeSelector CR list from a non-default.(expectation is any new node added to the cluster should run only default, and avoid the ambiguity of v1/v2)
    err = c.removeInvalidNodesfromCR(policy)
    if err != nil {
        return err
    }
    err = c.labelX100NodeswithCRDFields(policy, policySpec)
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
        //return endState, nil
        return getBootStatusState, nil
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
            return earlyExitState, cardstate, err
            //return getBootStatusState, cardstate, err
        }
        if cardstate != xcardv1.Operational {
            result = xcardv1.NotOperational
        }
    }

    // Transition to the iterate assets state
    return iterateAssetsState, result, nil
}

func (c *ControllerState) getx100bootupStatus() (xcardv1.State, error) {
    result := make(chan xcardv1.State, 1)
    go func() {
        result <- c.labelx100bootupStatusforNodes()
    }()
    select {
    case <-time.After(300 * time.Second):
        //Label the timeout and appropriate status for nodes
        log.Log.Info("ERROR: labelx100bootupStatusforNodes() Timed Out")
        c.markHealthCheckedforCurrentCR()
        log.Log.Info("ERROR:", "Reading Health Status from Nodes timed out- CR: ", c.x100Policy.ObjectMeta.Name)
        return xcardv1.Operational, nil
    case result := <-result:
        c.markHealthCheckedforCurrentCR()
        return result, nil
    }
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
            //log.Log.Info("ERROR: triggerStateMachine - iterateAssetsState")
            desiredState, err = c.iterateAssets()
            if err != nil {
                return xcardv1.NotOperational, err
            }
            c.desiredState = desiredState
        case createResourcesState:
            //log.Log.Info("ERROR: triggerStateMachine - createResourcesState")
            desiredState, cardState, err = c.createResources()
            if err != nil {
                return cardState, err
            }
            if cardState != xcardv1.Operational {
                return cardState, nil
            }
            c.desiredState = desiredState
        case getBootStatusState:
            //log.Log.Info("ERROR: triggerStateMachine - getBootStatusState")
            cardState, err = c.getx100bootupStatus()
            if cardState != xcardv1.Operational {
                return cardState, err
            }
            c.desiredState = rollbackState
        case rollbackState:
            //log.Log.Info("ERROR: triggerStateMachine - rollbackState")
            cardState, err = c.rollbackHandler(policy)
            if cardState != xcardv1.Operational {
                return cardState, err
            }
            c.desiredState = endState
        case endState:
            //log.Log.Info("ERROR: triggerStateMachine - endState")
            xcardStatus, err = c.endState()
            if err != nil {
                return xcardv1.NotOperational, err
            } else {
                return xcardStatus, nil
            }
        case earlyExitState:
            //log.Log.Info("ERROR: triggerStateMachine - earlyExitState")
            xcardStatus, err = xcardv1.NotOperational, nil
            if err != nil {
                return xcardv1.NotOperational, err
            } else {
                return xcardStatus, nil
            }
        }
    }
}
