package adapter

import (
	"context"
	"net/http"
)

// Event is the normalized representation of a webhook event from any source.
// It maps directly to the iknow.zendesk.RawEvent Avro schema.
type Event struct {
	EventID       string
	ReceivedAt    int64  // unix millis
	Source        string
	OrgID         string
	IntegrationID string
	EventType     string
	Payload       []byte // raw JSON from source
	WebhookID     string
}

// Adapter is the contract every integration source must implement.
type Adapter interface {
	// Source returns the source identifier (e.g. "zendesk", "github")
	Source() string
	// VerifyRequest validates the webhook request signature.
	// Returns nil if valid, error with message if invalid.
	VerifyRequest(r *http.Request, body []byte) error
	// ParseEvent extracts a normalized Event from the raw webhook body and headers.
	ParseEvent(ctx context.Context, body []byte, headers http.Header, orgID, integrationID string) (*Event, error)
}
