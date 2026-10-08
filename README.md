# Workspace Operator

This service provides reconcilers for a number of workspace configurations to ensure they remain aligned. This includes reconcilers for:
- namespace
- storage (EFS and S3)
- service account
- AWS IAM Policies

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
make docker-build docker-push IMG=public.ecr.aws/eodh/workspace-controller:<tag> # build and push 
make deploy IMG=public.ecr.aws/eodh/workspace-controller:<tag>  # deploy controller to cluster
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
helm push workspace-operator-x.y.z.tgz oci://public.ecr.aws/eodh/helm
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

### Pulsar events

If `pulsar.url` is set, the controller publishes workspace `update` and `delete` events to Pulsar.

```yaml
pulsar:
  url: pulsar://pulsar-proxy.pulsar:6650
  # Optional. File containing a JWT for Pulsar token authentication.
  # If not set, the controller connects without authentication.
  tokenFile: /var/run/secrets/pulsar/token
  # Optional. Defaults to workspace-controller.
  topic: persistent://public/workspaces/workspace-controller
```

The token file is read again each time the client connects or re-authenticates, so a rotated token is picked up without a restart. If `tokenFile` is set but the file cannot be read, the controller exits at startup.

### Helm chart values

The Helm chart renders `controllerManager.config` as the config file above. Extra environment variables, volume mounts and volumes can be added to the manager with `controllerManager.manager.extraEnv`, `controllerManager.manager.extraVolumeMounts` and `controllerManager.extraVolumes`. For example, to mount a Pulsar token from a secret:

```yaml
controllerManager:
  manager:
    extraVolumeMounts:
      - name: pulsar-token
        mountPath: /var/run/secrets/pulsar
        readOnly: true
  extraVolumes:
    - name: pulsar-token
      secret:
        secretName: workspace-controller-pulsar-token
        items:
          - key: TOKEN
            path: token
  config:
    pulsar:
      url: pulsar://pulsar-proxy.pulsar:6650
      tokenFile: /var/run/secrets/pulsar/token
      topic: persistent://public/workspaces/workspace-controller
```

Mount the secret as a directory rather than with `subPath`, otherwise the file is not updated when the secret changes.
