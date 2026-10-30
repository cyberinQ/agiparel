# Author: Sunil Jain, CyberinQ LLC, 2026-10-30.
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY services/aerolake-bridge/ services/aerolake-bridge/
RUN CGO_ENABLED=0 GOOS=linux go build -o /aerolake-bridge services/aerolake-bridge/main.go

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /aerolake-bridge /usr/local/bin/aerolake-bridge
ENTRYPOINT ["/usr/local/bin/aerolake-bridge"]
