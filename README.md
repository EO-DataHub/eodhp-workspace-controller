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

This module is designed for reuse and extensibility. To extend the workspace controller for your own project do the following:

1. Scaffold your own Kubebuilder project `kubebuilder init --domain my.domain --owner "My Org"`
2. Modify the `cmd/main.go` file as follows:

```golang
package main

import (
  ...

  // add these imports
  corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/UKEODHP/workspace-controller/controller"
)

...

func init() {
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
  if err = (&controller.WorkspaceReconciler{
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

You can now extend the controller functionality as you require. You may wish to wrap the `controller.Reconcile` method to add functionality to the workspace reconcile loop.
