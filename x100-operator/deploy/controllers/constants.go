/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
*/

package controllers

import (
    xcardv1 "x100-operator/api/v1"
)

var NamePrefixes = map[string]string{
    "firmware":     "csm-x100fw-daemonset",
    "HwManager":    "csm-x100hwmgr-daemonset",
    "kmodules":     "csm-x100-kmm",
    "kmmConfigMap": "csm-x100kmodules-configmap",
    "devicePlugin": "csm-x100-dpds",
}

var PodNamePrefixes = map[string]string{
    "firmware":     "csm-x100fw",
    "HwManager":    "csm-x100hwmgr",
    "kmodules":     "csm-x100-kmm",
    "kmmConfigMap": "csm-x100kmodules-configmap",
    "devicePlugin": "csmx100-dp",
}

const (
    FirmwareDsName     = "csm-x100-firmware-ds"
    HwManagerDsName    = "csm-x100-hwmanager-ds"
    KModulesName       = "csm-x100-kmm"
    KModuleCMName      = "csm-x100-kmmdockerfile"
    DevicePluginDsName = "csm-x100-deviceplugin-ds"
)

const (
    x100HwMgrRunning   = "qualcomm.com/x100.hwmgrRunning"
)

const (
    x100LabelKey   = "qualcomm.com/x100.present"
    x100LabelValue = "true"
    x100activecrdKey = "qualcomm.com/x100.activecrd"
    x100swversionKey = "qualcomm.com/x100.swversion"
    x100swvdefaultKey = "qualcomm.com/x100.swvdefault"
    x100swvdefaultValue = "true"
    x100swvNondefaultKey = "qualcomm.com/x100.swvNondefault"
    x100swvNondefaultValue = "true"
)

var x100crdLabels = []string{
    x100swversionKey,
    x100swvdefaultKey,
    x100swvNondefaultKey,
    x100activecrdKey,
    x100HwMgrRunning,
    x100LabelKey,
}

const (
    SoftwareVersionSeperator = "-"
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

var ModuleIdentifierLabelValue string = "KMODULES_NAME"

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
