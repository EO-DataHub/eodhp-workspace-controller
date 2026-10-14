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
