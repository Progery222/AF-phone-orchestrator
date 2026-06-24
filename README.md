# AF-phone-orchestrator

Микросервис координации платформы **AF**: жизненный цикл телефонов (FSM), вызов connector/provisioner/observer/recovery/executor.

- HTTP API управления телефонами (`/phones`, `/stats`) — по ТЗ v4
- Главный цикл FSM: ticker 2–5 с, per-phone goroutines, in-memory lock (Redis — позже)
- Postgres: `phones`, `phone_state_log`, `phone_tasks`
- NATS: `phone.state.changed`, recovery `af.recovery.request` / `response`
- gRPC `:50050` — scaffold; HTTP health `:9090`
- Hexagonal architecture (Go 1.22+)

## FSM телефона

```
new → wifi_setup → proxy_setup → apps_install → auth → ready → working
                              ↓                  ↓
                            error ←───────────── paused
                              ↓
                           retired
```

Оркестратор **не** выполняет ADB, dump, tap и LLM — только координация.

## Сценарии (ТЗ)

| # | Сценарий | Реализация |
|---|----------|------------|
| 1 | Настройка нового телефона | FSM + stub connector/provisioner |
| 2 | Heartbeat в WORKING | обновление `last_heartbeat` |
| 3 | PAUSED → recovery | `POST /phones/{serial}/pause` → RecoveryFlowService |
| 4 | ERROR | `MarkError`, ожидание ручного вмешательства |
| 5 | Ручной reprovision | `POST /phones/{serial}/reprovision` → `new` |

## Запуск

```bash
go mod tidy
go test ./...
# без Postgres:
STORE_MODE=memory RECOVERY_MODE=stub go run ./cmd/server
```

### Postgres (Docker)

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
# Windows: .\scripts\setup-db.ps1
```

### Env

| Переменная | По умолчанию |
|------------|--------------|
| `GRPC_ADDR` | `:50050` |
| `HEALTH_ADDR` | `:9090` |
| `POSTGRES_DSN` | `postgres://orchestrator:orchestrator@localhost:5434/orchestrator?sslmode=disable` |
| `STORE_MODE` | postgres (`memory` для тестов без БД) |
| `ORCHESTRATOR_TICK_SEC` | `2` |
| `PHONE_LOCK_TTL_SEC` | `30` |
| `OBSERVER_HTTP_URL` | `http://127.0.0.1:19090` |
| `NATS_URL` | `nats://localhost:4222` |
| `NATS_SUBJECT_STATE_CHANGED` | `phone.state.changed` |
| `RECOVERY_MODE` | `nats` (`stub` для локальной отладки) |

### HTTP API

```bash
# Добавить телефон
curl -X POST http://localhost:9090/phones -H "Content-Type: application/json" -d '{"serial":"R5CY331L8NF"}'

# Список и статистика
curl http://localhost:9090/phones
curl http://localhost:9090/stats

# Пауза → recovery
curl -X POST "http://localhost:9090/phones/R5CY331L8NF/pause?reason=Meta%20Terms"

# Возобновить / перенастроить
curl -X POST http://localhost:9090/phones/R5CY331L8NF/resume
curl -X POST http://localhost:9090/phones/R5CY331L8NF/reprovision
```

Debug recovery: `POST /recovery/run`, `POST /recovery/outcome` — см. [INTEGRATIONS.md](INTEGRATIONS.md).

## Соседние сервисы

| Сервис | Репозиторий | Порт |
|--------|-------------|------|
| phone-connector | AF-phone-connector | `:50052` |
| phone-action-executor | AF-phone-action-executor | `:50051` |
| phone-observer | AF-phone-observer | HTTP |
| recovery-engine | AF-recovery-engine | `:50054` |

## GitHub Flow

Не пушить в `main` напрямую — feature-ветка + PR. Conventional Commits.
