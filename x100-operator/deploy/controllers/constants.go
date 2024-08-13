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
	"kmodules":     "csmx100",
	"kmmConfigMap": "csm-x100kmodule-configmap",
	"devicePlugin": "csm-x100-deviceplugin-daemonset",
}

var PodNamePrefixes = map[string]string{
	"firmware":     "csm-x100fw",
	"HwManager":    "csm-x100hwmgr",
	"kmodules":     "csmx100",
	"kmmConfigMap": "csm-x100kmodule-configmap",
	"devicePlugin": "csm-x100-deviceplugin",
}

const (
	FirmwareDsName     = "csm-x100-firmware-ds"
	HwManagerDsName    = "csm-x100-hwmanager-ds"
	KModulesName       = "csmx100"
	KModuleCMName      = "csm-x100-kmmdockerfile"
	DevicePluginDsName = "csm-x100-deviceplugin-ds"
)
const (
	FirmwareDsSelectorLabelKey   = "qualcomm.com/x100.fw.present"
	HwMgrDsSelectorLabelKey      = "qualcomm.com/x100.hwmgr.present"
	KModuleSelectorLabelKey      = "qualcomm.com/x100.kmodules.present"
	DevicePluginSelectorLabelKey = "qualcomm.com/x100.deviceplugin.present"
)
var x100SelectorLabels = []string{
	FirmwareDsSelectorLabelKey,
	HwMgrDsSelectorLabelKey,
	KModuleSelectorLabelKey,
	DevicePluginSelectorLabelKey,
}

const (
	healthLogPath   = "/var/log/healthinfo.log"
	assetsNamespace = "x100-operator-resources"
	nfdResourcePath = "/opt/x100-operator/nfd-configuration"
)

const (
	x100LabelKey               = "qualcomm.com/x100.present"
	x100LabelValue             = "true"
	x100ActiveCR               = "qualcomm.com/x100.activeCR"
	x100PriorCR                = "qualcomm.com/x100.priorCR"
	x100SwVersion              = "qualcomm.com/x100.swversion"
	x100CountOnNode            = "qualcomm.com/x100.count"
	x100BootupSuccess          = "qualcomm.com/x100.bootsuccess"
	x100BootupSuccessCount     = "qualcomm.com/x100.bootSuccessCount"
	x100BootupFailedCount      = "qualcomm.com/x100.bootFailedCount"
	x100BootupStatusMarked     = "qualcomm.com/x100.bootStatusMarked"
	x100Upgrading              = "qualcomm.com/x100.upgrading"
	x100UpgradeFailed          = "qualcomm.com/x100.swUpgradeFailed"
	x100RollingBackUpgrade     = "qualcomm.com/x100.rollingBackUpgrade"
	x100RolledBack             = "qualcomm.com/x100.rolledBack"
	x100OwnerPolicyDeleted     = "qualcomm.com/x100.ownerPolicyDeleted"
	x100TeardownCompleted      = "qualcomm.com/x100.teardownCompleted"
	x100NodeAggregationBlocked = "qualcomm.com/x100.aggregationBlocked"
)

var x100CrdLabels = []string{
	x100SwVersion,
	x100ActiveCR,
	x100PriorCR,
	x100LabelKey,
	x100CountOnNode,
	x100BootupSuccess,
	x100BootupSuccessCount,
	x100BootupFailedCount,
	x100BootupStatusMarked,
	x100Upgrading,
	x100UpgradeFailed,
	x100RollingBackUpgrade,
	x100RolledBack,
	x100OwnerPolicyDeleted,
	x100TeardownCompleted,
	x100NodeAggregationBlocked,
}

var x100StatusLabels = []string{
	x100CountOnNode,
	x100BootupSuccess,
	x100BootupSuccessCount,
	x100BootupFailedCount,
	x100BootupStatusMarked,
	x100UpgradeFailed,
	//x100RolledBack,
	x100TeardownCompleted,
}

var x100TransientStateLabels = []string{
	x100Upgrading,
	x100RollingBackUpgrade,
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
	endState
	earlyExitState
)

var kmmReadyLabel string = "kmm.node.kubernetes.io/x100-operator-resources."

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
	currentState int
	desiredState int
	currentAsset string
	assets       []string
	x100Policy   *xcardv1.X100ManagementPolicy
	rec          *X100ManagementPolicyReconciler
}
