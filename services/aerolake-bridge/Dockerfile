FROM golang:alpine AS builder
WORKDIR /app

# Enable toolchain auto-switching if go.mod requires a newer micro/minor release
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY services/aerolake-bridge/ services/aerolake-bridge/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /aerolake-bridge ./services/aerolake-bridge

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /aerolake-bridge /usr/local/bin/aerolake-bridge

ENTRYPOINT ["/usr/local/bin/aerolake-bridge"]
