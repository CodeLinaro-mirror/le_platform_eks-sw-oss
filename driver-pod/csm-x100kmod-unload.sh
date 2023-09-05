#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/
echo "Unloading X100 host-drivers" >> /var/log/x100_kmodules-load_ocp.log

rmmod -f csm_dp mhi_ptp mhi_pci wwan_mhi wwan mhi_net mhi_uci mhi
lsmod | grep -e mhi -e csm >> /var/log/x100_kmodules-load_ocp.log
