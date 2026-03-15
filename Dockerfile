# ---- build stage ----
FROM golang:1.23-alpine AS builder

# Install gcc and musl-dev required by confluent-kafka-go (CGO).
RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# Download dependencies before copying source for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/webhook ./cmd/webhook

# ---- run stage ----
# distroless/static does not include a shell — use cc variant for CGO binaries.
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=builder /app/webhook /app/webhook

EXPOSE 8080

CMD ["/app/webhook"]
