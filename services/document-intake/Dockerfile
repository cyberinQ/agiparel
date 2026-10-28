FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
COPY services/ services/
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/document-intake ./services/document-intake/cmd/worker
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata poppler-utils
WORKDIR /app
COPY --from=builder /bin/document-intake /app/document-intake
USER 65532:65532
ENTRYPOINT ["/app/document-intake"]
