# AF-phone-orchestrator

Микросервис координации платформы **AF**: сценарии, вызов observer/recovery/executor, NATS.

- gRPC API (proto `orchestrator/v1`) — scaffold, регистрация после `make proto`
- HTTP debug: `POST /recovery/run`, `POST /recovery/outcome`, health `:9090`
- Hexagonal architecture (Go 1.22+)

## Поток recovery

```
Orchestrator → Observer (screen + xml)
            → Recovery (NATS af.recovery.request / response)
            → Executor (stub, gRPC позже)
```

Контракт с recovery-engine: см. [INTEGRATIONS.md](INTEGRATIONS.md) и `AF-recovery-engine/ORCHESTRATOR.md`.

## Запуск

```bash
go mod tidy
go test ./...
go run ./cmd/server
```

### Env

| Переменная | По умолчанию |
|------------|--------------|
| `GRPC_ADDR` | `:50050` |
| `HEALTH_ADDR` | `:9090` |
| `OBSERVER_HTTP_URL` | `http://127.0.0.1:19090` |
| `NATS_URL` | `nats://localhost:4222` |
| `RECOVERY_MODE` | `nats` (`stub` для локальной отладки без NATS) |

### Пример HTTP

```bash
curl -X POST http://localhost:9090/recovery/run \
  -H "Content-Type: application/json" \
  -d '{"serial":"R5CY331L8NF","scenario":"Meta Terms","context":"нажать CONTINUE"}'
```

## Соседние сервисы

| Сервис | Репозиторий | Порт |
|--------|-------------|------|
| phone-connector | AF-phone-connector | `:50052` |
| phone-action-executor | AF-phone-action-executor | `:50051` |
| phone-observer | AF-phone-observer | `:50053` HTTP |
| recovery-engine | AF-recovery-engine | `:50054` |

## GitHub Flow

Не пушить в `main` напрямую — feature-ветка + PR. Conventional Commits.
