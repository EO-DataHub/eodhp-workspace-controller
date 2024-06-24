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
	"log"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/apache/pulsar-client-go/pulsar"
)

type EventsClient struct {
	pulsar   pulsar.Client
	producer pulsar.Producer
	queue    chan Event
}

type Event struct {
	Event  string
	Spec   corev1alpha1.WorkspaceSpec
	Status corev1alpha1.WorkspaceStatus
}

func (e *Event) ToJSON(event Event) ([]byte, error) {
	// Convert struct to JSON
	jsonMessage, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	return jsonMessage, nil
}

func NewEventsClient(pulsarURL, topic string) (*EventsClient, error) {
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarURL,
	})

	if err != nil {
		return nil, err
	}

	// Create a producer on the topic
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})

	if err != nil {
		return nil, err
	}

	return &EventsClient{
		pulsar:   client,
		producer: producer,
		queue:    make(chan Event)}, nil
}

func (c *EventsClient) Notify(message Event) error {
	c.queue <- message
	return nil
}

func (c *EventsClient) Listen() {
	for event := range c.queue {
		// Convert map to JSON
		if jsonMessage, err := json.Marshal(event); err == nil {
			// Send a message
			if _, err := c.producer.Send(context.Background(),
				&pulsar.ProducerMessage{Payload: []byte(jsonMessage)}); err != nil {
				log.Printf("Failed to send message: %v", err)
			}
		} else {
			log.Printf("Failed to marshal message: %v", err)
		}
	}
}

func (c *EventsClient) Close() {
	c.producer.Close()
	c.pulsar.Close()
}
