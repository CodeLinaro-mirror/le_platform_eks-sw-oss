#!/bin/bash
#
#Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
#SPDX-License-Identifier: BSD-3-Clause-Clear
#
#

# Exit on error immediately
set -e

# Script needs both source and destination paths as argument
if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <source_path>"
    exit 1
fi

SOURCE_PATH=$1
DEST_PATH=$SOURCE_PATH/x100-operator
mkdir -p $DEST_PATH
# Clean the destination or else it will fail
rm -rf  $DEST_PATH/*

# Change directory to destination i.e. project root
cd $DEST_PATH

# Create an operator project, this will generate boilerplate code
operator-sdk init --domain=com --repo ${DEST_PATH:1}
operator-sdk create api --group=qualcomm --version=v1 --kind=X100ManagementPolicy
# https://sdk.operatorframework.io/docs/building-operators/golang/webhook/
operator-sdk create webhook --group=qualcomm --version=v1 --kind=X100ManagementPolicy --programmatic-validation

# Copy the PROJECT file, change it accordingly if domain and group have been changed
\cp $SOURCE_PATH/PROJECT $DEST_PATH/PROJECT
\cp $SOURCE_PATH/go.mod $DEST_PATH/go.mod
\rm $DEST_PATH/go.sum
\rm $DEST_PATH/controllers/suite_test.go

#\cp $SOURCE_PATH/go.sum $DEST_PATH/go.sum

# Copy the _types.go file that defines the CRD
\cp -r $SOURCE_PATH/api/v1/* $DEST_PATH/api/v1/

# Copy the go packages created for this CRD
rm -rf $DEST_PATH/controllers
\cp -r $SOURCE_PATH/controllers $DEST_PATH/

\cp $SOURCE_PATH/Dockerfile $DEST_PATH/Dockerfile
\cp $SOURCE_PATH/main.go $DEST_PATH/main.go
\cp $SOURCE_PATH/Makefile $DEST_PATH/Makefile

go mod tidy

# Run the make to generate manifests in the config
make generate manifests

# Copy the resources that this operator controls for creation/updation/deletion
\cp -r $SOURCE_PATH/assets $DEST_PATH/

# Config related files have naming changes
# If you change domain or project name, change these accordingly

\mkdir -p $DEST_PATH/config/x100-crd
\cp -r $SOURCE_PATH/config/x100-crd/* $DEST_PATH/config/x100-crd/
\cp -r $SOURCE_PATH/config/rbac/*role_binding*.yaml $DEST_PATH/config/rbac/
\cp -r $SOURCE_PATH/config/rbac/service_account.yaml $DEST_PATH/config/rbac/service_account.yaml

\cp -r $SOURCE_PATH/config/manifests/bases $DEST_PATH/config/manifests/
\cp -r $SOURCE_PATH/config/manager/* $DEST_PATH/config/manager/
\cp -r $SOURCE_PATH/config/default/* $DEST_PATH/config/default/
\cp -r $SOURCE_PATH/config/webhook/* $DEST_PATH/config/webhook/

# Run make commands
make docker-build docker-push
make bundle bundle-build bundle-push
