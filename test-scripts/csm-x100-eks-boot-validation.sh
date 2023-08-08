#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

numofx100=$(lspci | grep Qualcomm | cut -d ":" -f1 | uniq | wc -l)
echo "Number of Qualcomm x100 cards connected: $numofx100"

echo "Iterating through each card to verify the boot-up."
echo "--------------------------------------------------"
for i in $(seq 0 $(($numofx100-1)));
do
   if [[ -c "/dev/mhi${i}_CSM_CTRL" && -c "/dev/mhi${i}_NMEA" && -c "/dev/mhi${i}_LOOPBACK" ]]; then
      echo "Device $i booted OK."
      #device_boot_check $i
   else
      echo "ERROR: Device $i is not booted, in bad state."
   fi
done
echo "--------------------------------------------------"
