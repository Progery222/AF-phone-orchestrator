# syntax=docker/dockerfile:1

# Build context is the repository root (E:\Dev\AF) so the local Go module
# replace `../AF-phone-action-executor` resolves inside the builder.
FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY AF-phone-action-executor/ ./AF-phone-action-executor/
COPY AF-phone-orchestrator/ ./AF-phone-orchestrator/
WORKDIR /src/AF-phone-orchestrator
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /phone-orchestrator ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /phone-orchestrator /phone-orchestrator
EXPOSE 50050 9090
USER nonroot:nonroot
ENTRYPOINT ["/phone-orchestrator"]
