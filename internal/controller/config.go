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
	"encoding/json"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigReconciler struct {
	Client
}

func (r *ConfigReconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Fetch the Config resource
	config := &corev1.ConfigMap{}

	if err := r.Get(ctx,
		client.ObjectKey{Name: spec.Namespace}, config); err != nil {
		if errors.IsNotFound(err) {
			// Create config resource
			config.Name = "workspace-config"
			config.Namespace = spec.Namespace
			if err = client.IgnoreAlreadyExists(
				r.Create(ctx, config)); err == nil {
				log.Info("Config resource created", "config", config.Name)
			} else {
				log.Error(err, "Failed to create Config resource",
					"namespace", spec.Namespace)
				return err
			}
		} else {
			log.Error(err, "Failed to get Config resource",
				"namespace", spec.Namespace)
			return err
		}
	}

	data, err := r.createConfigData(spec, status)
	if err != nil {
		log.Error(err, "Failed to create Config data",
			"namespace", spec.Namespace)
		return err
	}

	if !mapsEqual(config.Data, data) {
		// Config out of date, update it
		config.Data = data
		if err = r.Update(ctx, config); err == nil {
			log.Info("Config resource updated", "config", config.Name)
		} else {
			log.Error(err, "Failed to update Config resource",
				"namespace", spec.Namespace)
			return err
		}
	}

	return nil
}

func (r *ConfigReconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Fetch the Config resource
	config := &corev1.ConfigMap{}

	if err := r.Get(ctx,
		client.ObjectKey{Name: spec.Namespace}, config); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted
			return nil
		} else {
			log.Error(err, "Failed to get Config resource",
				"namespace", spec.Namespace)
			return err
		}
	}

	if err := r.DeleteResource(ctx, config); err != nil {
		return err
	}

	return nil
}

func (r *ConfigReconciler) createConfigData(
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) (map[string]string, error) {

	data := make(map[string]string)

	for _, bucket := range status.AWS.S3.Buckets {
		data[bucket.EnvVar] = bucket.AccessPointARN
	}

	type PVCMap struct {
		PVCName string `json:"pvcName"`
		PVName  string `json:"pvName"`
	}
	var pvcMaps []PVCMap
	for _, pvc := range spec.Storage.PersistentVolumeClaims {
		pvcMaps = append(pvcMaps, PVCMap{
			PVCName: pvc.Name,
			PVName:  pvc.PVName,
		})
	}
	pvcMapsJSON, err := json.Marshal(pvcMaps)
	if err != nil {
		return nil, err
	}
	data["pvcs"] = string(pvcMapsJSON)
	return data, nil
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}
