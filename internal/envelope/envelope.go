package envelope

import (
	"time"

	"github.com/google/uuid"
	"github.com/tech-screen/iknow-ingestion/internal/adapter"
)

// RawEvent is the struct that gets serialized and published to Kafka.
// It matches the iknow.zendesk.RawEvent Avro schema (schema_version 1.0).
type RawEvent struct {
	EventID       string `json:"event_id"`
	ReceivedAt    int64  `json:"received_at"`
	Source        string `json:"source"`
	OrgID         string `json:"org_id"`
	IntegrationID string `json:"integration_id"`
	EventType     string `json:"event_type"`
	Payload       string `json:"payload"` // JSON string
	WebhookID     string `json:"webhook_id,omitempty"`
	SchemaVersion string `json:"schema_version"`
}

// New creates a RawEvent envelope from an adapter Event.
func New(e *adapter.Event) *RawEvent {
	id := e.EventID
	if id == "" {
		id = uuid.New().String()
	}
	ts := e.ReceivedAt
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return &RawEvent{
		EventID:       id,
		ReceivedAt:    ts,
		Source:        e.Source,
		OrgID:         e.OrgID,
		IntegrationID: e.IntegrationID,
		EventType:     e.EventType,
		Payload:       string(e.Payload),
		WebhookID:     e.WebhookID,
		SchemaVersion: "1.0",
	}
}
