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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	config      Config
	aws         aws.AWSClient
	reconcilers []Reconciler
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
			&FinalizerReconciler{
				Client:    client,
				Finalizer: "core.telespazio-uk.io/workspace-finalizer",
			},
			&NamespaceReconciler{Client: client},
			&aws.IAMRoleReconciler{
				Client:     client,
				AWS:        awsClient,
				RoleSuffix: fmt.Sprintf("-%s", config.ClusterName),
			},
		},
	}
}

func reverse[T any](s []T) []T {
	t := make([]T, len(s))
	for i := 0; i < len(s)/2; i++ {
		j := len(s) - i - 1
		t[i], t[j] = s[j], s[i]
	}
	return t
}

type Reconciler interface {
	Reconcile(ctx context.Context, ws *corev1alpha1.Workspace) error
	Teardown(ctx context.Context, ws *corev1alpha1.Workspace) error
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
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	workspace := &corev1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if workspace.ObjectMeta.DeletionTimestamp.IsZero() {
		for _, reconciler := range r.reconcilers {
			if err := reconciler.Reconcile(ctx, workspace); err != nil {
				log.Error(err, "Reconciler failed", "reconciler", reconciler)
			}
		}
	} else {
		for _, reconciler := range reverse(r.reconcilers) {
			if err := reconciler.Teardown(ctx, workspace); err != nil {
				log.Error(err, "Teardown failed", "reconciler", reconciler)
			}

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
