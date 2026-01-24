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
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	awsauthv1alpha1 "github.com/maruina/aws-auth-manager/api/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient    client.Client
	testEnv      *envtest.Environment
	ctx          context.Context
	cancel       context.CancelFunc
	reconciler   *AWSAuthItemReconciler
	fakeRecorder *record.FakeRecorder
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(suiteCtx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	_, ok := os.LookupEnv("AWS_AUTH_MANAGER_USE_EXISTING_CLUSTER")

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &ok,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = awsauthv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(k8sManager).NotTo(BeNil())
	Expect(err).NotTo(HaveOccurred())

	// Create a fake recorder with buffer of 100 events for testing
	fakeRecorder = record.NewFakeRecorder(100)

	reconciler = &AWSAuthItemReconciler{
		Client:                    k8sManager.GetClient(),
		Scheme:                    k8sManager.GetScheme(),
		Recorder:                  fakeRecorder,
		AWSAuthConfigMapName:      "aws-auth-dryrun",
		AWSAuthConfigMapNamespace: "kube-system",
	}
	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
}, NodeTimeout(60*time.Second))

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// Test timeout constants
const (
	eventuallyTimeout    = 30 * time.Second
	eventuallyInterval   = 100 * time.Millisecond
	consistentlyDuration = 2 * time.Second
	consistentlyInterval = 500 * time.Millisecond
)

// uniqueName generates a unique name for test resources to avoid conflicts
// between tests.
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, rand.String(5))
}

// getAWSAuthConfigMap retrieves the aws-auth ConfigMap managed by the controller.
func getAWSAuthConfigMap() (*corev1.ConfigMap, error) {
	var cm corev1.ConfigMap
	key := types.NamespacedName{
		Name:      reconciler.AWSAuthConfigMapName,
		Namespace: reconciler.AWSAuthConfigMapNamespace,
	}
	if err := k8sClient.Get(ctx, key, &cm); err != nil {
		return nil, err
	}
	return &cm, nil
}

// getMapRolesFromConfigMap parses the mapRoles data from a ConfigMap.
func getMapRolesFromConfigMap(cm *corev1.ConfigMap) ([]awsauthv1alpha1.MapRoleItem, error) {
	data, ok := cm.Data["mapRoles"]
	if !ok {
		return nil, nil
	}
	if data == "" {
		return nil, nil
	}
	var roles []awsauthv1alpha1.MapRoleItem
	if err := yaml.Unmarshal([]byte(data), &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// getMapUsersFromConfigMap parses the mapUsers data from a ConfigMap.
func getMapUsersFromConfigMap(cm *corev1.ConfigMap) ([]awsauthv1alpha1.MapUserItem, error) {
	data, ok := cm.Data["mapUsers"]
	if !ok {
		return nil, nil
	}
	if data == "" {
		return nil, nil
	}
	var users []awsauthv1alpha1.MapUserItem
	if err := yaml.Unmarshal([]byte(data), &users); err != nil {
		return nil, err
	}
	return users, nil
}

// drainEvents removes all events from the fake recorder's channel.
// Call this before a test that needs to verify events to ensure a clean slate.
func drainEvents() {
	for {
		select {
		case <-fakeRecorder.Events:
		default:
			return
		}
	}
}
