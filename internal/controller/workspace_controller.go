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
	"fmt"
	"os"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/UKEODHP/workspace-controller/internal/aws"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
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
	aws    aws.AWSClient
}

func NewWorkspaceReconciler(client client.Client, scheme *runtime.Scheme,
	config Config) *WorkspaceReconciler {

	aws := aws.AWSClient{}
	if config.AWS.Region != "" {
		log.Log.Info("AWS region set. AWS support enabled.", "region", config.AWS.Region)
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
//+kubebuilder:rbac:groups=core,resources=persistentvolume,verbs=get;list;watch;create;update;patch;delete

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
	log := log.FromContext(ctx)

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
	if namespace, err := r.ReconcileNamespace(ctx, workspace.Name); err == nil {
		// Update workspace status with namespace name
		workspace.Status.Namespace = namespace.Name
		if err := r.Status().Update(ctx, workspace); err != nil {
			log.Error(err, "Failed to update namespace in workspace status",
				"workspace.Status", workspace.Status)
		}
		// continue with reconciliation
	} else {
		return ctrl.Result{}, err
	}

	// Reconcile AWS resources
	uniqueName := fmt.Sprintf("%s-%s", workspace.Name, r.config.ClusterName)
	if r.aws.Enabled() {
		// Reconcile IAM
		role, err := r.aws.ReconcileIAMRole(ctx, uniqueName, workspace.Status.Namespace)
		if err == nil {
			workspace.Status.AWSRole = *role.RoleName
			if err := r.Status().Update(ctx, workspace); err != nil {
				log.Error(err, "Failed to update role name in workspace status",
					"workspace.Status", workspace.Status, "role", *role.RoleName)
			}
		} else {
			log.Error(err, "Failed to reconcile IAM role", "role", workspace.Name)
		}
		if _, err = r.aws.ReconcileIAMRolePolicy(ctx, uniqueName, role); err != nil {
			log.Error(err, "Failed to reconcile IAM role policy", "policy", workspace.Name)
		}

		// Reconcile EFS Access Points
		if apID, err := r.aws.ReconcileEFSAccessPoint(ctx, r.config.AWS.Storage.EFSID,
			&workspace.Spec.Storage.AWSEFS); err == nil {

			workspace.Status.Storage.AWSEFS.AccessPointID = *apID
			if err := r.Status().Update(ctx, workspace); err != nil {
				log.Error(err, "Failed to update EFS Access Point ID in workspace status",
					"workspace.Status", workspace.Status, "EFS Access Point ID", *apID)
			}
		} else {
			log.Error(err, "Failed to reconcile EFS access point",
				"access point", workspace.Spec.Storage.AWSEFS)
		}
	}

	// Reconcile block storage
	if workspace.Status.Storage.AWSEFS.AccessPointID != "" {
		if err := r.ReconcileBlockStorage(ctx, workspace.Name,
			workspace.Status.Namespace,
			&workspace.Spec.Storage,
			&(corev1.CSIPersistentVolumeSource{
				Driver: "efs.csi.aws.com",
				VolumeHandle: fmt.Sprintf(
					"%s:%s:%s",
					r.config.AWS.Storage.EFSID,
					workspace.Spec.Storage.AWSEFS.RootDirectory,
					workspace.Status.Storage.AWSEFS.AccessPointID,
				),
			})); err != nil {
			return ctrl.Result{}, err
		}
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

	log := log.FromContext(ctx)
	// Delete Kubernetes resources.
	r.DeleteBlockStorage(ctx, workspace.Name, workspace.Status.Namespace)
	r.DeleteNamespace(ctx, workspace.Name)

	// Delete AWS resources
	if r.aws.Enabled() {
		uniqueName := fmt.Sprintf("%s-%s", workspace.Name, r.config.ClusterName)
		if err := r.aws.DeleteIAMRolePolicy(ctx, uniqueName); err != nil {
			log.Error(err, "Failed to delete IAM role policy", "role", uniqueName)
		}
		if err := r.aws.DeleteIAMRole(ctx, uniqueName); err != nil {
			log.Error(err, "Failed to delete IAM role", "role", uniqueName)
		}
	}

	return nil
}

type Config struct {
	AWS         aws.AWSConfig `yaml:"aws"`
	ClusterName string        `yaml:"clusterName"`
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
