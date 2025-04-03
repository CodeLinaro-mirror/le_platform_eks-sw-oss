#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

mkdir -p /var/tmp/lassenconfig
echo $DYNAMICCONFIG_PODNAME | rev | cut -d- -f2- | rev > /var/tmp/lassenconfig/fconfig-version.log

rsync -ac /etc/config/* /var/lib/firmware/qcom/lassen/flatimg/config/.
rsync -ac /etc/config/* /var/tmp/lassenconfig/.

mkdir -p /tmp/configmap
cp -L /etc/configmap/"$CompressedConfig" /tmp/configmap/
tar -xzf /tmp/configmap/"$CompressedConfig" -C /tmp/configmap/

trim_version(){
    local filename="$1"
    local ext="${filename##*.}"
    local name="${filename%_[0-9]*}.$ext"
    echo "$name"
}

if [ -n $ApplyConfiguration ] && [ -f /tmp/configmap/$ApplyConfiguration ]; then
    conf_file=/tmp/configmap/"$ApplyConfiguration"

    while IFS= read -r file; do
        file=$(echo $file | sed 's/\r//g')
        dir=$(dirname $file)
        filename=$(basename $file)
        if [ -f /tmp/configmap/$file ]; then
            config_name=$(trim_version "$file")
            cp /tmp/configmap/$file /var/lib/firmware/qcom/lassen/flatimg/config/"$config_name"
            mkdir -p /var/tmp/lassenconfig/"$dir"
            cp /tmp/configmap/$file /var/tmp/lassenconfig/"$config_name"
        fi
    done < "$conf_file"
fi

chmod -R 777 /var/lib/firmware/qcom/lassen/flatimg/config
bash -c "echo -n 1 > /var/tmp/fconfig-state.conf"

while true; do
   echo "Sleeping 5: $(date)"
   sleep 5;
done;
