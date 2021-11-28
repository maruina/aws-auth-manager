package controllers

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
)

var _ = Describe("AWSAuth controller", func() {

	SetDefaultEventuallyTimeout(time.Second * 10)
	authCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AWSAuthMapName,
			Namespace: AWSAuthMapNamespace,
		},
	}

	It("Should manage the aws-auth ConfigMap", func() {
		By("Creating three AWSAuthItem objects")
		userItem := awsauthv1alpha1.AWSAuthItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-item",
				Namespace: AWSAuthMapNamespace,
			},
			Spec: awsauthv1alpha1.AWSAuthItemSpec{
				MapUsers: []awsauthv1alpha1.MapUserItem{
					{
						UserArn:  "arn:aws:iam::111122223333:user/admin",
						Username: "admin",
						Groups:   []string{"system:masters"},
					},
					{
						UserArn:  "arn:aws:iam::111122223333:user/ops-user",
						Username: "ops-user",
						Groups:   []string{"system:masters"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &userItem)).To(Succeed())

		roleItem := awsauthv1alpha1.AWSAuthItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "role-item",
				Namespace: AWSAuthMapNamespace,
			},
			Spec: awsauthv1alpha1.AWSAuthItemSpec{
				MapRoles: []awsauthv1alpha1.MapRoleItem{
					{
						RoleArn:  "arn:aws:iam::111122223333:role/eksctl-my-cluster-nodegroup-standard-wo-NodeInstanceRole-1WP3NUE3O6UCF",
						Username: "system:node:{{EC2PrivateDNSName}}",
						Groups:   []string{"system:bootstrappers", "system:nodes"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &roleItem)).To(Succeed())

		mixedItem := awsauthv1alpha1.AWSAuthItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mixed-item",
				Namespace: AWSAuthMapNamespace,
			},
			Spec: awsauthv1alpha1.AWSAuthItemSpec{
				MapRoles: []awsauthv1alpha1.MapRoleItem{
					{
						RoleArn:  "arn:aws:iam::111122223333:role/eks-developer-role",
						Username: "eks-developer",
						Groups:   []string{"tenant"},
					},
				},
				MapUsers: []awsauthv1alpha1.MapUserItem{
					{
						UserArn:  "arn:aws:iam::111122223333:role/eks-user",
						Username: "eks-user-{{Session}}",
						Groups:   []string{"edit", "view"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &mixedItem)).To(Succeed())

		// Check if the aws-auth configmap has the desired data
		expectedMapUsers := append(userItem.Spec.MapUsers, mixedItem.Spec.MapUsers...)
		Eventually(func() []awsauthv1alpha1.MapUserItem {
			var res awsauthv1alpha1.AWSAuthItem

			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm)
			_ = yaml.Unmarshal([]byte(authCm.Data["MapUsers"]), &res.Spec.MapUsers)

			return res.Spec.MapUsers
		}).Should(Equal(expectedMapUsers))

		expectedMapRoles := append(roleItem.Spec.MapRoles, mixedItem.Spec.MapRoles...)
		Eventually(func() []awsauthv1alpha1.MapRoleItem {
			var res awsauthv1alpha1.AWSAuthItem

			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm)
			_ = yaml.Unmarshal([]byte(authCm.Data["MapRoles"]), &res.Spec.MapRoles)

			return res.Spec.MapRoles
		}).Should(Equal(expectedMapRoles))

		// Check if the CRDs has the condition Ready
		Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&userItem), &userItem)

			return apimeta.IsStatusConditionTrue(*userItem.GetStatusConditions(), meta.ReadyCondition)
		}).Should(BeTrue())

		Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&roleItem), &roleItem)

			return apimeta.IsStatusConditionTrue(*roleItem.GetStatusConditions(), meta.ReadyCondition)
		}).Should(BeTrue())

		Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&mixedItem), &mixedItem)

			return apimeta.IsStatusConditionTrue(*mixedItem.GetStatusConditions(), meta.ReadyCondition)
		}).Should(BeTrue())

		By("Deleting a AWSAuthItem")
		Expect(k8sClient.Delete(ctx, &mixedItem)).To(Succeed())

		Eventually(func() []awsauthv1alpha1.MapUserItem {
			var res awsauthv1alpha1.AWSAuthItem

			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm)
			_ = yaml.Unmarshal([]byte(authCm.Data["MapUsers"]), &res.Spec.MapUsers)

			return res.Spec.MapUsers
		}).Should(Equal(userItem.Spec.MapUsers))

		Eventually(func() []awsauthv1alpha1.MapRoleItem {
			var res awsauthv1alpha1.AWSAuthItem

			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&authCm), &authCm)
			_ = yaml.Unmarshal([]byte(authCm.Data["MapRoles"]), &res.Spec.MapRoles)

			return res.Spec.MapRoles
		}).Should(Equal(roleItem.Spec.MapRoles))
	})
})
