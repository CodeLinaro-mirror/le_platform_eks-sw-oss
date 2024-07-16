#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

mkdir -p /var/tmp/lassenconfig
echo $DYNAMICCONFIG_PODNAME | rev | cut -d- -f2- | rev > /var/tmp/lassenconfig/fconfig-version.log

rsync -a /etc/config/* /var/lib/firmware/qcom/lassen/flatimg/config/.
rsync -a /etc/config/* /var/tmp/lassenconfig/.

trim_version(){
    local filename="$1"
    local ext="${filename##*.}"
    local name="${filename%_[0-9]*}.$ext"
    echo "$name"
}

if [ -n $ApplyConfiguration ] && [ -f /etc/configmap/$ApplyConfiguration ]; then
    conf_file=/etc/configmap/"$ApplyConfiguration"

    while IFS= read -r file; do
        file=$(echo $file | sed 's/\r//g')
        dir=$(dirname $file)
        filename=$(basename $file)
        if [ -f /etc/configmap/$filename ]; then
            config_name=$(trim_version "$file")
            cp -p /etc/configmap/$filename /var/lib/firmware/qcom/lassen/flatimg/config/"$config_name"
            mkdir -p /var/tmp/lassenconfig/"$dir"
            cp -p /etc/configmap/$filename /var/tmp/lassenconfig/"$config_name"
        fi
    done < "$conf_file"
fi

chmod -R 777 /var/lib/firmware/qcom/lassen/flatimg/config
bash -c "echo -n 1 > /var/tmp/fconfig-state.conf"

while true; do
   echo "Sleeping 5: $(date)"
   sleep 5;
done;
