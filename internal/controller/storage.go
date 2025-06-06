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
	"time"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type StorageReconciler struct {
	Client
}

func (r *StorageReconciler) Reconcile(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	if err := r.ReconcilePersistentVolumes(ctx, spec, status); err != nil {
		log.Error(err, "Failed to reconcile persistent volumes")
	}

	if err := r.ReconcilePersistentVolumeClaims(ctx, spec, status); err != nil {
		log.Error(err, "Failed to reconcile persistent volume claims")
	}

	return nil
}

func (r *StorageReconciler) Teardown(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	if err := r.DeleteBlockStoreData(ctx, spec, status); err != nil {
		log.Error(err, "Failed to delete block store data")
	}

	if err := r.DeletePersistentVolumes(ctx, spec, status); err != nil {
		log.Error(err, "Failed to teardown persistent volumes")
	}

	if err := r.DeletePersistentVolumeClaims(ctx, spec, status); err != nil {
		log.Error(err, "Failed to teardown persistent volume claims")
	}

	return nil
}

func (r *StorageReconciler) ReconcilePersistentVolumes(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	for _, pvSpec := range spec.Storage.PersistentVolumes {
		pv := &corev1.PersistentVolume{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      pvSpec.Name,
			Namespace: spec.Namespace}, pv); err == nil {
			// PersistentVolume already exists.
			if pv.Status.Phase == corev1.VolumeReleased {
				log.Info("PersistentVolume for workspace exists but was released. Reset.",
					"pv", pvSpec, "namespace", spec.Namespace)
				pv.Spec.ClaimRef = nil
				if err := client.IgnoreAlreadyExists(r.Update(ctx, pv)); err != nil {
					log.Error(err, "Failed to reset PersistentVolume",
						"pv", pvSpec, "namespace", spec.Namespace)
					continue
				}
			}
			return nil
		} else {
			if errors.IsNotFound(err) {
				// PersistentVolume does not exist.
			} else {
				log.Error(err, "Failed to get PersistentVolume",
					"pv", pvSpec, "namespace", spec.Namespace)
				continue
			}
		}

		// Create block storage
		pv.Name = pvSpec.Name
		pv.Namespace = spec.Namespace
		pv.Spec.StorageClassName = pvSpec.StorageClass
		pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		pv.Spec.Capacity = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(
				pvSpec.Size,
			),
		}
		volumeMode := corev1.PersistentVolumeFilesystem
		pv.Spec.VolumeMode = &volumeMode
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

		if pvSpec.VolumeSource != nil {
			var ap *corev1alpha1.EFSAccessStatus
			for _, accessPoint := range status.AWS.EFS.AccessPoints {
				if accessPoint.Name == pvSpec.VolumeSource.AccessPointName {
					ap = &accessPoint
					break
				}
			}
			if ap != nil {
				pv.Spec.CSI = &corev1.CSIPersistentVolumeSource{
					Driver: pvSpec.VolumeSource.Driver,
					VolumeHandle: fmt.Sprintf(
						"%s::%s", ap.FSID, ap.AccessPointID,
					),
				}
			}
		}

		if err := client.IgnoreAlreadyExists(r.Create(ctx, pv)); err == nil {
			log.Info("PersistentVolume created",
				"pv", pvSpec, "namespace", spec.Namespace)
		} else {
			log.Info("Failed to create persistent volume",
				"pv", pvSpec, "namespace", spec.Namespace)
		}
	}
	return nil
}

func (r *StorageReconciler) DeletePersistentVolumes(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	for _, pvSpec := range spec.Storage.PersistentVolumes {
		pv := &corev1.PersistentVolume{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      pvSpec.Name,
			Namespace: spec.Namespace}, pv); err != nil {
			if errors.IsNotFound(err) {
				// PersistentVolumeClaim does not exist
				continue
			} else {
				log.Error(err, "Failed to get PersistentVolumeClaim",
					"pv", pvSpec.Name)
				continue
			}
		}
		r.DeleteResource(ctx, pv)
	}
	return nil
}

func (r *StorageReconciler) ReconcilePersistentVolumeClaims(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	for _, pvcSpec := range spec.Storage.PersistentVolumeClaims {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      pvcSpec.Name,
			Namespace: spec.Namespace}, pvc); err == nil {
			continue // PersistentVolumeClaim already exists.
		} else {
			if errors.IsNotFound(err) {
				// PersistentVolumeClaim does not exist.
			} else {
				log.Error(err, "Failed to get PersistentVolumeClaim",
					"pvc", pvcSpec, "namespace", spec.Namespace)
				continue
			}
		}
		// Create persistent volume claim
		pvc.Name = pvcSpec.Name
		pvc.Namespace = spec.Namespace
		pvc.Spec.VolumeName = pvcSpec.PVName // this ensures we get the right PersistentVolume
		pvc.Spec.StorageClassName = &pvcSpec.StorageClass
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(
				pvcSpec.Size,
			),
		}
		if err := client.IgnoreAlreadyExists(r.Create(ctx, pvc)); err == nil {
			log.Info("Created PersistentVolumeClaim", "pvc", pvcSpec,
				"namespace", spec.Namespace)
		} else {
			log.Error(err, "Failed to create PersistentVolumeClaim",
				"pvc", pvcSpec, "namespace", spec.Namespace)
		}
	}
	return nil
}

func (r *StorageReconciler) DeletePersistentVolumeClaims(
	ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	for _, pvcSpec := range spec.Storage.PersistentVolumeClaims {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      pvcSpec.Name,
			Namespace: spec.Namespace}, pvc); err != nil {
			if errors.IsNotFound(err) {
				// PersistentVolumeClaim does not exist
				continue
			} else {
				log.Error(err, "Failed to get PersistentVolumeClaim",
					"pvc", pvcSpec)
				continue
			}
		}
		r.DeleteResource(ctx, pvc)
	}
	return nil
}

func (r *StorageReconciler) DeleteBlockStoreData(ctx context.Context, spec *corev1alpha1.WorkspaceSpec, status *corev1alpha1.WorkspaceStatus) error {
	log := log.FromContext(ctx)
	jobName := "delete-block-store"
	jobNamespace := spec.Namespace
	job := &batchv1.Job{}

	err := r.Client.Get(ctx, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, job)
	if err != nil && apierrors.IsNotFound(err) {
		log.Info("delete-block-store Job not found, creating new one", "job", jobName, "namespace", jobNamespace)
		newJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: jobNamespace,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{{
							Name:    "cleanup",
							Image:   "busybox",
							Command: []string{"sh", "-c", "rm -rf /workspace/*"},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "workspace-data",
								MountPath: "/workspace",
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: "workspace-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: spec.Storage.PersistentVolumeClaims[0].Name,
								},
							},
						}},
					},
				},
			},
		}
		if err := r.Client.Create(ctx, newJob); err != nil {
			log.Error(err, "Failed to create job", "job", jobName)
			return fmt.Errorf("failed to create job delete-block-store: %w", err)
		}
		log.Info("Job created", "job", jobName)
	}

	// Poll until job completes or fails - 60 seconds timeout - we want to ensure the job completes
	log.Info("Waiting for job to complete", "job", jobName)
	for i := 0; i < 60; i++ {
		err := r.Client.Get(ctx, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, job)
		if err != nil {
			log.Error(err, "Failed to get job status")
			return fmt.Errorf("failed to get job: %w", err)
		}
		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
				log.Info("Job completed")
				return nil
			}
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
				return fmt.Errorf("Job failed")
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for delete-block-store job to complete")
}
