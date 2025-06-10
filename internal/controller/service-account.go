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
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccountReconciler struct {
	Client
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
		if status.AWS.Role.ARN != "" &&
			serviceAccount.Annotations["eks.amazonaws.com/role-arn"] != status.AWS.Role.ARN {
			if serviceAccount.Annotations == nil {
				serviceAccount.Annotations = make(map[string]string)
			}
			serviceAccount.Annotations["eks.amazonaws.com/role-arn"] = status.AWS.Role.ARN
			if err := r.Update(ctx, serviceAccount); err != nil {
				log.Error(err, "Failed to update ServiceAccount",
					"service account", serviceAccount)
			}
		}
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
	if spec.ServiceAccount.Annotations == nil {
		serviceAccount.Annotations = make(map[string]string)
	} else {
		serviceAccount.Annotations = spec.ServiceAccount.Annotations
	}
	if status.AWS.Role.Name != "" {
		serviceAccount.Annotations["eks.amazonaws.com/role-arn"] = status.AWS.Role.Name
	}
	if err := r.Create(ctx, serviceAccount); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("ServiceAccount already exists", "name",
				spec.ServiceAccount.Name, "namespace", spec.Namespace)
		} else {
			log.Error(err, "Failed to create ServiceAccount", "name",
				spec.ServiceAccount.Name, "namespace", spec.Namespace)
			return err
		}
	}

	log.Info("ServiceAccount created/updated", "name", spec.ServiceAccount.Name,
		"namespace", spec.Namespace)

	return nil
}

func (r *ServiceAccountReconciler) Teardown(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      spec.ServiceAccount.Name,
		Namespace: spec.Namespace},
		serviceAccount,
	); err == nil {
		if err := r.DeleteResource(ctx, serviceAccount); err != nil {
			return err
		}
	} else {
		if !errors.IsNotFound(err) {
			return err
		}
		// ServiceAccount already deleted
	}

	return nil
}
