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
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
)

// AWSAuthItemReconciler reconciles a AWSAuthItem object
type AWSAuthItemReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	AWSAuthMapName      = "aws-auth"
	AWSAuthMapNamespace = "kube-system"
	MapRolesAnnotation  = "aws-auth-manager.maruina.k8s/map-roles-sha256"
	MapUsersAnnotation  = "aws-auth-manager.maruina.k8s/map-users-sha256"
)

//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.maruina.k8s,resources=awsauthitems/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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
		log.Error(err, "unable to fetch the AWSAuthItem object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer) {
		controllerutil.AddFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer)
		if err := r.Update(ctx, &item); err != nil {
			log.Error(err, "unable to update the AWSAuthItem object when adding finalizer")
			return ctrl.Result{}, err
		}
	}

	// If the object is being deleted, cleanup and remove the finalizer
	if !item.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, item)
	}

	// Reconcile
	reconciledItem, reconciledResult, reconciledError := r.reconcile(ctx, item)

	// Update status after reconciliation.
	if updateStatusErr := r.patchStatus(ctx, reconciledItem); updateStatusErr != nil {
		log.Error(updateStatusErr, "unable to update status after reconciliation")
		return ctrl.Result{Requeue: true}, updateStatusErr
	}

	return reconciledResult, reconciledError
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSAuthItemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsauthv1alpha1.AWSAuthItem{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *AWSAuthItemReconciler) findObjectsForConfigMap(configMap client.Object) []reconcile.Request {
	var itemList awsauthv1alpha1.AWSAuthItemList
	err := r.List(context.TODO(), &itemList)
	if err != nil {
		return []reconcile.Request{}
	}

	// We are only interested in the aws-auth/kube-system configmap
	if configMap.GetName() != AWSAuthMapName || configMap.GetNamespace() != AWSAuthMapNamespace {
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

func (r *AWSAuthItemReconciler) reconcile(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) (awsauthv1alpha1.AWSAuthItem, ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Observe AWSAuthItem generation
	if item.Status.ObservedGeneration != item.Generation {
		item.Status.ObservedGeneration = item.Generation
		return awsauthv1alpha1.AWSAuthItemProgressing(item), ctrl.Result{Requeue: true}, nil
	}

	// Get the aws-auth configMap
	var authCm corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: AWSAuthMapName, Namespace: AWSAuthMapNamespace}, &authCm); err != nil {
		// Create the aws-auth configmap if it doesn't exist
		if errors.IsNotFound(err) {
			authCm = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        AWSAuthMapName,
					Namespace:   AWSAuthMapNamespace,
					Annotations: make(map[string]string),
				},
				Data: map[string]string{
					"MapUsers": "",
					"MapRoles": "",
				},
			}

			if createErr := r.Create(ctx, &authCm); createErr != nil {
				log.Error(createErr, "unable to created aws-auth configmap")
				return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.CreateAwsAuthConfigMapFailedReason, err.Error()),
					ctrl.Result{Requeue: true}, createErr
			}
		} else {
			log.Error(err, "unable to fetch the aws-auth ConfigMap object")
			return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.GetAwsAuthConfigMapFailedReason, err.Error()),
				ctrl.Result{Requeue: true}, err
		}

	}

	// Get all the AWSAuthItem
	var itemList awsauthv1alpha1.AWSAuthItemList
	if err := r.List(ctx, &itemList); err != nil {
		log.Error(err, "unable to list the AWSAuthItem objects")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.ListAWSAuthItemFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
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
		log.Error(err, "unable to marshal mapRoles")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.MarshalMapRolesFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}

	mapUsersYaml, err := yaml.Marshal(mapUsers)
	if err != nil {
		log.Error(err, "unable to marshal mapUsers")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.MarshalMapUsersFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}

	// Calculate hash
	rolesHasher := sha256.New()
	if _, err := rolesHasher.Write([]byte(mapRolesYaml)); err != nil {
		log.Error(err, "unable to hash marshalled mapRoles")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.HashMapRolesFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}
	rolesHash := hex.EncodeToString(rolesHasher.Sum(nil))

	rolesHasher.Reset()

	if _, err := rolesHasher.Write([]byte(authCm.Data["MapRoles"])); err != nil {
		log.Error(err, "unable to hash MapRoles from configmap")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.HashMapRolesFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}
	currentRolesHash := hex.EncodeToString(rolesHasher.Sum(nil))

	usersHasher := sha256.New()
	if _, err := usersHasher.Write([]byte(mapUsersYaml)); err != nil {
		log.Error(err, "unable to hash marshalled mapUsers")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.HashMapUsersFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}
	usersHash := hex.EncodeToString(usersHasher.Sum(nil))

	usersHasher.Reset()

	if _, err := usersHasher.Write([]byte(authCm.Data["MapUsers"])); err != nil {
		log.Error(err, "unable to hash MapUsers from configmap")
		return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.HashMapUsersFailedReason, err.Error()),
			ctrl.Result{Requeue: true}, err
	}
	currentUsersHash := hex.EncodeToString(usersHasher.Sum(nil))

	// If the hash is different we need to update the configmap
	if rolesHash != currentRolesHash || usersHash != currentUsersHash {
		authCm.Data["MapRoles"] = string(mapRolesYaml)
		authCm.Data["MapUsers"] = string(mapUsersYaml)

		if authCm.ObjectMeta.Annotations == nil {
			authCm.ObjectMeta.Annotations = make(map[string]string)
		}
		authCm.ObjectMeta.Annotations[MapRolesAnnotation] = rolesHash
		authCm.ObjectMeta.Annotations[MapUsersAnnotation] = usersHash

		if err := r.Update(ctx, &authCm); err != nil {
			log.Error(err, "unable to update the aws-auth ConfigMap")
			return awsauthv1alpha1.AWSAuthItemNotReady(item, awsauthv1alpha1.UpdateAwsAuthConfigMapFailedReason, err.Error()),
				ctrl.Result{Requeue: true}, err
		}
	}

	return awsauthv1alpha1.AWSAuthItemReady(item), ctrl.Result{}, nil
}

func (r *AWSAuthItemReconciler) reconcileDelete(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Reset the annotations on to trigger a reconciliation
	authCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AWSAuthMapName,
			Namespace: AWSAuthMapNamespace,
		},
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm); err != nil {
		log.Error(err, "unabled to fetch the aws-auth configamp when deleting the AWSAuthItem object")
		return ctrl.Result{}, err
	}

	authCm.ObjectMeta.Annotations[MapRolesAnnotation] = ""
	authCm.ObjectMeta.Annotations[MapUsersAnnotation] = ""

	if err := r.Update(ctx, &authCm); err != nil {
		log.Error(err, "unable to update the aws-auth configmap when deleting the AWSAuthItem object")
	}

	controllerutil.RemoveFinalizer(&item, awsauthv1alpha1.AWSAuthFinalizer)
	if err := r.Update(ctx, &item); err != nil {
		log.Error(err, "unabled to remove finalizer when deleting the AWSAuthItem object")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

//patchStatus updates the AWSAuthItem using a MergeFrom strategy
func (r *AWSAuthItemReconciler) patchStatus(ctx context.Context, item awsauthv1alpha1.AWSAuthItem) error {
	var latest awsauthv1alpha1.AWSAuthItem

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&item), &latest); err != nil {
		return err
	}

	patch := client.MergeFrom(latest.DeepCopy())
	latest.Status = item.Status

	return r.Client.Status().Patch(ctx, &latest, patch)
}
