package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Create the workspace namespace if it does not already exist and update the
// workspace status with the namespace name.
func (r *WorkspaceReconciler) ReconcileNamespace(
	ctx context.Context, name string) (*corev1.Namespace, error) {
	log := log.FromContext(ctx)

	// Create workspace namespace if it does not already exist.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: name}, namespace); err != nil {
		if errors.IsNotFound(err) {
			// Namespace does not exist
			log.Info("Namespace for workspace does not exist", "namespace", name)

			// Create namespace
			namespace.Name = name
			err = client.IgnoreAlreadyExists(r.Create(ctx, namespace))
			if err == nil {
				log.Info("Namespace created", "name", name)
			} else {
				return nil, err
			}
		} else {
			log.Error(err, "Failed to get namespace", "namespace", name)
			return nil, err
		}
	}

	return namespace, nil
}

func (r *WorkspaceReconciler) DeleteNamespace(
	ctx context.Context, name string) error {
	log := log.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := client.IgnoreNotFound(
		r.Get(ctx, client.ObjectKey{Name: name}, namespace)); err == nil {

		if err := r.Delete(ctx, namespace); err != nil {
			log.Error(err, "Failed to delete namespace", "namespace", name)
			return err
		} else {
			log.Info("Namespace deleted", "namespace", name)
		}
	}
	return nil
}
