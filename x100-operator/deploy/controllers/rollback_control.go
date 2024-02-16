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
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"
    xcardv1 "x100-operator/api/v1"
    corev1 "k8s.io/api/core/v1"

    "context"
    "fmt"
)

func (c *ControllerState) rollbackHandler(policy *xcardv1.X100ManagementPolicy) (xcardv1.State, error) {
    //check if current CR has rollback enabled. if no skip to next CR.
    //if yes,  get all nodes that are running with current CR(active-cr). 
    //if a nodes bootsuccess label is false/unknown && bootstatusmarked(true). get the prior-cr name.
    //if prior-cr available(both label and cr object), delete the swversion, active-cr, all x100 labels on the node.
    //labels[x100RollingBack] = "true"?, set active-cr=prior, delete prior-cr label
    //wait for pods to go down on the node
    //get the CR object(prior one) and add the current node to NS[](Check if the fault CR gets over-written to Dummy-node)
    //set the swversion to prior-cr's swversion
    //based on the swversion && len(NodeSelectors[]) -> set x100swvNondefaultKey/x100swvdefaultKey
    //?get the current CR Object in the end and remove the node(last-step).
    log.Log.Info("rollbackHandler() - Entered()")
    if policy.Spec.EnableRollback != true {
        log.Log.Info("Rollback Disabled on the current CR, Returning!")
        log.Log.Info("rollbackHandler() Exited()")
        return xcardv1.Operational, nil
    } else {
        nopts := []client.ListOption{}
        nlist := &corev1.NodeList{}
        err := c.rec.List(context.TODO(), nlist, nopts...)
        if err != nil {
            log.Log.Info("rollbackHandler() Exited()")
            return xcardv1.NotOperational, fmt.Errorf("rollbackHandler(): Unable to list nodes for rollback, err %s", err.Error())
        }
        currCrRollback_nlist := &corev1.NodeList{}
        for _, node := range nlist.Items {
            labels := node.GetLabels()
            RunningwithcurrCR_flag := isRunningwithcurrCR(labels, policy.ObjectMeta.Name)
            BootSuccess_status     := hasX100BootupSuccessLabel(labels)
            BootStatus_marked      := isX100BootupStatusMarkedLabelTrue(labels)
            if !RunningwithcurrCR_flag || !BootStatus_marked || BootSuccess_status {
                continue
            } else {
                currCrRollback_nlist.Items = append(currCrRollback_nlist.Items, node)
                log.Log.Info(fmt.Sprintf("Rollback Needed for Node: [%s]",node.ObjectMeta.Name))
            }
        }
        for _, rnode := range currCrRollback_nlist.Items {
            labels := rnode.GetLabels()
            priorCr,priorCrAvailable := hasPriorCrLabel(labels)
            if !priorCrAvailable {
                log.Log.Info(fmt.Sprintf("Rollback Needed,But PriorCr not available for Node: [%s], Skipping!",rnode.ObjectMeta.Name))
                continue
            } else {
                cropts := []client.ListOption{}
                crlist := &xcardv1.X100ManagementPolicyList{}
                err  := c.rec.List(context.TODO(), crlist, cropts...)
                if err != nil {
                    log.Log.Error(err, "rollbackHandler()- Unable to list X100ManagementPolicy CRs")
                }
                priorCrInstanceAvailable := false
                priorCrInstance := &xcardv1.X100ManagementPolicy{}
                currentCrInstance := &xcardv1.X100ManagementPolicy{}
                for _, pcr := range crlist.Items {
                    if pcr.ObjectMeta.GetName() == priorCr {
                        priorCrInstanceAvailable = true
                        priorCrInstance = &pcr
                        log.Log.Info("rollbackHandler(): Prior CR Found")
                        break
                    }
                }
                for _, ccr := range crlist.Items {
                    if ccr.ObjectMeta.GetName() == policy.ObjectMeta.Name {
                        currentCrInstance = &ccr
                        log.Log.Info("rollbackHandler(): Fetched current CR")
                        break
                    }
                }
                if priorCrInstanceAvailable {
                    if priorCrInstance.Spec.SwVersion != "default" {
                        priorCrInstance.Spec.NodeSelectors = append(priorCrInstance.Spec.NodeSelectors, rnode.ObjectMeta.Name)
                        c.rec.Update(context.TODO(), priorCrInstance)
                    }
                    nopts := []client.ListOption{ client.MatchingFields{"metadata.name": rnode.ObjectMeta.Name }}
                    tempnodes := &corev1.NodeList{}
                    err = c.rec.List(context.TODO(), tempnodes, nopts...)
                    if err != nil {
                        log.Log.Info("rollbackHandler(): Could not get Latest Node object", err,"--")
                    }
                    tempnode := tempnodes.Items[0]
                    clabels := tempnode.GetLabels()
                    delete(clabels, x100CountOnNodekey)
                    delete(clabels, x100BootupSuccess)
                    delete(clabels, x100BootupSuccessCount)
                    delete(clabels, x100BootupFailedCount)
                    delete(clabels, x100HwMgrRunning)
                    delete(clabels, x100BootupStatusMarked)
                    delete(clabels, x100swversionKey)
                    clabels[x100activecrdKey] = priorCr
                    tempnode.SetLabels(clabels)
                    err = c.rec.Update(context.TODO(), &tempnode)
                    if err != nil {
                        log.Log.Info("rollbackHandler()- Unable to set labels on node %s , err %s", tempnode.ObjectMeta.Name, err.Error())
                    }
                    c.waitForKmmPodTermination(&tempnode)
                    c.waitForFwPodTermination(&tempnode)
                    c.waitForHwPodTermination(&tempnode)
                    nopts = []client.ListOption{ client.MatchingFields{"metadata.name": rnode.ObjectMeta.Name }}
                    tempnodes = &corev1.NodeList{}
                    err = c.rec.List(context.TODO(), tempnodes, nopts...)
                    if err != nil {
                        log.Log.Info("rollbackHandler() Could not get Latest Node", err,"--")
                    }
                    tempnode = tempnodes.Items[0]
                    clabels = tempnode.GetLabels()
                    clabels[x100swversionKey] = priorCrInstance.Spec.SwVersion
                    clabels[x100RolledBack] = "true"
                    if priorCrInstance.Spec.SwVersion != "default" {
                        clabels[x100swvNondefaultKey] = x100swvNondefaultValue
                    } else {
                        clabels[x100swvdefaultKey] = x100swvdefaultValue
                    }
                    delete(clabels, x100PriorCr)
                    tempnode.SetLabels(clabels)
                    err = c.rec.Update(context.TODO(), &tempnode)
                    if err != nil {
                        log.Log.Info("rollbackHandler() Unable to label node %s, err %s", tempnode.ObjectMeta.Name, err.Error())
                    }
                    //delete the Node from policyObject.Spec.NodeSelectors[]
                    pnodes := currentCrInstance.Spec.NodeSelectors
                    var ind int
                    for i, pnode := range pnodes {
                        if pnode == rnode.ObjectMeta.Name {
                            ind = i
                            break
                        }
                    }
                    //currentCrInstance.Spec.NodeSelectors[ind] = "Dummy-node"
                    //Deletion of node from NodeSelectors list should be done only on Non Default
                    log.Log.Info("rollbackHandler() Deleting node from priorCrInstance.Spec.NS[] at index:",ind, rnode.ObjectMeta.Name)
                    currentCrInstance.Spec.NodeSelectors = append(currentCrInstance.Spec.NodeSelectors[:ind], currentCrInstance.Spec.NodeSelectors[ind+1:]...)
                    c.rec.Update(context.TODO(), currentCrInstance)
                } else {
                    log.Log.Info("rollbackHandler() priorCrInstanceAvailable False, Skipping rollback")
                }
            }
        }
        log.Log.Info("rollbackHandler() Exited()")
        return xcardv1.Operational, nil
    }
}
