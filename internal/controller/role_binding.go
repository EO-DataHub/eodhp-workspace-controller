package controller

import (
	"context"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RoleBindingReconciler struct {
	Client
}

func (r *RoleBindingReconciler) Reconcile(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Fetch the RoleBinding resource
	roleBinding := &rbacv1.RoleBinding{}
	roleBindingName := spec.ServiceAccount.Name + "-config-reader-binding"
	if err := r.Get(ctx, client.ObjectKey{
		Name:      roleBindingName,
		Namespace: spec.Namespace},
		roleBinding,
	); err == nil {
		// RoleBinding already exists
		return nil
	} else {
		if errors.IsNotFound(err) {
			log.Info("RoleBinding does not exist", "name",
				roleBindingName, "namespace", spec.Namespace)
			// continue
		} else {
			log.Error(err, "Failed to get RoleBinding", "name",
				roleBindingName, "namespace", spec.Namespace)
			return err
		}
	}

	// Create the RoleBinding object
	roleBinding.Name = roleBindingName
	roleBinding.Namespace = spec.Namespace
	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     spec.ServiceAccount.Name + "-config-reader",
	}
	roleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      spec.ServiceAccount.Name,
			Namespace: spec.Namespace,
		},
	}
	if err := r.Create(ctx, roleBinding); err != nil {
		log.Error(err, "Failed to create RoleBinding", "name",
			roleBindingName, "namespace", spec.Namespace)
		return err
	}

	log.Info("RoleBinding created", "name", roleBindingName, "namespace", spec.Namespace)

	return nil
}

func (r *RoleBindingReconciler) Teardown(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	roleBinding := &rbacv1.RoleBinding{}
	roleBindingName := spec.ServiceAccount.Name + "-config-reader-binding"
	if err := r.Get(ctx, client.ObjectKey{
		Name:      roleBindingName,
		Namespace: spec.Namespace},
		roleBinding,
	); err == nil {
		if err := r.DeleteResource(ctx, roleBinding); err != nil {
			return err
		}
	}

	return nil
}
