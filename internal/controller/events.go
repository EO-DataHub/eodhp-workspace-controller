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

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/apache/pulsar-client-go/pulsar"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EventsReconciler struct {
	Client
}


func SendWorkspaceUpdateMessage(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus,
	pulsarURL string,) error {

	log := log.FromContext(ctx)

	// Send request to create these files via Pulsar
	log.Info("Creating pulsar!")
	// Create a Pulsar client
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarURL,
	})

	if err != nil {
		return err
	}

	// Create a producer on the topic
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: "workspace-controller",
	})

	if err != nil {
		return err
	}

	// Construct output message
	combined := map[string]interface{}{
		"spec":   spec,
		"status": status,
		"event": "update",
	}
	
	// Convert map to JSON
	jsonMessage, err := json.Marshal(combined)
	if err != nil {
		return err
	}


	// Send a message
	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: []byte(jsonMessage),
	})

	if err != nil {
		return err
	}

	// Close the producer and client when no longer needed
	producer.Close()
	client.Close()

	log.Info("Sent create command to transformer")

	return nil
}

func SendWorkspaceDeleteMessage(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus,
	pulsarURL string,) error {

	log := log.FromContext(ctx)

	// Send request to delete these files via Pulsar
	log.Info("Creating pulsar!")

	// Create a Pulsar client
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarURL,
	})

	if err != nil {
		return err
	}

	// Create a producer on the topic
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: "workspace-controller",
	})

	if err != nil {
		return err
	}

	// Construct output message
	combined := map[string]interface{}{
		"spec":   spec,
		"status": status,
		"event": "delete",
	}
	
	// Convert map to JSON
	jsonMessage, err := json.Marshal(combined)
	if err != nil {
		return err
	}


	// Send a message
	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: []byte(jsonMessage),
	})

	if err != nil {
		return err
	}

	// Close the producer and client when no longer needed
	producer.Close()
	client.Close()

	log.Info("Sent create command to transformer")

	return nil
}