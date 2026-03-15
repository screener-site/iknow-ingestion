package zendesk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/screener-site/iknow-ingestion/internal/adapter"
)

// Adapter implements adapter.Adapter for Zendesk webhooks.
type Adapter struct {
	webhookSecret string
}

// New creates a Zendesk adapter with the given HMAC webhook secret.
func New(webhookSecret string) *Adapter {
	return &Adapter{webhookSecret: webhookSecret}
}

func (a *Adapter) Source() string { return "zendesk" }

// VerifyRequest validates the Zendesk webhook signature.
//
// Zendesk signs: base64(HMAC-SHA256(webhookSecret, timestamp + nonce + body))
// Headers used:
//   - X-Zendesk-Webhook-Signature            — base64-encoded HMAC digest
//   - X-Zendesk-Webhook-Signature-Timestamp  — Unix timestamp string
//   - X-Zendesk-Webhook-Signature-Nonce      — random nonce string
//
// Docs: https://developer.zendesk.com/documentation/webhooks/verifying/
func (a *Adapter) VerifyRequest(r *http.Request, body []byte) error {
	signature := r.Header.Get("X-Zendesk-Webhook-Signature")
	timestamp := r.Header.Get("X-Zendesk-Webhook-Signature-Timestamp")
	nonce := r.Header.Get("X-Zendesk-Webhook-Signature-Nonce")

	if signature == "" || timestamp == "" || nonce == "" {
		return fmt.Errorf("missing Zendesk signature headers")
	}

	// Reconstruct the signed content: timestamp + nonce + raw body (no separator).
	signed := timestamp + nonce + string(body)

	mac := hmac.New(sha256.New, []byte(a.webhookSecret))
	mac.Write([]byte(signed))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("webhook signature mismatch")
	}
	return nil
}

// ParseEvent parses a Zendesk webhook payload into a normalized adapter.Event.
//
// Zendesk sends JSON payloads whose structure depends on the trigger configuration.
// We attempt to detect a meaningful event_type from common top-level fields, and
// fall back to "ticket.event" when the structure is unrecognised.
func (a *Adapter) ParseEvent(ctx context.Context, body []byte, headers http.Header, orgID, integrationID string) (*adapter.Event, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("zendesk: body is not valid JSON")
	}

	eventType := ticketEventType(body)
	webhookID := headers.Get("X-Zendesk-Webhook-Id")

	return &adapter.Event{
		// EventID is intentionally left empty; envelope.New will generate a UUID.
		ReceivedAt:    time.Now().UnixMilli(),
		Source:        a.Source(),
		OrgID:         orgID,
		IntegrationID: integrationID,
		EventType:     eventType,
		Payload:       body,
		WebhookID:     webhookID,
	}, nil
}

// ticketEventType inspects the top-level keys of a Zendesk webhook payload to
// infer an event type string.  Returns a best-effort string; never errors.
func ticketEventType(body []byte) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return "ticket.event"
	}

	// Zendesk ticket trigger payloads often include a "ticket" key.
	// The presence of nested keys lets us distinguish created vs updated.
	if ticketRaw, ok := raw["ticket"]; ok {
		var ticket map[string]json.RawMessage
		if err := json.Unmarshal(ticketRaw, &ticket); err == nil {
			// "via" with source "web" and no prior updates → created.
			// The most reliable signal available without additional API calls
			// is the "status" field transitioning from "" / "new".
			if statusRaw, ok := ticket["status"]; ok {
				var status string
				if json.Unmarshal(statusRaw, &status) == nil && status == "new" {
					return "ticket.created"
				}
			}
			return "ticket.updated"
		}
	}

	// Zendesk "ticket_event" automation payloads use a "ticket_event" wrapper.
	if _, ok := raw["ticket_event"]; ok {
		return "ticket.event"
	}

	// Some webhook triggers send a flat payload with an "id" field directly.
	if _, ok := raw["id"]; ok {
		return "ticket.updated"
	}

	return "ticket.event"
}
