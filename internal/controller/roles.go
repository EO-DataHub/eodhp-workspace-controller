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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RoleReconciler struct {
	Client
}

func (r *RoleReconciler) Reconcile(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Fetch the Role resource
	role := &rbacv1.Role{}
	roleName := spec.ServiceAccount.Name + "-config-reader"
	if err := r.Get(ctx, client.ObjectKey{
		Name:      roleName,
		Namespace: spec.Namespace},
		role,
	); err == nil {
		// Role already exists
		return nil
	} else {
		if errors.IsNotFound(err) {
			log.Info("Role does not exist", "name",
				roleName, "namespace", spec.Namespace)
			// continue
		} else {
			log.Error(err, "Failed to get Role", "name",
				roleName, "namespace", spec.Namespace)
			return err
		}
	}

	// Create the Role object
	role.Name = roleName
	role.Namespace = spec.Namespace
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	if err := r.Create(ctx, role); err != nil {
		log.Error(err, "Failed to create Role", "name",
			roleName, "namespace", spec.Namespace)
		return err
	}

	log.Info("Role created", "name", roleName, "namespace", spec.Namespace)

	return nil
}

func (r *RoleReconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	role := &rbacv1.Role{}
	roleName := spec.ServiceAccount.Name + "-config-reader"
	if err := r.Get(ctx, client.ObjectKey{
		Name:      roleName,
		Namespace: spec.Namespace},
		role,
	); err == nil {
		if err := r.DeleteResource(ctx, role); err != nil {
			return err
		}
	}

	return nil
}
