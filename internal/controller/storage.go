package controller

import (
	"context"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WorkspaceReconciler) ReconcilePersistentVolume(
	ctx context.Context, name, namespace string, storage *corev1alpha1.StorageSpec,
	csi *corev1.CSIPersistentVolumeSource) error {
	log := log.FromContext(ctx)

	pv := &corev1.PersistentVolume{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace}, pv); err == nil {
		return nil // PersistentVolume already exists.
	} else {
		if errors.IsNotFound(err) {
			// PersistentVolume does not exist.
			log.Info("PersistentVolume for workspace does not exist",
				"name", name, "namespace", namespace, "storage", storage)
			// Continue.
		} else {
			log.Error(err, "Failed to get PersistentVolume",
				"name", name, "namespace", namespace, "storage", storage)
			return err
		}
	}

	// Create block storage
	pv.Name = name
	pv.Namespace = namespace
	pv.Spec.StorageClassName = storage.StorageClass
	pv.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	pv.Spec.Capacity = corev1.ResourceList{
		corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(
			r.getStorageCapacity(storage),
		),
	}
	volumeMode := corev1.PersistentVolumeFilesystem
	pv.Spec.VolumeMode = &volumeMode
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	pv.Spec.CSI = csi
	if err := client.IgnoreAlreadyExists(r.Create(ctx, pv)); err == nil {
		log.Info("PersistentVolume created", "name", name,
			"namespace", namespace, "storage", storage)
		return nil
	} else {
		return err
	}
}

func (r *WorkspaceReconciler) DeletePersistentVolume(
	ctx context.Context, name, namespace string) error {

	log := log.FromContext(ctx)

	// Delete block storage
	pv := &corev1.PersistentVolume{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace}, pv); err != nil {
		if errors.IsNotFound(err) {
			// PersistentVolumeClaim does not exist
			return nil
		} else {
			log.Error(err, "Failed to get PersistentVolumeClaim", "pv", name)
			return err
		}
	}
	if err := r.Delete(ctx, pv); err != nil {
		log.Error(err, "Failed to delete PersistentVolumeClaim", "pv", name)
		return err
	} else {
		log.Info("PersistentVolumeClaim deleted", "pv", name)
		return nil
	}
}

func (r *WorkspaceReconciler) ReconcilePersistentVolumeClaim(
	ctx context.Context, name, namespace, pvName string,
	storage *corev1alpha1.StorageSpec) error {

	log := log.FromContext(ctx)

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace}, pvc); err == nil {
		return nil // PersistentVolumeClaim already exists.
	} else {
		if errors.IsNotFound(err) {
			// PersistentVolumeClaim does not exist.
			log.Info("PersistentVolumeClaim for workspace does not exist",
				"name", name, "namespace", namespace, "storage", storage)
			// Continue.
		} else {
			log.Error(err, "Failed to get PersistentVolumeClaim",
				"name", name, "namespace", namespace, "storage", storage)
			return err
		}
	}
	// Create persistent volume claim
	pvc.Name = name
	pvc.Namespace = namespace
	pvc.Spec.VolumeName = pvName // this ensures we get the right PersistentVolume
	pvc.Spec.StorageClassName = &storage.StorageClass
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	pvc.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(
			r.getStorageCapacity(storage),
		),
	}
	if err := client.IgnoreAlreadyExists(r.Create(ctx, pvc)); err == nil {
		log.Info("PersistentVolumeClaim created", "name", name,
			"namespace", namespace, "storage", storage)
		return nil
	} else {
		return err
	}
}

func (r *WorkspaceReconciler) DeletePersistentVolumeClaim(
	ctx context.Context, name, namespace string) error {

	log := log.FromContext(ctx)
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace}, pvc); err != nil {
		if errors.IsNotFound(err) {
			// PersistentVolumeClaim does not exist
			return nil
		} else {
			log.Error(err, "Failed to get PersistentVolumeClaim", "pvc", name)
			return err
		}
	}
	if err := r.Delete(ctx, pvc); err != nil {
		log.Error(err, "Failed to delete PersistentVolumeClaim", "pvc", name)
		return err
	} else {
		log.Info("PersistentVolumeClaim deleted", "pvc", name)
		return nil
	}
}

func (r *WorkspaceReconciler) getStorageCapacity(storage *corev1alpha1.StorageSpec) string {
	if storage.Size != "" {
		return storage.Size
	} else if r.config.Storage.DefaultSize != "" {
		return r.config.Storage.DefaultSize
	} else {
		return "2Gi"
	}
}
