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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Client struct {
	client.Client
}

func (c *Client) DeleteResource(ctx context.Context,
	obj client.Object) error {

	log := log.FromContext(ctx)

	if !obj.GetDeletionTimestamp().IsZero() {
		// Already being deleted
		return nil
	}

	// Delete resource
	if err := c.Delete(ctx, obj); err == nil {
		log.Info(fmt.Sprintf("%s deleted", obj.GetObjectKind()),
			"obj", obj.GetName(), "namespace", obj.GetNamespace())
		return nil
	} else {
		log.Error(err, fmt.Sprintf("Failed to delete %s", obj.GetObjectKind()),
			"obj", obj.GetName(), "namespace", obj.GetNamespace())
		return err
	}
}
