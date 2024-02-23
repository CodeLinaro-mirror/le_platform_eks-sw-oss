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
    healthLogPath   = "/var/log/healthinfo.log"
    assetsNamespace = "x100-operator-resources"
)

const (
    x100LabelKey            = "qualcomm.com/x100.present"
    x100LabelValue          = "true"
    x100activecrdKey        = "qualcomm.com/x100.active-cr"
    x100PriorCr             = "qualcomm.com/x100.prior-cr"
    x100swversionKey        = "qualcomm.com/x100.swversion"
    x100swvdefaultKey       = "qualcomm.com/x100.swvdefault"
    x100swvdefaultValue     = "true"
    x100swvNondefaultKey    = "qualcomm.com/x100.swvnondefault"
    x100swvNondefaultValue  = "true"
    x100HwMgrRunning        = "qualcomm.com/x100.hwmgrRunning"
    x100CountOnNodekey      = "qualcomm.com/x100.count"
    x100BootupSuccess       = "qualcomm.com/x100.bootsuccess"
    x100BootupSuccessCount  = "qualcomm.com/x100.bootsuccesscount"
    x100BootupFailedCount   = "qualcomm.com/x100.bootfailedcount"
    x100BootupStatusMarked  = "qualcomm.com/x100.bootstatusmarked"
    x100Upgrading           = "qualcomm.com/x100.upgrading"
    x100UpgradeFailed       = "qualcomm.com/x100.sw-upgrade-failed"
    x100RolledBack          = "qualcomm.com/x100.rolledback"
)

var x100crdLabels = []string{
    x100swversionKey,
    x100swvdefaultKey,
    x100swvNondefaultKey,
    x100activecrdKey,
    x100PriorCr,
    x100HwMgrRunning,
    x100LabelKey,
    x100CountOnNodekey,
    x100BootupSuccess,
    x100BootupSuccessCount,
    x100BootupFailedCount,
    x100BootupStatusMarked,
    x100Upgrading,
    x100UpgradeFailed,
    x100RolledBack,
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
    getBootStatusState
    rollbackState
    endState
    earlyExitState
)

var ModuleIdentifierLabelValue string = "KMODULES_NAME"

// Sequence below is in terms with Pod dependencies
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
    currentState       int
    desiredState       int
    currentAsset       string
    assets             []string
    x100Policy         *xcardv1.X100ManagementPolicy
    rec                *X100ManagementPolicyReconciler
}
