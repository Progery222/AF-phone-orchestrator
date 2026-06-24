module github.com/mobilefarm/af/phone-orchestrator

go 1.25.0

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/mobilefarm/af/phone-action-executor v0.0.0
	github.com/nats-io/nats-server/v2 v2.10.24
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/grpc v1.69.2
)

replace github.com/mobilefarm/af/phone-action-executor => ../AF-phone-action-executor

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.3 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
)
