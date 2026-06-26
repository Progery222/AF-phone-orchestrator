# Build context is the repository root (E:\Dev\AF) so the local Go module
# replaces resolve inside the builder:
#   ../AF-phone-action-executor (shared executor proto)
#   ../AF-phone-connector       (connector gRPC client)
#   ../AF-contacts-manager      (contacts gRPC client)
#   ../AF-video-generator       (video gRPC client)
FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY AF-phone-action-executor/ ./AF-phone-action-executor/
COPY AF-phone-connector/ ./AF-phone-connector/
COPY AF-contacts-manager/ ./AF-contacts-manager/
COPY AF-video-generator/ ./AF-video-generator/
COPY AF-phone-orchestrator/ ./AF-phone-orchestrator/
WORKDIR /src/AF-phone-orchestrator
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /phone-orchestrator ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /phone-orchestrator /phone-orchestrator
EXPOSE 50050 9090
USER nonroot:nonroot
ENTRYPOINT ["/phone-orchestrator"]
