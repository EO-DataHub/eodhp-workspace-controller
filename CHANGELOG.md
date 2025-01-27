# Workspace Controller

## v0.4.6 (27-01-2025)

- Added `state` and `errorDescription` to the Workspace status CRD and embedded in the reconciler
- Coupled with new helm/workspace-operator - 0.4.2-rc4

## v0.4.5 (23-01-2025)

- Bugfix EFS Reconciler to find all existing AccessPointIDs

## v0.4.0 (03-07-2024)

- Added workspace update/delete events

## v0.3.0 (15-05-2024)

- Added S3 access point support
- Refactored main loop to cycle through reconcilers

## v0.2.1 (07-05-2024)

- Added AWS client package
  - Added workspace IAM policy reconciler
  - Added workspace EFS storage reconciler
- Added workspace service account reconciler
- Added workspace storage reconciler

## v0.1.0 (17-04-2024)

- Scaffolded Kubebuilder project
- Added workspace namespace reconciler
