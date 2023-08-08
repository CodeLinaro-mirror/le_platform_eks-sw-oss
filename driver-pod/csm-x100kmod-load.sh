#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/
echo "X100 drivers loading" > /var/log/x100_kmodules-load_ocp.log

DPATH=/lib/modules/`uname -r`/
insmod $DPATH/mhi.ko*
insmod $DPATH/mhi_uci.ko*
insmod $DPATH/mhi_net.ko*
insmod $DPATH/wwan.ko*
insmod $DPATH/wwan_mhi.ko*
insmod $DPATH/mhi_pci.ko*
insmod $DPATH/csm_dp.ko*


lsmod | grep -e mhi -e csm >> /var/log/x100_kmodules-load_ocp.log
