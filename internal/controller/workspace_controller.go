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
	"os"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	config Config
	aws    AWSClient
}

func NewWorkspaceReconciler(client client.Client, scheme *runtime.Scheme,
	config Config) *WorkspaceReconciler {

	aws := AWSClient{}
	if config.AWS.Region != "" {
		log.Log.Info("AWS region set. AWS support enabled.", "region", config.AWS.Region)
		config.AWS.UniqueString = config.ClusterName // use cluster name as unique resource string
		if err := aws.Initialise(config.AWS); err != nil {
			log.Log.Error(err, "Problem initialising AWS client")
		}
	} else {
		log.Log.Info("No AWS region set. AWS support disabled.")
	}

	return &WorkspaceReconciler{
		Client: client,
		Scheme: scheme,
		config: config,
		aws:    aws,
	}
}

//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

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

	// Reconcile AWS resources
	r.aws.Reconcile(ctx, workspace)

	// Reconcile block storage
	if err := r.ReconcileBlockStorage(ctx, workspace); err != nil {
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
			if err := r.DeleteChildResources(ctx, workspace); err != nil {
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
func (r *WorkspaceReconciler) DeleteChildResources(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {

	// Delete Kubernetes resources.
	r.DeleteBlockStorage(ctx, workspace)
	r.DeleteNamespace(ctx, workspace)

	// Delete AWS resources
	r.aws.DeleteChildResources(ctx, workspace)

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

func (r *WorkspaceReconciler) DeleteNamespace(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)

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

func (r *WorkspaceReconciler) ReconcileBlockStorage(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      workspace.Name,
		Namespace: workspace.Status.Namespace}, pvc); err == nil {
		return nil // PersistentVolumeClaim already exists
	} else {
		if errors.IsNotFound(err) {
			// PersistentVolumeClaim does not exist
			log.Info("PersistentVolumeClaim for workspace does not exist",
				"pvc", workspace.Name)
		} else {
			log.Error(err, "Failed to get PersistentVolumeClaim",
				"pvc", workspace.Name)
			return err
		}
	}

	// Create block storage
	pvc.Name = workspace.Name
	pvc.Namespace = workspace.Status.Namespace
	pvc.Spec.StorageClassName = &workspace.Spec.Storage.StorageClass
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	var storageSize string
	if workspace.Spec.Storage.Size != "" {
		storageSize = workspace.Spec.Storage.Size
	} else if r.config.Storage.DefaultSize != "" {
		storageSize = r.config.Storage.DefaultSize
	} else {
		storageSize = "2Gi"
	}
	pvc.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(storageSize),
	}
	if err := client.IgnoreAlreadyExists(r.Create(ctx, pvc)); err == nil {
		log.Info("PersistentVolumeClaim created", "name", workspace.Name)
		return nil
	} else {
		return err
	}
}

func (r *WorkspaceReconciler) DeleteBlockStorage(
	ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	// Delete block storage
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      workspace.Name,
		Namespace: workspace.Status.Namespace}, pvc); err != nil {
		if errors.IsNotFound(err) {
			// PersistentVolumeClaim does not exist
			return nil
		} else {
			log.Error(err, "Failed to get PersistentVolumeClaim",
				"pvc", workspace.Name)
			return err
		}
	}
	if err := r.Delete(ctx, pvc); err != nil {
		log.Error(err, "Failed to delete PersistentVolumeClaim",
			"pvc", workspace.Name)
		return err
	} else {
		log.Info("PersistentVolumeClaim deleted", "pvc", workspace.Name)
		return nil
	}
}

type Config struct {
	AWS         AWSConfig `yaml:"aws"`
	ClusterName string    `yaml:"clusterName"`
	Storage     struct {
		StorageClass string `yaml:"storageClass"`
		DefaultSize  string `yaml:"defaultSize"`
	} `yaml:"storage"`
}

func (c *Config) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Log.Error(err, "Problem reading config file")
		return err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		log.Log.Error(err, "Problem unmarshaling config file")
		return err
	}

	return nil
}
