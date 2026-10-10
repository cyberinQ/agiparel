FROM golang:1.25-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY services/ services/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/plant-edge-os ./services/edge-daemon/cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
RUN mkdir -p /data/wal
COPY --from=builder /bin/plant-edge-os /app/plant-edge-os

EXPOSE 8080
ENTRYPOINT ["/app/plant-edge-os"]