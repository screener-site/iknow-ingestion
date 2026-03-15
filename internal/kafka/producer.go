package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// Producer wraps a confluent-kafka-go producer with a fixed topic.
type Producer struct {
	p     *kafka.Producer
	topic string
}

// NewProducer creates a new Kafka producer connected to bootstrap with the given topic.
func NewProducer(bootstrap, topic string) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":        bootstrap,
		"acks":                     "all",
		"retries":                  5,
		"retry.backoff.ms":         200,
		"socket.keepalive.enable":  true,
		"log.connection.close":     false,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewProducer: %w", err)
	}
	return &Producer{p: p, topic: topic}, nil
}

// Publish marshals value to JSON and produces it to the topic with the given key.
// Key is used for partition affinity (e.g. "orgID:ticketID").
// The call blocks until the message is acknowledged or the context is cancelled.
func (p *Producer) Publish(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	err = p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &p.topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(key),
		Value: data,
	}, deliveryChan)
	if err != nil {
		return fmt.Errorf("produce: %w", err)
	}

	// Wait for delivery confirmation or context cancellation.
	select {
	case e := <-deliveryChan:
		msg, ok := e.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected delivery event type: %T", e)
		}
		if msg.TopicPartition.Error != nil {
			return fmt.Errorf("delivery failed: %w", msg.TopicPartition.Error)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled waiting for delivery: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timed out waiting for kafka delivery")
	}
}

// Close flushes pending messages and releases producer resources.
func (p *Producer) Close() {
	// Flush with a 10-second timeout to drain any buffered messages.
	p.p.Flush(10_000)
	p.p.Close()
}
