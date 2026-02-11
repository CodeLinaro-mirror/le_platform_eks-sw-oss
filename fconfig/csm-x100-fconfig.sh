#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

# Create directories for both common and device-specific configs
mkdir -p /var/tmp/lassenconfig
mkdir -p /var/tmp/lassenconfig_common
mkdir -p /var/tmp/lassenconfig_devices
echo $DYNAMICCONFIG_PODNAME | rev | cut -d- -f2- | rev > /var/tmp/lassenconfig/fconfig-version.log

# Initialize arrays to track devices
declare -a all_devices
declare -a configured_devices

# Copy common configs to common directory and original locations (for backward compatibility)
rsync -a /etc/config/* /var/lib/firmware/qcom/lassen/flatimg/config/.
rsync -a /etc/config/* /var/tmp/lassenconfig/.
rsync -a /etc/config/* /var/tmp/lassenconfig_common/.

mkdir -p /tmp/configmap
cp -L /etc/configmap/"$CompressedConfig" /tmp/configmap/
tar -xzf /tmp/configmap/"$CompressedConfig" -C /tmp/configmap/

# Build slot -> serial mapping and synthesize serial-based views from slot-based inputs
# Users supply: device_list.conf entries like slot1, slot2; manifests apply_configuration_slotN.conf; payload under devices/slotN/
# We compute serial IDs via sysfs and create serial-named views under /tmp/configmap before existing processing.
declare -A slot_to_serial

slot_name_to_num() {
    local s="$1"
    if [[ "$s" =~ ^slot([0-9]+)$ ]]; then
        echo "${BASH_REMATCH[1]}"
    else
        echo ""
    fi
}

normalize_serial_hex() {
    local raw="$1"
    raw="${raw##*: }"
    echo "${raw,,}"
}

build_slot_to_serial_map() {
    echo "Building slot -> serial map..." >> /var/tmp/lassenconfig/config.log
    for slot in /sys/bus/pci/slots/*; do
        [ -e "$slot" ] || continue
        local slot_num
        slot_num="$(basename "$slot")"
        local addr
        addr="$(cat "$slot/address" 2>/dev/null)"
        [ -n "$addr" ] || continue
        for devfn in /sys/bus/pci/devices/${addr}.*; do
            [ -d "$devfn" ] || continue
            local mhi_entry
            mhi_entry="$(ls "$devfn" 2>/dev/null | grep -E '^mhi[0-9]+$' | head -n1)"
            if [ -n "$mhi_entry" ]; then
                local mhi_idx="${mhi_entry#mhi}"
                local sn_path="/sys/bus/mhi/devices/mhi${mhi_idx}/serial_number"
                if [ -f "$sn_path" ]; then
                    local sn
                    sn="$(normalize_serial_hex "$(cat "$sn_path" 2>/dev/null)")"
                    if [[ "$sn" =~ ^0x[0-9a-f]+$ ]]; then
                        slot_to_serial["$slot_num"]="$sn"
                        echo "slot $slot_num -> $sn via $addr ($mhi_entry)" >> /var/tmp/lassenconfig/config.log
                    else
                        echo "WARN: Unexpected serial format for slot $slot_num from $sn_path: $(cat "$sn_path" 2>/dev/null)" >> /var/tmp/lassenconfig/config.log
                    fi
                else
                    echo "WARN: No serial_number at $sn_path for slot $slot_num" >> /var/tmp/lassenconfig/config.log
                fi
                break
            fi
        done
    done
}

synthesize_slot_based_views() {
    echo "Synthesizing serial-named views from node-specific slot-based inputs..." >> /var/tmp/lassenconfig/config.log
    shopt -s nullglob
    # Node-specific manifests: nodes/$NODE_NAME/apply_configuration_slotN.conf -> apply_configuration_<serial>.conf
    for conf in /tmp/configmap/nodes/$NODE_NAME/apply_configuration_slot*.conf; do
        local filename slot_token slot_num serial_hex dest
        filename="$(basename "$conf")"
        slot_token="${filename#apply_configuration_}"
        slot_token="${slot_token%.conf}"
        slot_num="$(slot_name_to_num "$slot_token")"
        if [ -z "$slot_num" ]; then
            echo "WARN: Could not parse slot number from $filename" >> /var/tmp/lassenconfig/config.log
            continue
        fi
        serial_hex="${slot_to_serial[$slot_num]}"
        if [ -z "$serial_hex" ]; then
            echo "WARN: No serial mapping found for $slot_token on node $NODE_NAME; skipping manifest" >> /var/tmp/lassenconfig/config.log
            continue
        fi
        dest="/tmp/configmap/apply_configuration_${serial_hex}.conf"
        cp -p "$conf" "$dest"
        echo "Created $dest from nodes/$NODE_NAME/$filename" >> /var/tmp/lassenconfig/config.log
    done
    # Node-specific payload: nodes/$NODE_NAME/devices/slotN -> devices/<serial>
    for d in /tmp/configmap/nodes/$NODE_NAME/devices/slot*; do
        [ -d "$d" ] || continue
        local slot_dir slot_num serial_hex target_dir
        slot_dir="$(basename "$d")"
        slot_num="$(slot_name_to_num "$slot_dir")"
        if [ -z "$slot_num" ]; then
            echo "WARN: Could not parse slot number from nodes/$NODE_NAME/devices/$slot_dir" >> /var/tmp/lassenconfig/config.log
            continue
        fi
        serial_hex="${slot_to_serial[$slot_num]}"
        if [ -z "$serial_hex" ]; then
            echo "WARN: No serial mapping found for devices/$slot_dir on node $NODE_NAME; skipping payload copy" >> /var/tmp/lassenconfig/config.log
            continue
        fi
        target_dir="/tmp/configmap/devices/${serial_hex}"
        mkdir -p "$target_dir"
        rsync -a "$d"/ "$target_dir"/
        echo "Populated devices/${serial_hex} from nodes/$NODE_NAME/devices/${slot_dir}" >> /var/tmp/lassenconfig/config.log
    done
    shopt -u nullglob
}

parse_device_entry() {
    local entry="$1"
    local node_name slot_name config_type

    if [[ "$entry" =~ ^([^:]+):\*:common$ ]]; then
        node_name="${BASH_REMATCH[1]}"
        slot_name="*"
        config_type="wildcard_common"
    elif [[ "$entry" =~ ^([^:]+):([^:]+):common$ ]]; then
        node_name="${BASH_REMATCH[1]}"
        slot_name="${BASH_REMATCH[2]}"
        config_type="force_common"
    elif [[ "$entry" =~ ^([^:]+):([^:]+)$ ]]; then
        node_name="${BASH_REMATCH[1]}"
        slot_name="${BASH_REMATCH[2]}"
        config_type="auto"
    else
        echo "Warning: Invalid device_list entry format '$entry'" >> /var/tmp/lassenconfig/config.log
        return 1
    fi

    echo "$node_name|$slot_name|$config_type"
}

get_valid_x100_slots() {
    local valid_slots=()
    for slot_num in "${!slot_to_serial[@]}"; do
        if [[ -n "${slot_to_serial[$slot_num]}" ]]; then
            valid_slots+=("$slot_num")
        fi
    done
    echo "${valid_slots[@]}"
}

get_config_for_device() {
    local node="$1" slot_num="$2" config_type="$3"

    if [[ "$config_type" != "force_common" ]]; then
        # Try specific config first
        if [[ -f "/tmp/configmap/nodes/$node/apply_configuration_slot${slot_num}.conf" ]]; then
            echo "nodes/$node/apply_configuration_slot${slot_num}.conf|nodes/$node/devices/slot${slot_num}"
            return
        fi
    fi

    # Try node common config
    if [[ -f "/tmp/configmap/nodes/$node/apply_configuration.conf" ]]; then
        echo "nodes/$node/apply_configuration.conf|nodes/$node/common"
        return
    fi

    # Fallback to global common
    if [[ -f "/tmp/configmap/apply_configuration.conf" ]]; then
        echo "apply_configuration.conf|common"
        return
    fi

    # No config available
    echo "none|none"
}

trim_version(){
    local filename="$1"
    local ext="${filename##*.}"
    local name="${filename%_[0-9]*}.$ext"
    echo "$name"
}

apply_config_to_device() {
    local serial="$1"
    local config_source="$2"

    IFS='|' read -r config_file files_dir <<< "$config_source"

    echo "Applying config to device $serial using $config_file from $files_dir" >> /var/tmp/lassenconfig_devices/device_config.log

    # Create device-specific config directory
    mkdir -p /var/tmp/lassenconfig_devices/$serial

    # Process configuration file
    local conf_path="/tmp/configmap/$config_file"
    if [[ ! -f "$conf_path" ]]; then
        echo "Error: Configuration file not found: $conf_path" >> /var/tmp/lassenconfig_devices/device_config.log
        return 1
    fi

    while IFS= read -r file; do
        # Skip empty lines and comments
        file=$(echo $file | sed 's/\r//g')
        if [ -n "$file" ] && [[ ! "$file" =~ ^[[:space:]]*# ]]; then
            local dir=$(dirname $file)
            local filename=$(basename $file)
            local source_file=""

            # Look for file in the specified files directory
            if [ -f "/tmp/configmap/$files_dir/$dir/$filename" ]; then
                source_file="/tmp/configmap/$files_dir/$dir/$filename"
            elif [ -f "/tmp/configmap/$files_dir/$filename" ]; then
                source_file="/tmp/configmap/$files_dir/$filename"
            else
                echo "Warning: File $dir/$filename not found in $files_dir for device $serial" >> /var/tmp/lassenconfig_devices/device_config.log
                continue
            fi

            local config_name=$(trim_version "$file")
            mkdir -p /var/tmp/lassenconfig_devices/$serial/"$dir"
            cp -p "$source_file" /var/tmp/lassenconfig_devices/$serial/"$config_name"
            echo "Applied config file $dir/$filename to device $serial" >> /var/tmp/lassenconfig_devices/device_config.log
        fi
    done < "$conf_path"
}

process_wildcard_common() {
    local node="$1"
    local config_source

    # Determine common config source using fallback hierarchy
    if [[ -f "/tmp/configmap/nodes/$node/apply_configuration.conf" ]]; then
        config_source="nodes/$node/apply_configuration.conf|nodes/$node/common"
        echo "Using node common config for wildcard: $config_source" >> /var/tmp/lassenconfig_devices/device_config.log
    elif [[ -f "/tmp/configmap/apply_configuration.conf" ]]; then
        config_source="apply_configuration.conf|common"
        echo "Using global common config for wildcard: $config_source" >> /var/tmp/lassenconfig_devices/device_config.log
    else
        echo "Warning: No common config available for wildcard $node:*:common" >> /var/tmp/lassenconfig_devices/device_config.log
        return
    fi

    # Apply to all valid x100 slots
    local valid_slots=($(get_valid_x100_slots))
    echo "Found ${#valid_slots[@]} valid x100 slots for wildcard: ${valid_slots[*]}" >> /var/tmp/lassenconfig_devices/device_config.log

    for slot_num in "${valid_slots[@]}"; do
        local serial="${slot_to_serial[$slot_num]}"
        if [[ -n "$serial" ]]; then
            apply_config_to_device "$serial" "$config_source"
            configured_devices+=("$serial")
            all_devices+=("$serial")
            echo "Applied common config to $node:slot$slot_num ($serial) via wildcard" >> /var/tmp/lassenconfig_devices/device_config.log
        fi
    done
}

# Build mapping and synthesize serial-based views before any device-specific processing
build_slot_to_serial_map
synthesize_slot_based_views

# Parse device_list.conf and build processing queue
declare -a device_queue
echo "Reading device list from device_list.conf for node: $NODE_NAME..." >> /var/tmp/lassenconfig/config.log


if [ -f /tmp/configmap/device_list.conf ]; then
    while IFS= read -r device; do
        # Skip empty lines and comments
        device=$(echo $device | sed 's/\r//g')
        if [ -n "$device" ] && [[ ! "$device" =~ ^[[:space:]]*# ]]; then
            parsed=$(parse_device_entry "$device")
            if [[ $? -eq 0 ]]; then
                IFS='|' read -r node_name slot_name config_type <<< "$parsed"

                # Only process if this is our node
                if [ "$node_name" = "$NODE_NAME" ]; then
                    device_queue+=("$node_name|$slot_name|$config_type")
                    echo "Queued device entry: $node_name:$slot_name ($config_type)" >> /var/tmp/lassenconfig/config.log
                else
                    echo "Skipping device for different node: $node_name:$slot_name (current node: $NODE_NAME)" >> /var/tmp/lassenconfig/config.log
                fi
            fi
        fi
    done < /tmp/configmap/device_list.conf

    # Log the total number of device entries found for this node
    echo "Total device entries found in device_list.conf for node $NODE_NAME: ${#device_queue[@]}" >> /var/tmp/lassenconfig/config.log
else
    echo "Warning: No device_list.conf found" >> /var/tmp/lassenconfig/config.log
fi

# For backward compatibility, still process common configuration for the flatimg directory
echo "Processing common configuration for backward compatibility..." >> /var/tmp/lassenconfig/config.log
if [ -n "$ApplyConfiguration" ] && [ -f /tmp/configmap/$ApplyConfiguration ]; then
    conf_file=/tmp/configmap/"$ApplyConfiguration"
    echo "Using configuration file: $conf_file" >> /var/tmp/lassenconfig/config.log

    while IFS= read -r file; do
        # Skip empty lines and comments
        file=$(echo $file | sed 's/\r//g')
        if [ -n "$file" ] && [[ ! "$file" =~ ^[[:space:]]*# ]]; then
            dir=$(dirname $file)
            filename=$(basename $file)

            # Check if file exists in common directory with subdirectory structure or at root level
            if [ -f /tmp/configmap/common/$dir/$filename ]; then
                source_file="/tmp/configmap/common/$dir/$filename"
                echo "Found file in common directory: $dir/$filename" >> /var/tmp/lassenconfig/config.log
            elif [ -f /tmp/configmap/common/$filename ]; then
                source_file="/tmp/configmap/common/$filename"
                echo "Found file in common directory: $filename" >> /var/tmp/lassenconfig/config.log
            elif [ -f /tmp/configmap/$filename ]; then
                source_file="/tmp/configmap/$filename"
                echo "Found file in root directory: $filename" >> /var/tmp/lassenconfig/config.log
            else
                echo "Warning: File $dir/$filename not found" >> /var/tmp/lassenconfig/config.log
                continue
            fi

        config_name=$(trim_version "$file")
        cp -p "$source_file" /var/lib/firmware/qcom/lassen/flatimg/config/"$config_name"
        mkdir -p /var/tmp/lassenconfig/"$dir"
        cp -p "$source_file" /var/tmp/lassenconfig/"$config_name"
        fi
    done < "$conf_file"
fi

# Process devices explicitly listed in device_list.conf only
mkdir -p /var/tmp/lassenconfig_devices
echo "Processing devices explicitly listed in device_list.conf..." > /var/tmp/lassenconfig_devices/device_config.log

# Process each device entry from device_list.conf
for device_entry in "${device_queue[@]}"; do
    IFS='|' read -r node_name slot_name config_type <<< "$device_entry"

    echo "Processing device entry: $node_name:$slot_name ($config_type)" >> /var/tmp/lassenconfig_devices/device_config.log

    if [[ "$config_type" == "wildcard_common" ]]; then
        # Handle wildcard: apply common config to all valid x100 slots
        process_wildcard_common "$node_name"
        continue
    fi

    # Handle specific slot entries
    slot_num="$(slot_name_to_num "$slot_name")"

    if [[ -z "$slot_num" ]]; then
        echo "Warning: Invalid slot name '$slot_name' for node $node_name" >> /var/tmp/lassenconfig_devices/device_config.log
        continue
    fi

    serial="${slot_to_serial["$slot_num"]}"

    if [[ -z "$serial" ]]; then
        echo "Warning: No serial mapping found for $slot_name on node $node_name; skipping" >> /var/tmp/lassenconfig_devices/device_config.log
        continue
    fi

    # Get configuration source using fallback hierarchy
    config_source=$(get_config_for_device "$node_name" "$slot_num" "$config_type")
    IFS='|' read -r config_file files_dir <<< "$config_source"

    if [[ "$config_file" == "none" ]]; then
        echo "Warning: No configuration available for $node_name:$slot_name ($serial); skipping" >> /var/tmp/lassenconfig_devices/device_config.log
        continue
    fi

    # Apply configuration to device
    apply_config_to_device "$serial" "$config_source"
    configured_devices+=("$serial")
    all_devices+=("$serial")

    echo "Successfully processed $node_name:$slot_name -> $serial using $config_file" >> /var/tmp/lassenconfig_devices/device_config.log
done

# Create a special marker file if we have any device configurations
if [ ${#all_devices[@]} -gt 0 ]; then
    echo "1" > /var/tmp/device_specific_configs_available
    echo "Device configurations prepared for ${#all_devices[@]} devices: ${all_devices[*]}" >> /var/tmp/lassenconfig_devices/device_config.log
else
    echo "No devices processed from device_list.conf" >> /var/tmp/lassenconfig_devices/device_config.log
fi

chmod -R 777 /var/lib/firmware/qcom/lassen/flatimg/config
chmod -R 777 /var/tmp/lassenconfig_common
chmod -R 777 /var/tmp/lassenconfig_devices
bash -c "echo -n 1 > /var/tmp/fconfig-state.conf"

while true; do
   echo "Sleeping 5: $(date)"
   sleep 5;
done;

