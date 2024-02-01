#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

mkdir -p /var/tmp/lassenconfig
echo $DYNAMICCONFIG_PODNAME | rev | cut -d- -f2- | rev > /var/tmp/lassenconfig/fconfig-version.log

rsync -a /etc/DuMgrConfig.txt /var/lib/firmware/qcom/lassen/flatimg/config/.
rsync -a /etc/DuMgrConfig.txt /var/tmp/lassenconfig/.

rsync -a /etc/splane_cfg.conf /var/lib/firmware/qcom/lassen/flatimg/config/.
rsync -a /etc/splane_cfg.conf /var/tmp/lassenconfig/.

rsync -a /etc/vf-count.conf /var/tmp/.
chmod -R 777 /var/lib/firmware/qcom/lassen/flatimg/config
bash -c "echo -n 1 > /var/tmp/fconfig-state.conf"

while true; do
   echo "Sleeping 5: $(date)"
   sleep 5;
done;
