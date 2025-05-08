/*
Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.
SPDX-License-Identifier: BSD-3-Clause-Clear

Copyright 2025.

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
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var k8Client client.Client

func (r *X100ManagementPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	k8Client = mgr.GetClient() // Save client for use in validation
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-qualcomm-com-v1-x100managementpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=qualcomm.com,resources=x100managementpolicies,verbs=create;update,versions=v1,name=vx100managementpolicy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &X100ManagementPolicy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *X100ManagementPolicy) ValidateCreate() (admission.Warnings, error) {
	log.Log.Info("validate create", "name", r.Name)
	err := r.ValidateSwVersionUniqueness()
	return nil, err
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *X100ManagementPolicy) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	log.Log.Info("validate update", "name", r.Name)
	err := r.ValidateSwVersionUniqueness()
	return nil, err
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *X100ManagementPolicy) ValidateDelete() (admission.Warnings, error) {
	log.Log.Info("validate delete", "name", r.Name)
	return nil, nil
}

func (r *X100ManagementPolicy) ValidateSwVersionUniqueness() error {
	policyopts := []client.ListOption{}
	policyList := &X100ManagementPolicyList{}

	if err := k8Client.List(context.TODO(), policyList, policyopts...); err != nil {
		return fmt.Errorf("failed to list policies: %v", err)
	}

	for _, policy := range policyList.Items {
		if policy.Name != r.Name && policy.Spec.SwVersion == r.Spec.SwVersion {
			return fmt.Errorf("SwVersion '%s' is already used by policy '%s'", r.Spec.SwVersion, policy.Name)
		}
	}
	return nil
}
