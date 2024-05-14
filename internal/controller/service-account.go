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
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccountReconciler struct {
	client.Client
}

func (r *ServiceAccountReconciler) Reconcile(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      spec.ServiceAccount.Name,
		Namespace: spec.Namespace},
		serviceAccount,
	); err == nil {
		// ServiceAccount already exists
		return nil
	} else {
		if errors.IsNotFound(err) {
			log.Info("ServiceAccount does not exist", "name",
				spec.ServiceAccount.Name, "namespace", spec.Namespace)
			// continue
		} else {
			log.Error(err, "Failed to get ServiceAccount", "name",
				spec.ServiceAccount.Name, "namespace", spec.Namespace)
			return err
		}
	}

	// Create the ServiceAccount object
	serviceAccount.Name = spec.ServiceAccount.Name
	serviceAccount.Namespace = spec.Namespace
	serviceAccount.Annotations = spec.ServiceAccount.Annotations
	if err := r.Create(ctx, serviceAccount); err != nil {
		log.Error(err, "Failed to create ServiceAccount", "name",
			spec.ServiceAccount.Name, "namespace", spec.Namespace)
		return err
	}

	log.Info("ServiceAccount created", "name", spec.ServiceAccount.Name,
		"namespace", spec.Namespace)

	return nil
}

func (r *ServiceAccountReconciler) Teardown(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      spec.ServiceAccount.Name,
		Namespace: spec.Namespace},
		serviceAccount,
	); err != nil {
		if errors.IsNotFound(err) {
			// ServiceAccount does not exist
			return nil
		} else {
			log.Error(err, "Failed to delete ServiceAccount",
				"name", spec.ServiceAccount.Name, "namespace", spec.Namespace)
			return err
		}
	}

	if err := r.Delete(ctx, serviceAccount); err == nil {
		log.Info("ServiceAccount deleted",
			"name", spec.ServiceAccount.Name, "namespace", spec.Namespace)
	} else {
		log.Error(err, "Failed to delete ServiceAccount",
			"name", spec.ServiceAccount.Name, "namespace", spec.Namespace)
		return err
	}
	return nil
}
