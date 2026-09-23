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
	"fmt"
	"log"
	"os"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	"github.com/apache/pulsar-client-go/pulsar"
)

// DefaultPulsarTopic is the topic workspace events are published to when no
// topic is configured.
const DefaultPulsarTopic = "workspace-controller"

type PulsarConfig struct {
	URL string `yaml:"url"`
	// TokenFile is the path to a file containing a JWT used to authenticate
	// with Pulsar. The file is re-read whenever the client (re)authenticates,
	// so a rotated token is picked up without a restart. Empty means connect
	// without authentication.
	TokenFile string `yaml:"tokenFile"`
	// Topic to publish workspace events to. Defaults to DefaultPulsarTopic.
	Topic string `yaml:"topic"`
}

// TopicName returns the configured topic, or DefaultPulsarTopic if none is set.
func (c PulsarConfig) TopicName() string {
	if c.Topic == "" {
		return DefaultPulsarTopic
	}
	return c.Topic
}

type EventsClient struct {
	pulsar   pulsar.Client
	producer pulsar.Producer
	queue    chan Event
}

type Event struct {
	Event  string                       `json:"event"`
	Spec   corev1alpha1.WorkspaceSpec   `json:"spec"`
	Status corev1alpha1.WorkspaceStatus `json:"status"`
}

func (e *Event) ToJSON(event Event) ([]byte, error) {
	// Convert struct to JSON
	jsonMessage, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	return jsonMessage, nil
}

func NewEventsClient(config PulsarConfig) (*EventsClient, error) {
	options := pulsar.ClientOptions{
		URL: config.URL,
	}

	if config.TokenFile != "" {
		// Check the file up front so a missing secret mount gives a clear error
		f, err := os.Open(config.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("pulsar tokenFile is configured but cannot be read: %w", err)
		}
		f.Close()
		options.Authentication = pulsar.NewAuthenticationTokenFromFile(config.TokenFile)
	}

	client, err := pulsar.NewClient(options)

	if err != nil {
		return nil, err
	}

	// Create a producer on the topic
	topic := config.TopicName()
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})

	if err != nil {
		client.Close()
		return nil, fmt.Errorf("could not create producer on topic %q: %w", topic, err)
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
