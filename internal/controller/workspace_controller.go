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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workspace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Get a reference to the workspace that has been updated
	workspace := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle finalizer
	if continue_, err := r.ReconcileFinalizer(ctx, workspace); err != nil {
		return ctrl.Result{}, err
	} else if !continue_ {
		// Workspace is being deleted, do not continue with reconciliation
		return ctrl.Result{}, nil
	}

	// Reconcile namespace
	if err := r.ReconcileNamespace(ctx, workspace); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Workspace{}).
		Complete(r)
}

// Reconcile finalizer
func (r *WorkspaceReconciler) ReconcileFinalizer(ctx context.Context, workspace *corev1alpha1.Workspace) (bool, error) {
	log := log.FromContext(ctx)

	// Create finalizer
	finalizer := "core.telespazio-uk.io/workspace-finalizer"
	// Is object being deleted?
	if workspace.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so we add our finalizer if it is not already present
		if !controllerutil.ContainsFinalizer(workspace, finalizer) {
			controllerutil.AddFinalizer(workspace, finalizer)
			if err := r.Update(ctx, workspace); err != nil {
				log.Info("Added finalizer to workspace", "workspace.Status", workspace.Status)
				// Continue reconciliation
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(workspace, finalizer) {
			// Our finalizer is present, so lets handle any external dependency
			if err := r.deleteChildResources(ctx, workspace); err != nil {
				// If we fail to delete the external resource, we need to requeue the item
				return false, err
			}
			// Remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(workspace, finalizer)
			if err := r.Update(ctx, workspace); err != nil {
				return false, err
			}
		}
		// Stop reconciliation as the item is being deleted.
		return false, nil
	}
	return true, nil
}

// Delete any child resources of the object.
func (r *WorkspaceReconciler) deleteChildResources(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {

	log := log.FromContext(ctx)

	// Delete namespace
	namespace := &corev1.Namespace{}
	if err := client.IgnoreNotFound(
		r.Get(ctx, client.ObjectKey{Name: workspace.Status.Namespace}, namespace)); err == nil {

		if err := r.Delete(ctx, namespace); err != nil {
			log.Error(err, "Failed to delete namespace", "namespace", workspace.Status.Namespace)
			return err
		} else {
			log.Info("Namespace deleted", "namespace", workspace.Status.Namespace)
		}
	}

	return nil
}

// Create the workspace namespace if it does not already exist and update the
// workspace status with the namespace name.
func (r *WorkspaceReconciler) ReconcileNamespace(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	// Create workspace namespace if it does not already exist.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: workspace.Name}, namespace); err != nil {
		if errors.IsNotFound(err) {
			// Namespace does not exist
			log.Info("Namespace for workspace does not exist", "namespace", workspace.Name)

			// Create namespace
			namespace.Name = workspace.Name
			err = client.IgnoreAlreadyExists(r.Create(ctx, namespace))
			if err == nil {
				log.Info("Namespace created", "name", workspace.Name)
			} else {
				return err
			}
		} else {
			log.Error(err, "Failed to get namespace", "namespace", workspace.Name)
			return err
		}
	}

	// Update workspace status with namespace name
	workspace.Status.Namespace = namespace.Name
	if err := r.Status().Update(ctx, workspace); err != nil {
		log.Error(err, "Failed to update workspace status",
			"workspace.Status", workspace.Status)
	}

	return nil
}
