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
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var awsauthitemlog = logf.Log.WithName("awsauthitem-resource")

func (r *AWSAuthItem) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy[*AWSAuthItem](mgr, r).
		WithValidator(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-aws-maruina-k8s-v1alpha1-awsauthitem,mutating=false,failurePolicy=fail,sideEffects=None,groups=aws.maruina.k8s,resources=awsauthitems,verbs=create;update,versions=v1alpha1,name=vawsauthitem.aws.maruina.k8s,admissionReviewVersions=v1

var _ admission.Validator[*AWSAuthItem] = &AWSAuthItem{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type.
func (r *AWSAuthItem) ValidateCreate(_ context.Context, _ *AWSAuthItem) (admission.Warnings, error) {
	awsauthitemlog.Info("validate create", "name", r.Name)

	return nil, r.validateAWSAuthItem()
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type.
func (r *AWSAuthItem) ValidateUpdate(_ context.Context, _, _ *AWSAuthItem) (admission.Warnings, error) {
	awsauthitemlog.Info("validate update", "name", r.Name)

	return nil, r.validateAWSAuthItem()
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type.
func (r *AWSAuthItem) ValidateDelete(_ context.Context, _ *AWSAuthItem) (admission.Warnings, error) {
	return nil, nil
}

func (r *AWSAuthItem) validateAWSAuthItem() error {
	var allErrs field.ErrorList

	if errs := r.validateArns(); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "aws.maruina.k8s", Kind: "AWSAuthItem"},
		r.Name, allErrs)
}

func (r *AWSAuthItem) validateArns() field.ErrorList {
	var errList field.ErrorList

	for _, mapRole := range r.Spec.MapRoles {
		if !arn.IsARN(mapRole.RoleArn) {
			errList = append(errList, field.Invalid(field.NewPath("spec").Child("MapRoles"), mapRole.RoleArn, "invalid role ARN"))
		}
	}

	for _, mapUser := range r.Spec.MapUsers {
		if !arn.IsARN(mapUser.UserArn) {
			errList = append(errList, field.Invalid(field.NewPath("spec").Child("MapUsers"), mapUser.UserArn, "invalid user ARN"))
		}
	}

	return errList
}
