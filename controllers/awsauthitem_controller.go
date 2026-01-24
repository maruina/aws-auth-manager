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

package controllers

import (
	"context"
	"fmt"

	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

// AWSAuthItemReconciler reconciles a AWSAuthItem object.
type AWSAuthItemReconciler struct {
	client.Client
	Scheme                    *runtime.Scheme
	Recorder                  record.EventRecorder
	AWSAuthConfigMapName      string
	AWSAuthConfigMapNamespace string
}

const (
	MapRolesAnnotation = "aws-auth-manager.maruina.k8s/map-roles-sha256"
	MapUsersAnnotation = "aws-auth-manager.maruina.k8s/map-users-sha256"
)

//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *AWSAuthItemReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciliation started")

	// Get the AWSAuthItem
	var item awsauthv1alpha1.AWSAuthItem
	if err := r.Get(ctx, req.NamespacedName, &item); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer) {
		controllerutil.AddFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer)
		if err := r.Update(ctx, &item); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// If the object is being deleted, cleanup and remove the finalizer
	if !item.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, item)
	}

	// Reconcile
	return r.reconcile(ctx, item)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSAuthItemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsauthv1alpha1.AWSAuthItem{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *AWSAuthItemReconciler) findObjectsForConfigMap(ctx context.Context, obj client.Object) []reconcile.Request {
	var itemList awsauthv1alpha1.AWSAuthItemList
	err := r.List(ctx, &itemList)
	if err != nil {
		return []reconcile.Request{}
	}

	// We are only interested in the aws-auth/kube-system configmap
	if obj.GetName() != r.AWSAuthConfigMapName || obj.GetNamespace() != r.AWSAuthConfigMapNamespace {
		return []reconcile.Request{}
	}

	// Trigger a reconciliation loop for all the AWSAuthItem objects
	requests := make([]reconcile.Request, len(itemList.Items))
	for i, item := range itemList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}

	return requests
}

func (r *AWSAuthItemReconciler) reconcile(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Handle suspension
	if item.Spec.Suspend {
		log.Info("reconciliation is suspended for this resource")
		r.Recorder.Event(&item, corev1.EventTypeNormal, awsauthv1alpha1.SuspendedReason,
			"Reconciliation is suspended")
		item.AWSAuthItemSuspended()
		if err := r.patchStatus(ctx, item); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching status for suspended: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// Observe AWSAuthItem generation
	if item.Status.ObservedGeneration != item.Generation {
		item.Status.ObservedGeneration = item.Generation
		item.AWSAuthItemProgressing()
		if err := r.patchStatus(ctx, item); err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("patching status for progressing: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// Get the aws-auth configMap
	var authCm corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Name: r.AWSAuthConfigMapName, Namespace: r.AWSAuthConfigMapNamespace}, &authCm)

	// Create the aws-auth configmap if it doesn't exist
	if errors.IsNotFound(err) {
		authCm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.AWSAuthConfigMapName,
				Namespace: r.AWSAuthConfigMapNamespace,
				Annotations: map[string]string{
					awsauthv1alpha1.AWSAuthAnnotationKey: awsauthv1alpha1.AWSAuthAnnotationValue,
				},
			},
			Data: map[string]string{
				"mapUsers": "",
				"mapRoles": "",
			},
		}

		if err := r.Create(ctx, &authCm); err != nil {
			r.Recorder.Eventf(&item, corev1.EventTypeWarning, awsauthv1alpha1.CreateAwsAuthConfigMapFailedReason,
				"Failed to create aws-auth ConfigMap: %s", err.Error())
			item.AWSAuthItemNotReady(awsauthv1alpha1.CreateAwsAuthConfigMapFailedReason, err.Error())
			if statusErr := r.patchStatus(ctx, item); statusErr != nil {
				log.Error(statusErr, "failed to patch status after ConfigMap creation failure")
			}

			return ctrl.Result{}, fmt.Errorf("creating aws-auth ConfigMap: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// Return if there is an error fetching the aws auth configmap
	if err != nil {
		r.Recorder.Eventf(&item, corev1.EventTypeWarning, awsauthv1alpha1.GetAwsAuthConfigMapFailedReason,
			"Failed to fetch aws-auth ConfigMap: %s", err.Error())
		item.AWSAuthItemNotReady(awsauthv1alpha1.GetAwsAuthConfigMapFailedReason, err.Error())
		if statusErr := r.patchStatus(ctx, item); statusErr != nil {
			log.Error(statusErr, "failed to patch status after ConfigMap fetch failure")
		}

		return ctrl.Result{}, fmt.Errorf("fetching aws-auth ConfigMap: %w", err)
	}

	// Get all the AWSAuthItem
	var itemList awsauthv1alpha1.AWSAuthItemList
	if err := r.List(ctx, &itemList); err != nil {
		item.AWSAuthItemNotReady(awsauthv1alpha1.ListAWSAuthItemFailedReason, err.Error())
		if statusErr := r.patchStatus(ctx, item); statusErr != nil {
			log.Error(statusErr, "failed to patch status after listing AWSAuthItems failure")
		}

		return ctrl.Result{Requeue: true}, fmt.Errorf("listing AWSAuthItems: %w", err)
	}

	// Get all the mapRoles and mapUsers
	var mapRoles []awsauthv1alpha1.MapRoleItem
	var mapUsers []awsauthv1alpha1.MapUserItem
	for _, i := range itemList.Items {
		mapRoles = append(mapRoles, i.Spec.MapRoles...)
		mapUsers = append(mapUsers, i.Spec.MapUsers...)
	}

	// Marshal the objects
	mapRolesYaml, err := yaml.Marshal(mapRoles)
	if err != nil {
		item.AWSAuthItemNotReady(awsauthv1alpha1.MarshalMapRolesFailedReason, err.Error())
		if statusErr := r.patchStatus(ctx, item); statusErr != nil {
			log.Error(statusErr, "failed to patch status after mapRoles marshal failure")
		}

		return ctrl.Result{Requeue: true}, fmt.Errorf("marshaling mapRoles: %w", err)
	}

	mapUsersYaml, err := yaml.Marshal(mapUsers)
	if err != nil {
		item.AWSAuthItemNotReady(awsauthv1alpha1.MarshalMapUsersFailedReason, err.Error())
		if statusErr := r.patchStatus(ctx, item); statusErr != nil {
			log.Error(statusErr, "failed to patch status after mapUsers marshal failure")
		}

		return ctrl.Result{Requeue: true}, fmt.Errorf("marshaling mapUsers: %w", err)
	}

	// Update the configmap using Patch to avoid conflicts
	patch := client.MergeFrom(authCm.DeepCopy())
	authCm.Data["mapRoles"] = string(mapRolesYaml)
	authCm.Data["mapUsers"] = string(mapUsersYaml)

	if err := r.Patch(ctx, &authCm, patch); err != nil {
		r.Recorder.Eventf(&item, corev1.EventTypeWarning, awsauthv1alpha1.UpdateAwsAuthConfigMapFailedReason,
			"Failed to update aws-auth ConfigMap: %s", err.Error())
		item.AWSAuthItemNotReady(awsauthv1alpha1.UpdateAwsAuthConfigMapFailedReason, err.Error())
		if statusErr := r.patchStatus(ctx, item); statusErr != nil {
			log.Error(statusErr, "failed to patch status after ConfigMap update failure")
		}

		return ctrl.Result{Requeue: true}, fmt.Errorf("patching aws-auth ConfigMap: %w", err)
	}

	r.Recorder.Event(&item, corev1.EventTypeNormal, awsauthv1alpha1.ReconciliationSucceededReason,
		"aws-auth ConfigMap updated successfully")
	item.AWSAuthItemReady()
	item.ClearStalledCondition()
	if err := r.patchStatus(ctx, item); err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("patching status for ready: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *AWSAuthItemReconciler) reconcileDelete(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) (ctrl.Result, error) {
	// Reset the annotations to trigger a reconciliation
	authCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.AWSAuthConfigMapName,
			Namespace: r.AWSAuthConfigMapNamespace,
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm); err != nil {
		return ctrl.Result{}, fmt.Errorf("fetching aws-auth ConfigMap during deletion: %w", err)
	}

	patch := client.MergeFrom(authCm.DeepCopy())
	if authCm.Annotations == nil {
		authCm.Annotations = make(map[string]string)
	}

	authCm.Annotations[MapRolesAnnotation] = ""
	authCm.Annotations[MapUsersAnnotation] = ""

	if err := r.Patch(ctx, &authCm, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching aws-auth ConfigMap during deletion: %w", err)
	}

	controllerutil.RemoveFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer)
	if err := r.Update(ctx, &item); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

// patchStatus updates the AWSAuthItem status using a MergeFrom strategy.
func (r *AWSAuthItemReconciler) patchStatus(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) error {
	var latest awsauthv1alpha1.AWSAuthItem

	if err := r.Get(ctx, client.ObjectKeyFromObject(&item), &latest); err != nil {
		return fmt.Errorf("fetching latest AWSAuthItem for status patch: %w", err)
	}

	patch := client.MergeFrom(latest.DeepCopy())
	latest.Status = item.Status

	if err := r.Client.Status().Patch(ctx, &latest, patch); err != nil {
		return fmt.Errorf("patching AWSAuthItem status: %w", err)
	}

	return nil
}
