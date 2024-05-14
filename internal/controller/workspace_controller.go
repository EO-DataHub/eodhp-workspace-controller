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
	"reflect"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/UKEODHP/workspace-controller/internal/aws"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	config      Config
	aws         aws.AWSClient
	reconcilers []Reconciler
	finalizer   string
}

func NewWorkspaceReconciler(client client.Client, scheme *runtime.Scheme,
	config Config) *WorkspaceReconciler {

	awsClient := aws.AWSClient{}
	if config.AWS.Region != "" {
		log.Log.Info("AWS region set. AWS support enabled.", "region", config.AWS.Region)
		if err := awsClient.Initialise(config.AWS); err != nil {
			log.Log.Error(err, "Problem initialising AWS client")
		}
	} else {
		log.Log.Info("No AWS region set. AWS support disabled.")
	}

	return &WorkspaceReconciler{
		Client: client,
		Scheme: scheme,
		config: config,
		aws:    awsClient,
		reconcilers: []Reconciler{
			&NamespaceReconciler{Client: client},
			&ServiceAccountReconciler{Client: client},
			&aws.IAMRoleReconciler{Client: client, AWS: awsClient},
			&aws.EFSReconciler{Client: client, AWS: awsClient},
			&StorageReconciler{Client: client},
			&aws.S3Reconciler{Client: client, AWS: awsClient},
		},
		finalizer: "core.telespazio-uk.io/workspace-finalizer",
	}
}

func reverse[T any](s []T) []T {
	t := make([]T, len(s))
	copy(t, s)
	for i := 0; i < len(s)/2; i++ {
		j := len(s) - i - 1
		t[i], t[j] = s[j], s[i]
	}
	return t
}

type Reconciler interface {
	Reconcile(
		ctx context.Context,
		spec *corev1alpha1.WorkspaceSpec,
		status *corev1alpha1.WorkspaceStatus,
	) error
	Teardown(
		ctx context.Context,
		spec *corev1alpha1.WorkspaceSpec,
		status *corev1alpha1.WorkspaceStatus,
	) error
}

//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.telespazio-uk.io,resources=workspaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workspace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *WorkspaceReconciler) Reconcile(ctx context.Context,
	req ctrl.Request) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	ws := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	sts := ws.Status.DeepCopy()

	if ws.ObjectMeta.DeletionTimestamp.IsZero() {
		// Workspace is not being deleted, continue with reconciliation
		if updated, err := r.ReconcileFinalizer(ctx, req); updated {
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			return ctrl.Result{}, err
		}

		for _, reconciler := range r.reconcilers {
			// Reconcile
			if err := reconciler.Reconcile(ctx, &ws.Spec, sts); err != nil {
				log.Error(err, "Reconciler failed", "reconciler", reconciler)
				return ctrl.Result{}, err
			}
		}

		if _, err := r.UpdateStatus(ctx, req, sts); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Workspace is being deleted, teardown dependents
		for _, reconciler := range reverse(r.reconcilers) {
			// Teardown
			if err := reconciler.Teardown(ctx, &ws.Spec, sts); err != nil {
				log.Error(err, "Teardown failed", "reconciler", reconciler)
				return ctrl.Result{}, err
			}
		}

		if _, err := r.UpdateStatus(ctx, req, sts); err != nil {
			return ctrl.Result{}, err
		}

		if _, err := r.TeardownFinalizer(ctx, req); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) ReconcileFinalizer(ctx context.Context,
	req ctrl.Request) (bool, error) {

	log := log.FromContext(ctx)
	ws := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if updated := controllerutil.AddFinalizer(ws, r.finalizer); updated {
		if err := r.Update(ctx, ws); err != nil {
			return false, err
		}
		log.Info("Added finalizer", "workspace", ws)
		return true, nil
	} else {
		return false, nil
	}

}

func (r *WorkspaceReconciler) TeardownFinalizer(ctx context.Context,
	req ctrl.Request) (bool, error) {

	log := log.FromContext(ctx)
	ws := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if updated := controllerutil.RemoveFinalizer(ws, r.finalizer); updated {
		if err := r.Update(ctx, ws); err != nil {
			return false, err
		}
		log.Info("Removed finalizer", "workspace", ws)
		return true, nil
	} else {
		return false, nil
	}

}

func (r *WorkspaceReconciler) UpdateStatus(ctx context.Context, req ctrl.Request,
	sts *corev1alpha1.WorkspaceStatus) (bool, error) {

	ws := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if !reflect.DeepEqual(ws.Status, *sts) {
		ws.Status = *sts
		if err := r.Status().Update(ctx, ws); err == nil {
			return true, nil
		} else {
			return false, err
		}
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Workspace{}).
		Complete(r)
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
		log.Log.Error(err, "Problem unmarshalling config file")
		return err
	}

	return nil
}
