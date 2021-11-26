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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const AWSAuthFinalizer = "finalizer.aws.maruina.k8s"

// AWSAuthItemSpec defines the desired state of AWSAuthItem
type AWSAuthItemSpec struct {
	// MapRoles holds a list of MapRoleItem objects
	MapRoles []MapRoleItem `json:"mapRoles,omitempty"`

	// MapUsers holds a list of MapUserItem objects
	MapUsers []MapUserItem `json:"mapUsers,omitempty"`
}

type MapRoleItem struct {
	// The ARN of the IAM role to add.
	RoleArn string `json:"rolearn"`

	// The user name within Kubernetes to map to the IAM role.
	Username string `json:"username"`

	// A list of groups within Kubernetes to which the role is mapped.
	Groups []string `json:"groups"`
}

type MapUserItem struct {
	// The ARN of the IAM user to add.
	UserArn string `json:"userarn"`

	// The user name within Kubernetes to map to the IAM user.
	Username string `json:"username"`

	// A list of groups within Kubernetes to which the user is mapped to.
	Groups []string `json:"groups"`
}

// AWSAuthItemStatus defines the observed state of AWSAuthItem
type AWSAuthItemStatus struct {
	// ObservedGeneration is the last observed generation.
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the AWS Auth Item.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AWSAuthItem is the Schema for the awsauthitems API
type AWSAuthItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AWSAuthItemSpec   `json:"spec,omitempty"`
	Status AWSAuthItemStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AWSAuthItemList contains a list of AWSAuthItem
type AWSAuthItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSAuthItem `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AWSAuthItem{}, &AWSAuthItemList{})
}
