# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /phone-orchestrator ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /phone-orchestrator /phone-orchestrator
EXPOSE 50050 9090
USER nonroot:nonroot
ENTRYPOINT ["/phone-orchestrator"]
