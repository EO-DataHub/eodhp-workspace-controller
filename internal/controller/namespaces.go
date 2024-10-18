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
