package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
)

var _ = Describe("Workspace Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const finalizer = "core.telespazio-uk.io/workspace-finalizer"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		workspace := &corev1alpha1.Workspace{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Workspace")
			err := k8sClient.Get(ctx, typeNamespacedName, workspace)
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1alpha1.Workspace{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &corev1alpha1.Workspace{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Removing the finalizer so the resource can be deleted")
			controllerutil.RemoveFinalizer(resource, finalizer)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())

			By("Cleanup the specific resource instance Workspace")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should add the finalizer on first reconcile", func() {
			controllerReconciler := NewWorkspaceReconciler(Client{Client: k8sClient},
				k8sClient.Scheme(), Config{}, nil)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			resource := &corev1alpha1.Workspace{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(resource, finalizer)).To(BeTrue())
		})
	})
})
