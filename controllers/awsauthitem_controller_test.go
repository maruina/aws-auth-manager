package controllers

import (
	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("AWSAuthItem controller", func() {
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultEventuallyPollingInterval(eventuallyInterval)
	SetDefaultConsistentlyDuration(consistentlyDuration)
	SetDefaultConsistentlyPollingInterval(consistentlyInterval)

	Context("when creating an AWSAuthItem", func() {
		var item *awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			if item != nil {
				err := k8sClient.Delete(ctx, item)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), item)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		})

		It("should add finalizer", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("finalizer-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/finalizer-user",
							Username: "finalizer-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(&fetched, awsauthv1alpha1.AWSAuthFinalizer)).To(BeTrue())
			}).Should(Succeed())
		})

		It("should create ConfigMap if missing", func() {
			// Delete ConfigMap if it exists
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      reconciler.AWSAuthConfigMapName,
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
			}
			_ = k8sClient.Delete(ctx, cm)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("create-cm-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/cm-test-user",
							Username: "cm-test-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cm).NotTo(BeNil())
				g.Expect(cm.Annotations).To(HaveKeyWithValue(
					awsauthv1alpha1.AWSAuthAnnotationKey,
					awsauthv1alpha1.AWSAuthAnnotationValue,
				))
			}).Should(Succeed())
		})

		It("should update ConfigMap with mapUsers", func() {
			expectedUsers := []awsauthv1alpha1.MapUserItem{
				{
					UserArn:  "arn:aws:iam::111122223333:user/user-test",
					Username: "user-test",
					Groups:   []string{"system:masters"},
				},
			}
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("user-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: expectedUsers,
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElements(expectedUsers))
			}).Should(Succeed())
		})

		It("should update ConfigMap with mapRoles", func() {
			expectedRoles := []awsauthv1alpha1.MapRoleItem{
				{
					RoleArn:  "arn:aws:iam::111122223333:role/role-test",
					Username: "system:node:{{EC2PrivateDNSName}}",
					Groups:   []string{"system:bootstrappers", "system:nodes"},
				},
			}
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("role-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapRoles: expectedRoles,
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				roles, err := getMapRolesFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(roles).To(ContainElements(expectedRoles))
			}).Should(Succeed())
		})

		It("should set Ready condition to True", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("ready-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/ready-user",
							Username: "ready-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				cond := apimeta.FindStatusCondition(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal(awsauthv1alpha1.ReconciliationSucceededReason))
			}).Should(Succeed())
		})

		It("should set ObservedGeneration", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("observed-gen-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/obs-gen-user",
							Username: "obs-gen-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
			}).Should(Succeed())
		})
	})

	Context("when multiple AWSAuthItems exist", func() {
		var items []*awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			for _, item := range items {
				if item != nil {
					err := k8sClient.Delete(ctx, item)
					if err != nil && !apierrors.IsNotFound(err) {
						Expect(err).NotTo(HaveOccurred())
					}
				}
			}
			// Wait for all deletions
			for _, item := range items {
				if item != nil {
					Eventually(func() bool {
						err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), item)
						return apierrors.IsNotFound(err)
					}).Should(BeTrue())
				}
			}
			items = nil
		})

		It("should aggregate all mapRoles and mapUsers", func() {
			userItem := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("aggregate-user"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/agg-admin",
							Username: "agg-admin",
							Groups:   []string{"system:masters"},
						},
						{
							UserArn:  "arn:aws:iam::111122223333:user/agg-ops-user",
							Username: "agg-ops-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			items = append(items, userItem)
			Expect(k8sClient.Create(ctx, userItem)).To(Succeed())

			roleItem := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("aggregate-role"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapRoles: []awsauthv1alpha1.MapRoleItem{
						{
							RoleArn:  "arn:aws:iam::111122223333:role/agg-node-role",
							Username: "system:node:{{EC2PrivateDNSName}}",
							Groups:   []string{"system:bootstrappers", "system:nodes"},
						},
					},
				},
			}
			items = append(items, roleItem)
			Expect(k8sClient.Create(ctx, roleItem)).To(Succeed())

			mixedItem := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("aggregate-mixed"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapRoles: []awsauthv1alpha1.MapRoleItem{
						{
							RoleArn:  "arn:aws:iam::111122223333:role/agg-dev-role",
							Username: "agg-developer",
							Groups:   []string{"tenant"},
						},
					},
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/agg-mixed-user",
							Username: "agg-mixed-user",
							Groups:   []string{"edit", "view"},
						},
					},
				},
			}
			items = append(items, mixedItem)
			Expect(k8sClient.Create(ctx, mixedItem)).To(Succeed())

			// All expected users from userItem and mixedItem
			expectedUsers := append(userItem.Spec.MapUsers, mixedItem.Spec.MapUsers...)
			// All expected roles from roleItem and mixedItem
			expectedRoles := append(roleItem.Spec.MapRoles, mixedItem.Spec.MapRoles...)

			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())

				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElements(expectedUsers))

				roles, err := getMapRolesFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(roles).To(ContainElements(expectedRoles))
			}).Should(Succeed())

			// Verify all items are Ready
			for _, item := range items {
				Eventually(func(g Gomega) {
					var fetched awsauthv1alpha1.AWSAuthItem
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
					g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				}).Should(Succeed())
			}
		})
	})

	Context("when updating an AWSAuthItem", func() {
		var item *awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			if item != nil {
				err := k8sClient.Delete(ctx, item)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), item)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		})

		It("should update ConfigMap with new values", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("update-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/update-original",
							Username: "update-original",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			// Wait for Ready
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
			}).Should(Succeed())

			// Update the item
			var toUpdate awsauthv1alpha1.AWSAuthItem
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate)).To(Succeed())
			toUpdate.Spec.MapUsers = []awsauthv1alpha1.MapUserItem{
				{
					UserArn:  "arn:aws:iam::111122223333:user/update-modified",
					Username: "update-modified",
					Groups:   []string{"view"},
				},
			}
			Expect(k8sClient.Update(ctx, &toUpdate)).To(Succeed())

			// Verify ConfigMap was updated
			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
					UserArn:  "arn:aws:iam::111122223333:user/update-modified",
					Username: "update-modified",
					Groups:   []string{"view"},
				}))
				// Original should not be present
				for _, u := range users {
					g.Expect(u.UserArn).NotTo(Equal("arn:aws:iam::111122223333:user/update-original"))
				}
			}).Should(Succeed())
		})
	})

	Context("when deleting an AWSAuthItem", func() {
		var remainingItem *awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			if remainingItem != nil {
				err := k8sClient.Delete(ctx, remainingItem)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(remainingItem), remainingItem)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		})

		It("should remove entries from ConfigMap and remove finalizer", func() {
			// Create a remaining item to ensure cleanup reconciliation triggers
			// (the controller relies on other items' reconciliation to clean up ConfigMap)
			remainingItem = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("remaining-item"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/remaining-user",
							Username: "remaining-user",
							Groups:   []string{"view"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, remainingItem)).To(Succeed())

			// Wait for remaining item to be Ready
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(remainingItem), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
			}).Should(Succeed())

			// Create the item to be deleted
			itemToDelete := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("delete-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/delete-user",
							Username: "delete-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, itemToDelete)).To(Succeed())

			// Wait for Ready and data in ConfigMap
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(itemToDelete), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())

				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
					UserArn:  "arn:aws:iam::111122223333:user/delete-user",
					Username: "delete-user",
					Groups:   []string{"system:masters"},
				}))
			}).Should(Succeed())

			// Delete the item
			Expect(k8sClient.Delete(ctx, itemToDelete)).To(Succeed())

			// Verify item is deleted (finalizer was removed)
			Eventually(func() bool {
				var fetched awsauthv1alpha1.AWSAuthItem
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(itemToDelete), &fetched)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			// Verify deleted user was removed from ConfigMap but remaining user still present
			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())

				// Deleted user should be gone
				for _, u := range users {
					g.Expect(u.UserArn).NotTo(Equal("arn:aws:iam::111122223333:user/delete-user"))
				}
				// Remaining user should still exist
				g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
					UserArn:  "arn:aws:iam::111122223333:user/remaining-user",
					Username: "remaining-user",
					Groups:   []string{"view"},
				}))
			}).Should(Succeed())

			// Verify state remains consistent
			Consistently(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				for _, u := range users {
					g.Expect(u.UserArn).NotTo(Equal("arn:aws:iam::111122223333:user/delete-user"))
				}
			}).Should(Succeed())
		})
	})

	Context("when suspended", func() {
		var item *awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			if item != nil {
				// Unsuspend before deletion to ensure cleanup
				var toUpdate awsauthv1alpha1.AWSAuthItem
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate); err == nil {
					toUpdate.Spec.Suspend = false
					_ = k8sClient.Update(ctx, &toUpdate)
				}

				err := k8sClient.Delete(ctx, item)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), item)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		})

		It("should set Ready=False with Suspended reason", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("suspended-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					Suspend: true,
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/suspended-user",
							Username: "suspended-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())

				cond := apimeta.FindStatusCondition(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(awsauthv1alpha1.SuspendedReason))
			}).Should(Succeed())
		})

		It("should resume reconciliation when unsuspended", func() {
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("resume-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					Suspend: true,
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/resume-user",
							Username: "resume-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			// Wait for Suspended status
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				cond := apimeta.FindStatusCondition(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal(awsauthv1alpha1.SuspendedReason))
			}).Should(Succeed())

			// Unsuspend
			var toUpdate awsauthv1alpha1.AWSAuthItem
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate)).To(Succeed())
			toUpdate.Spec.Suspend = false
			Expect(k8sClient.Update(ctx, &toUpdate)).To(Succeed())

			// Verify it becomes Ready
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	Context("when ConfigMap is modified externally", func() {
		var item *awsauthv1alpha1.AWSAuthItem

		AfterEach(func() {
			if item != nil {
				err := k8sClient.Delete(ctx, item)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), item)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		})

		It("should reconcile back to desired state", func() {
			expectedUser := awsauthv1alpha1.MapUserItem{
				UserArn:  "arn:aws:iam::111122223333:user/external-mod-user",
				Username: "external-mod-user",
				Groups:   []string{"system:masters"},
			}
			item = &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("external-mod-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{expectedUser},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())

			// Wait for Ready and verify ConfigMap has correct data
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())

				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElement(expectedUser))
			}).Should(Succeed())

			// Externally modify ConfigMap
			cm, err := getAWSAuthConfigMap()
			Expect(err).NotTo(HaveOccurred())
			patch := client.MergeFrom(cm.DeepCopy())
			cm.Data["mapUsers"] = "corrupted-data"
			Expect(k8sClient.Patch(ctx, cm, patch)).To(Succeed())

			// Verify ConfigMap is reconciled back to desired state
			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElement(expectedUser))
			}).Should(Succeed())
		})
	})
})
