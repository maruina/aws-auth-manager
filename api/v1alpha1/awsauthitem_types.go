/*
Copyright 2021.

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

package v1alpha1

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AWSAuthFinalizer       = "finalizer.aws-auth-manager.maruina.k8s"
	AWSAuthAnnotationKey   = "aws-auth-manager.maruina.k8s/managed"
	AWSAuthAnnotationValue = "true"
)

const (
	// ReadyCondition is the name of the Ready condition implemented by all toolkit
	// resources.
	ReadyCondition string = "Ready"

	// ReconcilingCondition is the name of the Reconciling kstatus condition.
	ReconcilingCondition string = "Reconciling"
)

const (
	// ReconciliationSucceededReason represents the fact that the reconciliation of
	// a toolkit resource has succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"

	// ReconciliationFailedReason represents the fact that the reconciliation of a
	// toolkit resource has failed.
	ReconciliationFailedReason string = "ReconciliationFailed"

	// ProgressingReason represents the fact that the reconciliation of a toolkit
	// resource is underway.
	ProgressingReason string = "Progressing"

	// SuspendedReason represents the fact that the reconciliation of a toolkit
	// resource is suspended.
	SuspendedReason string = "Suspended"
)

// AWSAuthItemSpec defines the desired state of AWSAuthItem.
type AWSAuthItemSpec struct {
	// MapRoles holds a list of MapRoleItem
	//+kubebuilder:validation:Optional
	MapRoles []MapRoleItem `json:"mapRoles,omitempty"`

	// MapUsers holds a list of MapUserItem
	//+kubebuilder:validation:Optional
	MapUsers []MapUserItem `json:"mapUsers,omitempty"`
}

type MapRoleItem struct {
	// The ARN of the IAM role to add
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinLength=25
	RoleArn string `json:"rolearn"`

	// The user name within Kubernetes to map to the IAM role
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinLength=1
	Username string `json:"username"`

	// A list of groups within Kubernetes to which the role is mapped
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinItems=1
	Groups []string `json:"groups"`
}

type MapUserItem struct {
	// The ARN of the IAM user to add
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinLength=25
	UserArn string `json:"userarn"`

	// The user name within Kubernetes to map to the IAM user
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinLength=1
	Username string `json:"username"`

	// A list of groups within Kubernetes to which the user is mapped to
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:MinItems=1
	Groups []string `json:"groups"`
}

// AWSAuthItemStatus defines the observed state of AWSAuthItem.
type AWSAuthItemStatus struct {
	// ObservedGeneration is the last observed generation.
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the AWSAuthItem.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AWSAuthItemProgressing registers progress toward
// reconciling the given AWSAuthItem by setting the meta.ReadyCondition to
// 'Unknown' for meta.ProgressingReason.
func (r *AWSAuthItem) AWSAuthItemProgressing() {
	r.Status.Conditions = []metav1.Condition{}
	r.SetResourceCondition(ReadyCondition, metav1.ConditionUnknown, ProgressingReason,
		"Reconciliation in progress")
}

// AWSAuthItemNotReady registers a failed reconciliation of the given AWSAuthItem.
func (r *AWSAuthItem) AWSAuthItemNotReady(reason, message string) {
	r.SetResourceCondition(ReadyCondition, metav1.ConditionFalse, reason, message)
}

// AWSAuthItemReady registers a successful reconciliation of the given AWSAuthItem.
func (r *AWSAuthItem) AWSAuthItemReady() {
	r.SetResourceCondition(ReadyCondition, metav1.ConditionTrue, ReconciliationSucceededReason,
		"Item reconciliation succeeded")
}

// SetResourceCondition sets the given condition with the given status,
// reason and message on a resource.
func (r *AWSAuthItem) SetResourceCondition(condition string, status metav1.ConditionStatus, reason, message string) {
	conditions := r.GetStatusConditions()

	newCondition := metav1.Condition{
		Type:    condition,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	apimeta.SetStatusCondition(conditions, newCondition)
}

// GetStatusConditions returns a pointer to the Status.Conditions slice.
func (r *AWSAuthItem) GetStatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AWSAuthItem is the Schema for the awsauthitems API.
type AWSAuthItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AWSAuthItemSpec   `json:"spec,omitempty"`
	Status AWSAuthItemStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AWSAuthItemList contains a list of AWSAuthItem.
type AWSAuthItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSAuthItem `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AWSAuthItem{}, &AWSAuthItemList{})
}
