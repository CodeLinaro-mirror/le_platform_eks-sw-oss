/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear

Copyright 2023.

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


package v1

import (
    "strings"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// INSERT ADDITIONAL SPEC/Status FIELDS
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// X100ManagementPolicySpec defines the desired state of X100ManagementPolicy
type X100ManagementPolicySpec struct {
    Firmware      ComponentSpec `json:"firmware"`
    HwManager     ComponentSpec `json:"hwManager"`
    KModule       KModuleSpec   `json:"kModule"`
    DevicePlugin  ComponentSpec `json:"devicePlugin"`
}

// ComponentSpec defines the properties for individual x100 operator components
type ComponentSpec struct {
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
    Repository string `json:"repository"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\-]+
    Image string `json:"image"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
    Version string `json:"version"`
    // Image pull policy
    // +kubebuilder:validation:Optional
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Image Pull Policy"
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:imagePullPolicy"
    ImagePullPolicy string `json:"imagePullPolicy,omitempty"`
    // Image pull secrets
    // +kubebuilder:validation:Optional
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Image pull secrets"
    // +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes:Secret"
    ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
}

type KModuleSpec struct {
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-_]+
    Repository string `json:"repository"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\-]+
    Image string `json:"image"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
    Version string `json:"version"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
    Tag             string `json:"tag"`
    // +kubebuilder:validation:Pattern=[a-zA-Z0-9\.-]+
    ImageRepoSecret string `json:"imageRepoSecret"`
}

type State string

const (
    Ignored        State = "ignored"
    Operational    State = "ready"
    NotOperational State = "notReady"
)

// x100ManagementPolicyStatus defines the observed state of X100ManagementPolicy
type X100ManagementPolicyStatus struct {
    // +kubebuilder:validation:Enum=ignored;ready;notReady
    State State `json:"state"`
}

// Need API marker to generate the Object interface for
// converting go types to kubernetes objects
// +kubebuilder:object:root=true

// X100ManagementPolicy allows you to configure the x100 Operator
// +kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
type X100ManagementPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   X100ManagementPolicySpec   `json:"spec,omitempty"`
    Status X100ManagementPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// X100ManagementPolicyList contains a list of X100ManagementPolicy
type X100ManagementPolicyList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []X100ManagementPolicy `json:"items"`
}

func init() {
    SchemeBuilder.Register(&X100ManagementPolicy{}, &X100ManagementPolicyList{})
}

func (m *X100ManagementPolicy) SetState(s State) {
    m.Status.State = s
}

func (k *KModuleSpec) KModuleImagePath() string {

    if strings.HasPrefix(k.Version, "sha256:") {
        return k.Repository + "/" + k.Image + "@" + k.Version
    }
    return k.Repository + "/" + k.Image + ":" + k.Version + "-${KERNEL_FULL_VERSION}"
}

// Helper to make a complete Image path
// Component spec identifies individual components of path as
// repo, image, version
// May replace this with an ImageURL unless this kind of separation
// has a usecase for us
func (c *ComponentSpec) ImagePath() string {
    // use @ if image digest is specified instead of tag
    if strings.HasPrefix(c.Version, "sha256:") {
        return c.Repository + "/" + c.Image + "@" + c.Version
    }
    return c.Repository + "/" + c.Image + ":" + c.Version
}

// Mark the pull policy based on string representation
// Always / Never / IfNotPresent

func (c *ComponentSpec) ImagePolicy(pullPolicy string) corev1.PullPolicy {
    var imagePullPolicy corev1.PullPolicy

    switch pullPolicy {
    case "Always":
        imagePullPolicy = corev1.PullAlways
    case "Never":
        imagePullPolicy = corev1.PullNever
    case "IfNotPresent":
        imagePullPolicy = corev1.PullIfNotPresent
    default:
        imagePullPolicy = corev1.PullAlways
    }

    return imagePullPolicy
}
