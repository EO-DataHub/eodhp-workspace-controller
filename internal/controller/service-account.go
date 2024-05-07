package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *WorkspaceReconciler) ReconcileServiceAccount(ctx context.Context,
	name, namespace string, annotations map[string]string) error {

	log := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{
		Name: name, Namespace: namespace}, serviceAccount); err == nil {
		// ServiceAccount already exists
		return nil
	} else {
		if errors.IsNotFound(err) {
			log.Info("ServiceAccount does not exist", "name",
				name, "namespace", namespace)
			// continue
		} else {
			log.Error(err, "Failed to get ServiceAccount", "name", name,
				"namespace", namespace)
			return err
		}
	}

	// Create the ServiceAccount object
	serviceAccount.Name = name
	serviceAccount.Namespace = namespace
	serviceAccount.Annotations = annotations
	if err := r.Create(ctx, serviceAccount); err != nil {
		log.Error(err, "Failed to create ServiceAccount", "name", name,
			"namespace", namespace)
		return err
	}

	log.Info("ServiceAccount created", "name", name, "namespace", namespace)

	return nil
}

func (r *WorkspaceReconciler) DeleteServiceAccount(ctx context.Context,
	name, namespace string) error {

	log := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{
		Name: name, Namespace: namespace}, serviceAccount); err != nil {
		if errors.IsNotFound(err) {
			// ServiceAccount does not exist
			return nil
		} else {
			log.Error(err, "Failed to delete ServiceAccount", "name", name,
				"namespace", namespace)
			return err
		}
	}

	if err := r.Delete(ctx, serviceAccount); err == nil {
		log.Info("ServiceAccount deleted", "name", name, "namespace", namespace)
	} else {
		log.Error(err, "Failed to delete ServiceAccount", "name", name,
			"namespace", namespace)
		return err
	}
	return nil
}
