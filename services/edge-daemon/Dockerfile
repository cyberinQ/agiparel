# syntax=docker/dockerfile:1
# Author: Sunil Jain, 2026-10-28
# Cache Go dependencies separately from application source so UI-only changes rebuild faster.
FROM golang:1.24-alpine AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY services/ services/
COPY scripts/ scripts/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/plant-edge-os ./services/edge-daemon/cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ingest-docs ./scripts/ingest_docs.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata poppler-utils
WORKDIR /app
RUN mkdir -p /data/wal /app/data/staging/docs /app/data/staging/output
COPY --from=builder /bin/plant-edge-os /app/plant-edge-os
COPY --from=builder /bin/ingest-docs /app/ingest-docs
EXPOSE 18530
ENTRYPOINT ["/app/plant-edge-os"]
