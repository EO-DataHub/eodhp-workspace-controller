package controller

import (
	"context"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NamespaceReconciler struct {
	Client
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Create namespace if it does not already exist.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: spec.Namespace},
		namespace); err != nil {
		if errors.IsNotFound(err) {
			// Create namespace
			namespace.Name = spec.Namespace
			err = client.IgnoreAlreadyExists(r.Create(ctx, namespace))
			if err == nil {
				log.Info("Namespace created", "namespace", namespace.Name)
			} else {
				return err
			}
		} else {
			log.Error(err, "Failed to get namespace",
				"namespace", spec.Namespace)
			return err
		}
	}

	status.Namespace = namespace.Name
	return nil
}

func (r *NamespaceReconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: spec.Namespace},
		namespace); err == nil {
		if !namespace.ObjectMeta.DeletionTimestamp.IsZero() {
			// Already being deleted
			return nil
		}
		// Delete namespace
		if err := r.DeleteResource(ctx, namespace); err != nil {
			return err
		}
	} else {
		if !errors.IsNotFound(err) {
			return err
		}
		// Already deleted
	}

	status.Namespace = ""
	return nil
}
