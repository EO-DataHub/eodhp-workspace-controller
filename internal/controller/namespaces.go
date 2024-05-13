/*
Copyright 2024 Telespazio UK.

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

package controller

import (
	"context"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
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

type NamespaceReconciler struct {
	client.Client
}

func (r *NamespaceReconciler) Reconcile(ws *corev1alpha1.Workspace) error {
	ctx := context.Background()
	log := log.FromContext(ctx)

	// Create workspace namespace if it does not already exist.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: ws.Spec.Namespace}, namespace); err != nil {
		if errors.IsNotFound(err) {
			// Namespace does not exist
			log.Info("Namespace for workspace does not exist", "namespace", ws.Spec.Namespace)

			// Create namespace
			namespace.Name = ws.Spec.Namespace
			err = client.IgnoreAlreadyExists(r.Create(ctx, namespace))
			if err == nil {
				log.Info("Namespace created", "Namespace", namespace)
			} else {
				return err
			}
		} else {
			log.Error(err, "Failed to get namespace", "namespace", ws.Spec.Namespace)
			return err
		}
	}

	return nil
}

func (r *NamespaceReconciler) Teardown(ws *corev1alpha1.Workspace) error {
	ctx := context.Background()
	log := log.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := client.IgnoreNotFound(
		r.Get(ctx, client.ObjectKey{Name: ws.Spec.Namespace}, namespace)); err == nil {

		if err := r.Delete(ctx, namespace); err == nil {
			log.Info("Namespace deleted", "namespace", namespace)
		} else {
			log.Error(err, "Failed to delete namespace", "namespace", ws.Spec.Namespace)
			return err
		}
	}

	return nil
}
