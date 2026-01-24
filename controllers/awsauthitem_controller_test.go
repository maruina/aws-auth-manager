package controllers

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
)

// cleanupAWSAuthItem deletes the item and waits for deletion to complete.
// Intended for use with DeferCleanup.
func cleanupAWSAuthItem(item *awsauthv1alpha1.AWSAuthItem) {
	if item == nil {
		return
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

var _ = Describe("AWSAuthItem controller", func() {
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultEventuallyPollingInterval(eventuallyInterval)
	SetDefaultConsistentlyDuration(consistentlyDuration)
	SetDefaultConsistentlyPollingInterval(consistentlyInterval)

	Context("when creating an AWSAuthItem", func() {
		It("should add finalizer", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, item)

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

			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, item)

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

		It("should set Ready condition to True", func() {
			drainEvents()

			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, item)

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				cond := apimeta.FindStatusCondition(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal(awsauthv1alpha1.ReconciliationSucceededReason))
			}).Should(Succeed())

			// Verify event was emitted
			Eventually(func() bool {
				select {
				case event := <-fakeRecorder.Events:
					return strings.Contains(event, "ReconciliationSucceeded")
				default:
					return false
				}
			}).Should(BeTrue())
		})

		It("should set ObservedGeneration", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, item)

			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
			}).Should(Succeed())
		})

		It("should handle empty AWSAuthItem without mapUsers or mapRoles", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("empty-spec-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					// Empty spec - no mapUsers or mapRoles
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, item)

			// Should become Ready
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
			}).Should(Succeed())

			// ConfigMap should exist and be valid (not corrupted)
			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cm).NotTo(BeNil())
				// Verify data is valid YAML (or empty)
				_, err = getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				_, err = getMapRolesFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())
		})

		// Table-driven tests for mapUsers and mapRoles
		DescribeTable("should update ConfigMap with mapped entries",
			func(createSpec func() awsauthv1alpha1.AWSAuthItemSpec,
				validate func(g Gomega, cm *corev1.ConfigMap)) {
				spec := createSpec()
				item := &awsauthv1alpha1.AWSAuthItem{
					ObjectMeta: metav1.ObjectMeta{
						Name:      uniqueName("table-test"),
						Namespace: reconciler.AWSAuthConfigMapNamespace,
					},
					Spec: spec,
				}
				Expect(k8sClient.Create(ctx, item)).To(Succeed())
				DeferCleanup(cleanupAWSAuthItem, item)

				Eventually(func(g Gomega) {
					cm, err := getAWSAuthConfigMap()
					g.Expect(err).NotTo(HaveOccurred())
					validate(g, cm)
				}).Should(Succeed())
			},
			Entry("mapUsers only",
				func() awsauthv1alpha1.AWSAuthItemSpec {
					return awsauthv1alpha1.AWSAuthItemSpec{
						MapUsers: []awsauthv1alpha1.MapUserItem{
							{
								UserArn:  "arn:aws:iam::111122223333:user/table-user",
								Username: "table-user",
								Groups:   []string{"system:masters"},
							},
						},
					}
				},
				func(g Gomega, cm *corev1.ConfigMap) {
					users, err := getMapUsersFromConfigMap(cm)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
						UserArn:  "arn:aws:iam::111122223333:user/table-user",
						Username: "table-user",
						Groups:   []string{"system:masters"},
					}))
				},
			),
			Entry("mapRoles only",
				func() awsauthv1alpha1.AWSAuthItemSpec {
					return awsauthv1alpha1.AWSAuthItemSpec{
						MapRoles: []awsauthv1alpha1.MapRoleItem{
							{
								RoleArn:  "arn:aws:iam::111122223333:role/table-role",
								Username: "system:node:{{EC2PrivateDNSName}}",
								Groups:   []string{"system:bootstrappers", "system:nodes"},
							},
						},
					}
				},
				func(g Gomega, cm *corev1.ConfigMap) {
					roles, err := getMapRolesFromConfigMap(cm)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(roles).To(ContainElement(awsauthv1alpha1.MapRoleItem{
						RoleArn:  "arn:aws:iam::111122223333:role/table-role",
						Username: "system:node:{{EC2PrivateDNSName}}",
						Groups:   []string{"system:bootstrappers", "system:nodes"},
					}))
				},
			),
			Entry("both mapUsers and mapRoles",
				func() awsauthv1alpha1.AWSAuthItemSpec {
					return awsauthv1alpha1.AWSAuthItemSpec{
						MapUsers: []awsauthv1alpha1.MapUserItem{
							{
								UserArn:  "arn:aws:iam::111122223333:user/both-user",
								Username: "both-user",
								Groups:   []string{"view"},
							},
						},
						MapRoles: []awsauthv1alpha1.MapRoleItem{
							{
								RoleArn:  "arn:aws:iam::111122223333:role/both-role",
								Username: "both-role-user",
								Groups:   []string{"edit"},
							},
						},
					}
				},
				func(g Gomega, cm *corev1.ConfigMap) {
					users, err := getMapUsersFromConfigMap(cm)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
						UserArn:  "arn:aws:iam::111122223333:user/both-user",
						Username: "both-user",
						Groups:   []string{"view"},
					}))
					roles, err := getMapRolesFromConfigMap(cm)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(roles).To(ContainElement(awsauthv1alpha1.MapRoleItem{
						RoleArn:  "arn:aws:iam::111122223333:role/both-role",
						Username: "both-role-user",
						Groups:   []string{"edit"},
					}))
				},
			),
		)
	})

	Context("when multiple AWSAuthItems exist", func() {
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
			Expect(k8sClient.Create(ctx, userItem)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, userItem)

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
			Expect(k8sClient.Create(ctx, roleItem)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, roleItem)

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
			Expect(k8sClient.Create(ctx, mixedItem)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, mixedItem)

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
			items := []*awsauthv1alpha1.AWSAuthItem{userItem, roleItem, mixedItem}
			for _, item := range items {
				Eventually(func(g Gomega) {
					var fetched awsauthv1alpha1.AWSAuthItem
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
					g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				}).Should(Succeed())
			}
		})

		It("should aggregate items from different namespaces", func() {
			// Create a namespace for this test
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: uniqueName("test-ns"),
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, testNs)
			})

			// Item in kube-system (same as ConfigMap)
			kubeSystemItem := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("ns-kube-system"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/kube-system-user",
							Username: "kube-system-user",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, kubeSystemItem)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, kubeSystemItem)

			// Item in different namespace
			otherNsItem := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("ns-other"),
					Namespace: testNs.Name,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/other-ns-user",
							Username: "other-ns-user",
							Groups:   []string{"view"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, otherNsItem)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, otherNsItem)

			// Verify both items contribute to ConfigMap
			Eventually(func(g Gomega) {
				cm, err := getAWSAuthConfigMap()
				g.Expect(err).NotTo(HaveOccurred())

				users, err := getMapUsersFromConfigMap(cm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
					UserArn:  "arn:aws:iam::111122223333:user/kube-system-user",
					Username: "kube-system-user",
					Groups:   []string{"system:masters"},
				}))
				g.Expect(users).To(ContainElement(awsauthv1alpha1.MapUserItem{
					UserArn:  "arn:aws:iam::111122223333:user/other-ns-user",
					Username: "other-ns-user",
					Groups:   []string{"view"},
				}))
			}).Should(Succeed())

			// Verify both items become Ready
			for _, item := range []*awsauthv1alpha1.AWSAuthItem{kubeSystemItem, otherNsItem} {
				Eventually(func(g Gomega) {
					var fetched awsauthv1alpha1.AWSAuthItem
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
					g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				}).Should(Succeed())
			}
		})

	})

	Context("when updating an AWSAuthItem", func() {
		It("should update ConfigMap with new values", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, item)

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

		It("should update ObservedGeneration after spec change", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("obs-gen-update-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{
						{
							UserArn:  "arn:aws:iam::111122223333:user/obs-gen-original",
							Username: "obs-gen-original",
							Groups:   []string{"system:masters"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, item)

			// Wait for Ready and record initial generation
			var initialGeneration int64
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
				initialGeneration = fetched.Generation
			}).Should(Succeed())

			// Update spec (change mapUsers)
			var toUpdate awsauthv1alpha1.AWSAuthItem
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate)).To(Succeed())
			toUpdate.Spec.MapUsers = []awsauthv1alpha1.MapUserItem{
				{
					UserArn:  "arn:aws:iam::111122223333:user/obs-gen-updated",
					Username: "obs-gen-updated",
					Groups:   []string{"view"},
				},
			}
			Expect(k8sClient.Update(ctx, &toUpdate)).To(Succeed())

			// Verify ObservedGeneration equals new Generation
			Eventually(func(g Gomega) {
				var fetched awsauthv1alpha1.AWSAuthItem
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &fetched)).To(Succeed())
				// Generation should have increased
				g.Expect(fetched.Generation).To(BeNumerically(">", initialGeneration))
				// ObservedGeneration should match new Generation after reconciliation
				g.Expect(apimeta.IsStatusConditionTrue(fetched.Status.Conditions, awsauthv1alpha1.ReadyCondition)).To(BeTrue())
				g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
			}).Should(Succeed())
		})
	})

	Context("when deleting an AWSAuthItem", func() {
		It("should remove entries from ConfigMap and remove finalizer", func() {
			// Create a remaining item to ensure cleanup reconciliation triggers
			// (the controller relies on other items' reconciliation to clean up ConfigMap)
			remainingItem := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(cleanupAWSAuthItem, remainingItem)

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
		It("should set Ready=False with Suspended reason", func() {
			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(func() {
				// Unsuspend before deletion to ensure cleanup
				var toUpdate awsauthv1alpha1.AWSAuthItem
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate); err == nil {
					toUpdate.Spec.Suspend = false
					_ = k8sClient.Update(ctx, &toUpdate)
				}
				cleanupAWSAuthItem(item)
			})

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
			item := &awsauthv1alpha1.AWSAuthItem{
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
			DeferCleanup(func() {
				// Unsuspend before deletion to ensure cleanup
				var toUpdate awsauthv1alpha1.AWSAuthItem
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(item), &toUpdate); err == nil {
					toUpdate.Spec.Suspend = false
					_ = k8sClient.Update(ctx, &toUpdate)
				}
				cleanupAWSAuthItem(item)
			})

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

	// This test implicitly verifies the findObjectsForConfigMap watch handler
	// by confirming that external ConfigMap modifications trigger reconciliation
	// of all AWSAuthItems that reference it.
	Context("when ConfigMap is modified externally", func() {
		It("should reconcile back to desired state", func() {
			expectedUser := awsauthv1alpha1.MapUserItem{
				UserArn:  "arn:aws:iam::111122223333:user/external-mod-user",
				Username: "external-mod-user",
				Groups:   []string{"system:masters"},
			}
			item := &awsauthv1alpha1.AWSAuthItem{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueName("external-mod-test"),
					Namespace: reconciler.AWSAuthConfigMapNamespace,
				},
				Spec: awsauthv1alpha1.AWSAuthItemSpec{
					MapUsers: []awsauthv1alpha1.MapUserItem{expectedUser},
				},
			}
			Expect(k8sClient.Create(ctx, item)).To(Succeed())
			DeferCleanup(cleanupAWSAuthItem, item)

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
