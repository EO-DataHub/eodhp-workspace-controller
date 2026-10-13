package controller

import (
	"context"
	"encoding/json"
	"log"

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	"github.com/apache/pulsar-client-go/pulsar"
)

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
