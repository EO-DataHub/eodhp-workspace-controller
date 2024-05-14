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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type FinalizerReconciler struct {
	client.Client
	Finalizer string
}

func (r *FinalizerReconciler) Reconcile(ctx context.Context,
	workspace *corev1alpha1.Workspace) error {

	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(workspace, r.Finalizer) {
		controllerutil.AddFinalizer(workspace, r.Finalizer)
		if err := r.Update(ctx, workspace); err != nil {
			log.Info("Added finalizer to workspace", "workspace.Status", workspace.Status)
		}
	}

	return nil
}

func (r *FinalizerReconciler) Teardown(ctx context.Context,
	workspace *corev1alpha1.Workspace) error {

	controllerutil.RemoveFinalizer(workspace, r.Finalizer)
	if err := r.Update(ctx, workspace); err != nil {
		return err
	}

	return nil
}
