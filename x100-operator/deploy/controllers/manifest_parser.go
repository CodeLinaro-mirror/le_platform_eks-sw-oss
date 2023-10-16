/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package controllers

import (
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"

    kmmv1 "github.com/kubernetes-sigs/kernel-module-management/api/v1beta1"
    secv1 "github.com/openshift/api/security/v1"
    nfdk8s "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
    opg "github.com/operator-framework/api/pkg/operators/v1"
    subs "github.com/operator-framework/api/pkg/operators/v1alpha1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    rbacv1 "k8s.io/api/rbac/v1"

    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/serializer/json"
    "k8s.io/client-go/kubernetes/scheme"
    "sigs.k8s.io/controller-runtime/pkg/log"
)

type manifest []byte

var AssetMap map[string]Asset

type Asset struct {
    ServiceAccount             corev1.ServiceAccount
    Secret                     corev1.Secret
    Role                       rbacv1.Role
    RoleBinding                rbacv1.RoleBinding
    ClusterRole                rbacv1.ClusterRole
    ClusterRoleBinding         rbacv1.ClusterRoleBinding
    ConfigMap                  corev1.ConfigMap
    DaemonSet                  appsv1.DaemonSet
    Deployment                 appsv1.Deployment
    PersistentVolume           corev1.PersistentVolume
    PersistentVolumeClaim      corev1.PersistentVolumeClaim
    Namespace                  corev1.Namespace
    Subscription               subs.Subscription
    OperatorGroup              opg.OperatorGroup
    NodeFeatureRule            nfdk8s.NodeFeatureRule
    Module                     kmmv1.Module
    SecurityContextConstraints secv1.SecurityContextConstraints
    objectMappings             []kvpair
    nameToField                map[string]runtime.Object
}

type kvpair struct {
    key   string
    value runtime.Object
}

func createAsset() *Asset {
    asset := Asset{nameToField: make(map[string]runtime.Object)}
    // Mappings help to refer back to actual struct field from kind
    asset.nameToField["ServiceAccount"] = &asset.ServiceAccount
    asset.nameToField["Secret"] = &asset.Secret
    asset.nameToField["Role"] = &asset.Role
    asset.nameToField["RoleBinding"] = &asset.RoleBinding
    asset.nameToField["ClusterRole"] = &asset.ClusterRole
    asset.nameToField["ClusterRoleBinding"] = &asset.ClusterRoleBinding
    asset.nameToField["ConfigMap"] = &asset.ConfigMap
    asset.nameToField["DaemonSet"] = &asset.DaemonSet
    asset.nameToField["Deployment"] = &asset.Deployment
    asset.nameToField["PersistentVolume"] = &asset.PersistentVolume
    asset.nameToField["PersistentVolumeClaim"] = &asset.PersistentVolumeClaim
    asset.nameToField["Namespace"] = &asset.Namespace
    asset.nameToField["Subscription"] = &asset.Subscription
    asset.nameToField["OperatorGroup"] = &asset.OperatorGroup
    asset.nameToField["NodeFeatureRule"] = &asset.NodeFeatureRule
    asset.nameToField["Module"] = &asset.Module
    asset.nameToField["SecurityContextConstraints"] = &asset.SecurityContextConstraints

    return &asset
}

func listManifestsFromDirectory(root string) ([]string, error) {
    files := []string{}
    err := filepath.Walk(root, func(path string, finfo os.FileInfo, err error) error {
        if err != nil {
            log.Log.Info("Error while walking %s: %v", path, err)
            return nil
        }
        if !finfo.IsDir() {
            if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
                files = append(files, path)
            }
        }
        return nil
    })

    return files, err
}

func extractResources(assetpath string) []manifest {

    // List all manifests avaiable in the asset
    files, err := listManifestsFromDirectory(assetpath)
    LogAndExitOnError(fmt.Sprintf("Error encountered while listing files in %s", assetpath), err)

    //Manifests are numbered to force an ordering in the sort
    sort.Strings(files)

    //All manifests for the resource are added to manifests list
    var manifests []manifest
    for _, file := range files {
        bytesRead, err := ioutil.ReadFile(file)
        LogAndExitOnError(fmt.Sprintf("Error encountered while reading file %s", file), err)
        manifests = append(manifests, bytesRead)
    }

    return manifests
}

func createAssetMap(path string) error {
    asset := *(createAsset())

    log.Log.Info("Getting resources from manifests: ", "path:", path)
    manifests := extractResources(path)

    s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
    regex, _ := regexp.Compile(`\b(\w*kind:\w*)\B.*\b`)

    for _, manifest := range manifests {
        // Fetch the Kind type from the manifest data
        kind := strings.TrimSpace(strings.Split(regex.FindString(string(manifest)), ":")[1])
        log.Log.Info("Resource identified", "kind:", kind)
        _, _, err := s.Decode(manifest, nil, asset.nameToField[kind])
        if err != nil {
            log.Log.Info(fmt.Sprintf("Error encountered while decoding %s resource in manifest %s", asset.nameToField[kind], path), err)
            return err
        }
        mapping := kvpair{key: kind, value: asset.nameToField[kind]}
        asset.objectMappings = append(asset.objectMappings, mapping)
    }

    AssetMap[path] = asset
    return nil
}
