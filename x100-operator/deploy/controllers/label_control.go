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

    kerrors "k8s.io/apimachinery/pkg/api/errors"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"
    xcardv1 "x100-operator/api/v1"
    corev1 "k8s.io/api/core/v1"

    "strconv"
    "context"
    "time"
    "fmt"
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
            // previously labelled node and no longer has X100s'
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

    log.Log.Info("Entering labelX100NodeswithCRDFields() ")
    nodeSelectorsListfromcrd := policySpec.NodeSelector
    log.Log.Info("labelX100NodeswithCRDFields()- ","Nodes list length from CRD- ",len(nodeSelectorsListfromcrd))
    if len(nodeSelectorsListfromcrd) == 0 && policySpec.SwVersion == "default" {
        // fetch all nodes in the cluster
        opts := []client.ListOption{}
        list := &corev1.NodeList{}
        err := c.rec.List(context.TODO(), list, opts...)
        if err != nil {
            return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
        }

        for _, node := range list.Items {
            // get node labels
            labels := node.GetLabels()
            hasCustomLabel := hasCustomX100Label(labels)
            //Below fn call will return true if a defaultSW version(label present and value is true) is running on the node.
            isrunningNonDefaultSW_flag := isrunningNonDefaultSW(labels)
            isRunningwithcurrCR_flag := isRunningwithcurrCR(labels, policy.ObjectMeta.Name)
            if hasCustomLabel && !isrunningNonDefaultSW_flag {
                // label node with the custom label
                if !isRunningwithcurrCR_flag {
                    c.waitForKmmPodTermination(&node)
                    c.waitForFwPodTermination(&node)
                    c.waitForHwPodTermination(&node)
                }
                labels = node.GetLabels()
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
        log.Log.Info("labelX100NodeswithCRDFields()- Non Default CR ")
        // fetch all nodes in the cluster
        opts := []client.ListOption{}
        list := &corev1.NodeList{}
        err := c.rec.List(context.TODO(), list, opts...)
        if err != nil {
            return fmt.Errorf("Unable to list nodes to check labels, err %s", err.Error())
        }

        for _, node := range list.Items {
            for _, ns := range nodeSelectorsListfromcrd {
                if node.ObjectMeta.Name == ns {
                    labels := node.GetLabels()
                    hasCustomLabel := hasCustomX100Label(labels)
                    isrunningDefaultSW_flag := isrunningDefaultSW(labels)
                    isrunningNonDefaultSW_flag := isrunningNonDefaultSW(labels)
                    isRunningwithcurrCR_flag := isRunningwithcurrCR(labels, policy.ObjectMeta.Name)
                    if hasCustomLabel && !isrunningNonDefaultSW_flag {
                        if isrunningDefaultSW_flag {
                            delete(labels, x100swvdefaultKey)
                        }
                        delete(labels, x100CountOnNodekey)
                        delete(labels, x100BootupSuccess)
                        delete(labels, x100BootupSuccessCount)
                        delete(labels, x100BootupFailedCount)
                        delete(labels, x100HwMgrRunning)
                        delete(labels, x100BootupStatusMarked)
                        delete(labels, x100swversionKey)
                        labels[x100Upgrading] = "true"
                        labels[x100PriorCr] = labels[x100activecrdKey]
                        labels[x100activecrdKey] = policy.ObjectMeta.Name
                        node.SetLabels(labels)
                        err = c.rec.Update(context.TODO(), &node)
                        if err != nil {
                            return fmt.Errorf("Non-Default Unable to delete the old CR labels on node %s , err %s", node.ObjectMeta.Name, err.Error())
                        }
                        if !isRunningwithcurrCR_flag {
                            c.waitForKmmPodTermination(&node)
                            c.waitForFwPodTermination(&node)
                            c.waitForHwPodTermination(&node)
                        }
                        labels = node.GetLabels()
                        labels[x100swversionKey] = policySpec.SwVersion
                        labels[x100swvNondefaultKey] = x100swvNondefaultValue
                        node.SetLabels(labels)
                        err = c.rec.Update(context.TODO(), &node)
                        if err != nil {
                            return fmt.Errorf("Non-Default- Unable to label node %s with %s, err %s", node.ObjectMeta.Name, x100activecrdKey, err.Error())
                        }
                    } else if hasCustomLabel && isrunningNonDefaultSW_flag {
                        cropts := []client.ListOption{}
                        crlist := &xcardv1.X100ManagementPolicyList{}
                        err  := c.rec.List(context.TODO(), crlist, cropts...)
                        if err != nil {
                            log.Log.Error(err, "labelX100NodeswithCRDFields()- Unable to list X100ManagementPolicy CRs")
                        }
                        for _, policy := range crlist.Items {
                            currCR, _ := getActiveCRonNode(labels)
                            if policy.ObjectMeta.GetName() == currCR && c.x100Policy.ObjectMeta.Name != policy.ObjectMeta.GetName() {
                                //delete the Node from policyObject.Spec.NodeSelector[]
                                pnodes := policy.Spec.NodeSelector
                                var ind int
                                for i, pnode := range pnodes {
                                    if pnode == node.ObjectMeta.Name {
                                        ind = i
                                        break
                                    }
                                }
                                policy.Spec.NodeSelector = append(policy.Spec.NodeSelector[:ind], policy.Spec.NodeSelector[ind+1:]...)
                                c.rec.Update(context.TODO(), &policy)
                                //Update the CR back with context.TODO()
                                labels = node.GetLabels()
                                delete(labels, x100CountOnNodekey)
                                delete(labels, x100BootupSuccess)
                                delete(labels, x100BootupSuccessCount)
                                delete(labels, x100BootupFailedCount)
                                delete(labels, x100HwMgrRunning)
                                delete(labels, x100BootupStatusMarked)
                                delete(labels, x100swversionKey)
                                labels[x100Upgrading] = "true"
                                labels[x100PriorCr] = labels[x100activecrdKey]
                                labels[x100activecrdKey] = c.x100Policy.ObjectMeta.Name
                                node.SetLabels(labels)
                                err = c.rec.Update(context.TODO(), &node)
                                if err != nil {
                                    return fmt.Errorf("Non-Default- Unable to delete labels on node %s , err %s", node.ObjectMeta.Name, err.Error())
                                }
                                if !isRunningwithcurrCR_flag {
                                    c.waitForKmmPodTermination(&node)
                                    c.waitForFwPodTermination(&node)
                                    c.waitForHwPodTermination(&node)
                                }
                                nopts := []client.ListOption{ client.MatchingFields{"metadata.name": node.ObjectMeta.Name }}
                                tempnodes := &corev1.NodeList{}
                                err = c.rec.List(context.TODO(), tempnodes, nopts...)
                                if err != nil {
                                    log.Log.Info("Could not get Latest Node", err,"--")
                                }
                                tempnode := tempnodes.Items[0]
                                labels = tempnode.GetLabels()
                                labels[x100swversionKey] = policySpec.SwVersion
                                labels[x100swvNondefaultKey] = x100swvNondefaultValue
                                tempnode.SetLabels(labels)
                                err = c.rec.Update(context.TODO(), &tempnode)
                                if err != nil {
                                    return fmt.Errorf("Non-Default- Unable to label node %s with %s, err %s", tempnode.ObjectMeta.Name, x100activecrdKey, err.Error())
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    return nil
}


func (c *ControllerState) labelx100bootupStatusforNodes() (xcardv1.State) {

    nodesCount := 0
    for {
        opts := []client.ListOption{}
        nodes_list := &corev1.NodeList{}
        err := c.rec.List(context.TODO(), nodes_list, opts...)
        if err != nil {
            if kerrors.IsNotFound(err) {
                log.Log.Info("labelx100bootupStatusforNodes: Reached endState but unable to list any nodes", "Error : ", err.Error())
                return xcardv1.NotOperational
            }
        }
        nodesCount = len(nodes_list.Items)
        log.Log.Info("labelx100bootupStatusforNodes:", "NodesCount", nodesCount)
        for _, node := range nodes_list.Items {
            labels := node.GetLabels()
            if !isRunningwithcurrCR(labels,c.x100Policy.ObjectMeta.Name) {
                nodesCount--
                log.Log.Info("labelx100bootupStatusforNodes:", "not running with current CR: Removing Node ",node.GetName())
            } else if isRunningwithcurrCR(labels,c.x100Policy.ObjectMeta.Name) &&  isX100BootupStatusMarkedLabelTrue(labels) {
                nodesCount--
                log.Log.Info("labelx100bootupStatusforNodes:", " hasBootupSuccessTrue label: Removing Node ",node.GetName())
                continue
            }
            // Look at nodes with only x100 labels and HwmgrRunning=true label and currCR is activeCR on Node
            if !hasCustomX100Label(labels) || !hasHwmgrRunningLabel(labels)  || !isRunningwithcurrCR(labels,c.x100Policy.ObjectMeta.Name) {
                continue
            }

            x100count, err := c.getX100CardCountOnNode(&node)
            if x100count > 0 {
                labels = node.GetLabels()
                labels[x100CountOnNodekey] = strconv.Itoa(x100count)
                node.SetLabels(labels)
                err = c.rec.Update(context.TODO(), &node)

                if err != nil {
                    log.Log.Info("labelx100bootupStatusforNodes: Unable to label node", node.ObjectMeta.Name, " with ",x100CountOnNodekey , err.Error())
                } else {
                    log.Log.Info("labelx100bootupStatusforNodes: X100 Count on Node", node.ObjectMeta.Name,  x100count)
                }

                successBootCount, failedBootCount, err := c.getX100BootupStatusOnNode(&node, x100count)
                labels = node.GetLabels()
                if err != nil {
                    log.Log.Info("labelx100bootupStatusforNodes: Unable to get X100 Bootup status for node", node.ObjectMeta.Name, err.Error())
                    labels[x100BootupSuccess] = "unknown"
                    labels[x100BootupSuccessCount] = strconv.Itoa(successBootCount)
                } else if successBootCount+failedBootCount != x100count {
                    log.Log.Info("labelx100bootupStatusforNodes: Success X100 Bootup for node", node.ObjectMeta.Name, successBootCount, ", Failed bootup:",failedBootCount)
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
                    log.Log.Info("labelx100bootupStatusforNodes: Unable to label node", node.ObjectMeta.Name, " with ", x100BootupSuccess , err.Error())
                } else {
                    log.Log.Info("labelx100bootupStatusforNodes: X100 Bootup success Count on Node", node.ObjectMeta.Name,  successBootCount)
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
