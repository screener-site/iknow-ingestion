package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/screener-site/iknow-ingestion/internal/adapter"
	"github.com/screener-site/iknow-ingestion/internal/envelope"
	"github.com/screener-site/iknow-ingestion/internal/kafka"
	"github.com/screener-site/iknow-ingestion/internal/zendesk"
)

func main() {
	// Configure structured logger (respects LOG_LEVEL env var).
	initLogger(getenv("LOG_LEVEL", "info"))

	port := getenv("PORT", "8080")
	kafkaBootstrap := mustenv("KAFKA_BOOTSTRAP")
	kafkaTopic := getenv("KAFKA_TOPIC_RAW", "zendesk.ticket.raw")

	// Build adapter registry — add new integrations here.
	adapters := map[string]adapter.Adapter{
		"zendesk": zendesk.New(mustenv("ZENDESK_WEBHOOK_SECRET")),
	}

	// Build Kafka producer.
	producer, err := kafka.NewProducer(kafkaBootstrap, kafkaTopic)
	if err != nil {
		slog.Error("failed to create kafka producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	// HTTP routes.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{source}/webhook", makeWebhookHandler(adapters, producer))
	mux.HandleFunc("GET /health", healthHandler)

	slog.Info("starting webhook server", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// makeWebhookHandler returns an http.HandlerFunc that dispatches incoming
// webhook requests to the appropriate adapter, then publishes to Kafka.
func makeWebhookHandler(adapters map[string]adapter.Adapter, producer *kafka.Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := r.PathValue("source")
		a, ok := adapters[source]
		if !ok {
			http.Error(w, "unknown source", http.StatusNotFound)
			return
		}

		// Limit body to 1 MiB to guard against oversized payloads.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		if err := a.VerifyRequest(r, body); err != nil {
			slog.Warn("signature verification failed", "source", source, "err", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Prefer per-request headers; fall back to env vars configured at
		// startup (useful when a single instance serves one org/integration).
		orgID := r.Header.Get("X-Iknow-Org-Id")
		integrationID := r.Header.Get("X-Iknow-Integration-Id")
		if orgID == "" {
			orgID = os.Getenv("ZENDESK_ORG_ID")
		}
		if integrationID == "" {
			integrationID = os.Getenv("ZENDESK_INTEGRATION_ID")
		}

		event, err := a.ParseEvent(r.Context(), body, r.Header, orgID, integrationID)
		if err != nil {
			slog.Error("parse error", "source", source, "err", err)
			http.Error(w, "parse error", http.StatusBadRequest)
			return
		}

		// Use orgID:webhookID as the partition key for ordering per-org.
		// If there is a ticket ID embedded in the payload we could use that
		// instead; the TODO below is a natural extension point.
		partitionKey := fmt.Sprintf("%s:%s", event.OrgID, ticketID(event.Payload))

		env := envelope.New(event)

		if err := producer.Publish(r.Context(), partitionKey, env); err != nil {
			slog.Error("kafka publish failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		slog.Info("event published",
			"source", source,
			"event_id", env.EventID,
			"event_type", env.EventType,
			"org_id", env.OrgID,
		)

		w.WriteHeader(http.StatusAccepted)
	}
}

// ticketID attempts to extract a ticket ID from a Zendesk-style JSON payload
// so it can be used as a Kafka partition key for per-ticket ordering.
// Returns an empty string when the ID cannot be determined.
func ticketID(payload []byte) string {
	var outer struct {
		Ticket struct {
			ID int64 `json:"id"`
		} `json:"ticket"`
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(payload, &outer); err != nil {
		return ""
	}
	if outer.Ticket.ID != 0 {
		return fmt.Sprintf("%d", outer.Ticket.ID)
	}
	if outer.ID != 0 {
		return fmt.Sprintf("%d", outer.ID)
	}
	return ""
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// initLogger configures the default slog handler based on the LOG_LEVEL env var.
// Recognised levels: debug, info, warn, error.  Defaults to info.
func initLogger(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}

func mustenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
