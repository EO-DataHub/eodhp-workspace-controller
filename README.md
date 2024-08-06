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
make manifests  # generate the latest manifests
make install  # installs CRDs to the cluster
make docker-build docker-push IMG=public.ecr.aws/n1b3o1k2/workspace-controller:<tag> # build and push 
make deploy IMG=public.ecr.aws/n1b3o1k2/workspace-controller:<tag>  # deploy controller to cluster
```

## Uninstall

```bash
make uninstall  # removes CRDs from cluster
make undeploy  # remove controller from the cluster
```

## Install CRDs

```bash
make install # installs CRDs to the cluster
```

## Development

### Updating API

After updating any api/**/*_types.go files run:

```bash
make manifests  # generate the manifests
make  # regenerate the code
make install  # install the CRDs to the cluster
```

### Generate Helm Chart

To update the Helm chart:

```bash
make helm CHART=chart/workspace-operator
```

__Be careful not to overwrite manual changes to the Helm manifests. Always commit to Git just before applying `make helm` and compare changes, reverting the change where the modification undoes manual changes.__

To publish the Helm chart:

```bash
helm package chart/workspace-operator
aws ecr-public get-login-password --region us-east-1 | helm registry login --username AWS --password-stdin public.ecr.aws
helm push workspace-operator-x.y.z.tgz oci://public.ecr.aws/n1b3o1k2/helm
```

## Manually Export Manifests

```bash
kustomize build config/crd > crds.yaml  # crds
kustomize build config/default > manifests.yaml  # all other manifests
```

## Configuration

A file path with following parameters is required to be passed to the operator with `--config <path>` flag.

```yaml
aws:
  accountID: 123456789
  region: eu-west-2
  oidc:
    provider: oidc.eks.my-region.amazonaws.com/id/A1B2C3D4E5F6G7H8
```
