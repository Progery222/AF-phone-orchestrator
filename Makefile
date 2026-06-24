.PHONY: build run test proto docker-build tidy

BINARY=phone-orchestrator
MAIN=./cmd/server

build:
	go build -o bin/$(BINARY) $(MAIN)

run:
	go run $(MAIN)

test:
	go test ./...

tidy:
	go mod tidy

proto:
	protoc -I proto \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		proto/common/v1/phone.proto \
		proto/orchestrator/v1/orchestrator.proto

docker-build:
	docker build -f deploy/Dockerfile -t af-phone-orchestrator:latest .
