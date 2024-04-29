# Workspace Operator

See [Kubebuilder docs](https://book.kubebuilder.io/quick-start.html) for full instructions.

## Prerequisites

- Kustomize version matching the KUSTOMIZE_VERSION (specified in Makefile)

## Run Locally

You can run the controller locally for debugging purposes. Make sure your kubectl is pointed at the correct cluster.

```bash
make install  # installs CRDs to the cluster
make run  # run a local instance of the controller
```

## Run in Cluster

```bash
make install  # installs CRDs to the cluster
make docker-build docker-push IMG=<some-registry>/<project-name>:tag  # build controller
make deploy IMG=<some-registry>/<project-name>:tag  # deploy controller to cluster
```

## Uninstall

```bash
make uninstall  # removes CRDs from cluster
make undeploy  # remove controller from the cluster
```

## Install CRDs

```bash
make manifests  # generate the CRDs
```

## Development

### Updating API

After updating any api/**/*_types.go files run:

```bash
make manifests  # generate the manifests
make  # regenerate the code
make install  # install the CRDs to the cluster
```

## Manually Export Manifests

```bash
kustomize build config/crd > crds.yaml  # crds
kustomize build config/default > manifests.yaml  # all other manifests
```

## Extending the Workspace Controller

This module is designed for reuse and extensibility. This section gives separate details on how to reuse or extend the workspace controller.

### Reuse

This section details how to reuse the workspace controller in your project. This scenario allows you to use the existing functionality of the workspace controller along side any other APIs you wish to generate. It will not allow you to extend the functionality of the workspace controller. You can install the required CRDs to your cluster using the original workspace controller project using `make install`.

1. Scaffold your own Kubebuilder project `kubebuilder init --domain my.domain --owner "My Org"`
2. Modify the `cmd/main.go`:

```golang
// cmd/main.go

package main

import (
  ...

  // add these imports
  corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	corecontroller "github.com/UKEODHP/workspace-controller/controller"
)

...

  // add this scheme
  utilruntime.Must(corev1alpha1.AddToScheme(scheme))
}

func main() {
  ...

  mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    ...
  }
  if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

  // add the controller here
  if err = (&corecontroller.WorkspaceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Workspace")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder
  ...
}
```

### Extend

To extend the functionality, as well as scaffolding a Kubebuilder project, you will also have to generate your own workspace API. You can then import and embed the Workspace, Spec and Status structs and embed them in your own.

You can also call the existing reconciliation methods from your own.

1. Scaffold your own Kubebuilder project `kubebuilder init --domain my.domain --owner "My Org"`
2. Create your own API which will wrap the core functionality `kubebuilder create api --group ai --kind Workspace --version v1alpha1`.
3. Modify the `cmd/main.go`:

```golang
// cmd/main.go

package main

import (
  ...

  // add these imports
  corecontroller "github.com/UKEODHP/workspace-controller/controller"
)

func main() {
  ...

  if err = (&controller.WorkspaceReconciler{
		WorkspaceReconciler: corecontroller.WorkspaceReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		},  // insert embedded reconciler into your own
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Workspace")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder
}
```

3. Modify the `api/v1alpha1/workspace_types.go` file to embed core structs within your own:

```golang
package v1alpha1

import (
	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
)

type WorkspaceSpec struct {
	corev1alpha1.WorkspaceSpec `json:",inline"`
}

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	corev1alpha1.WorkspaceStatus `json:",inline"`
}
```

Now generate the CRDs with `make manifests` and verify they are correct.

4. Call the reconcile core reconcile logic as required from your extension packages reconcile loop.

TODO: this does not work correctly at the moment.