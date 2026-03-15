# iknow-ingestion

Webhook receiver and event publisher for the iknow platform.

Incoming webhooks are verified, normalised into a `RawEvent` envelope, and
published to a Kafka topic for downstream consumers.  The architecture is
adapter-based: adding a new integration requires only a new package under
`internal/` that satisfies the `adapter.Adapter` interface.

## Adapters

| Source      | Status      |
|-------------|-------------|
| Zendesk     | Implemented |
| GitHub      | TODO        |
| Jira        | TODO        |
| PagerDuty   | TODO        |

## Quick start

```bash
cp .env.example .env
# Fill in KAFKA_BOOTSTRAP, ZENDESK_WEBHOOK_SECRET, ZENDESK_ORG_ID, ZENDESK_INTEGRATION_ID

make run-webhook
# Server listens on :8080 by default (override with PORT=)
```

Webhook endpoint:

```
POST /{source}/webhook
```

Example for Zendesk:

```
POST /zendesk/webhook
```

## Environment variables

| Variable                  | Required | Default                  | Description                                      |
|---------------------------|----------|--------------------------|--------------------------------------------------|
| `KAFKA_BOOTSTRAP`         | yes      | —                        | Kafka bootstrap servers (host:port)              |
| `KAFKA_TOPIC_RAW`         | no       | `zendesk.ticket.raw`     | Topic to publish raw events to                   |
| `PORT`                    | no       | `8080`                   | HTTP listen port                                 |
| `LOG_LEVEL`               | no       | `info`                   | Log level: debug / info / warn / error           |
| `ZENDESK_WEBHOOK_SECRET`  | yes      | —                        | HMAC secret from the Zendesk webhook config      |
| `ZENDESK_ORG_ID`          | no       | —                        | Default org ID (overridden by X-Iknow-Org-Id header)        |
| `ZENDESK_INTEGRATION_ID`  | no       | —                        | Default integration ID (overridden by X-Iknow-Integration-Id header) |

## Adding a new adapter

1. Create `internal/<source>/adapter.go` and implement the three methods of
   `adapter.Adapter` (`Source`, `VerifyRequest`, `ParseEvent`).

2. Register the adapter in `cmd/webhook/main.go`:

```go
adapters := map[string]adapter.Adapter{
    "zendesk": zendesk.New(mustenv("ZENDESK_WEBHOOK_SECRET")),
    "github":  github.New(mustenv("GITHUB_WEBHOOK_SECRET")), // new
}
```

That's it — no changes to the HTTP routing or Kafka publishing logic.

## Make targets

```
make build          compile all packages
make test           run all tests
make run-webhook    run the webhook HTTP server
make lint           run golangci-lint
make docker-build   build the container image
make docker-push    push the container image
make tidy           go mod tidy
```

## Container image

```bash
# Build
make docker-build IMAGE=ghcr.io/screener-site/iknow-ingestion TAG=latest

# Push
make docker-push IMAGE=ghcr.io/screener-site/iknow-ingestion TAG=latest
```

Note: confluent-kafka-go uses CGO, so the build stage uses `golang:1.23-alpine`
with `gcc`/`musl-dev`, and the runtime stage uses `distroless/cc-debian12`
(not the static variant).
