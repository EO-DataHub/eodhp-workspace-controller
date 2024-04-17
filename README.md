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