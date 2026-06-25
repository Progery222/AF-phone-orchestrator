# AF-phone-orchestrator

Центральный координатор платформы **AF** (Android Farm): FSM телефонов, heartbeat, pause→recovery, вызов соседних микросервисов.

## Роль по ТЗ

| Зона ответственности | Да | Нет |
|----------------------|----|-----|
| FSM `new → … → working / paused / error` | ✓ | |
| HTTP API `/phones`, `/stats` | ✓ | |
| ADB, screencap, tap | | ✓ (connector / observer / executor) |
| LLM и планы recovery | | ✓ (recovery-engine) |

Оркестратор **только координирует**: не трогает устройство напрямую.

## Архитектура

Hexagonal (ports & adapters), Go 1.22+.

```
cmd/server/          — HTTP :19091, gRPC :50050
internal/service/    — FSM, RecoveryFlowService, refine tap из XML
internal/adapter/    — NATS recovery, gRPC executor, HTTP observer
```

## FSM

```
new → wifi_setup → proxy_setup → apps_install → auth → ready → working
                              ↓                  ↓
                            error ←───────────── paused → recovery → working
                              ↓
                           retired
```

## Связи с микросервисами

```
                    ┌─────────────────┐
                    │  orchestrator   │
                    │  HTTP :19091    │
                    └────────┬────────┘
         HTTP              │              NATS                gRPC
    ┌──────────────────────┼──────────────────────┬──────────────────┐
    ▼                      ▼                      ▼                  ▼
phone-observer      recovery-engine         af.recovery.*      phone-action-executor
:19090 (dev)        :9094 / :50054          request/response   :50051
MinIO скрины        Postgres + Ollama       outcome            tap/swipe/batch
```

| Сервис | Транспорт | Назначение |
|--------|-----------|------------|
| **phone-observer** | HTTP `OBSERVER_HTTP_URL` | `CaptureScreen`, `DumpUI` |
| **recovery-engine** | NATS `af.recovery.request` → `af.recovery.response` | LLM-план (tap/wait/back) |
| **phone-action-executor** | gRPC `EXECUTOR_GRPC_ADDR` | выполнение плана на телефоне |
| **phone-connector** | gRPC `:50052` (stub) | ADB-сессии, provision |
| **NATS** | `phone.state.changed` | события смены состояния |

Подробные контракты: [INTEGRATIONS.md](INTEGRATIONS.md).

## Ключевые сценарии (ТЗ)

| # | Действие | Endpoint |
|---|----------|----------|
| 1 | Добавить телефон | `POST /phones` |
| 2 | Heartbeat в WORKING | автоматически в цикле |
| 3 | Pause → recovery | `POST /phones/{serial}/pause?reason=…` |
| 7 | Debug recovery | `POST /recovery/run` |
| 8 | Observe / tap | `GET /phones/{serial}/observe`, `POST …/tap` |

Перед executor координаты tap **уточняются из XML** (`permission_allow_button` → центр bounds).

## Запуск (реальный телефон)

```powershell
$env:HEALTH_ADDR=":19091"
$env:OBSERVER_HTTP_URL="http://127.0.0.1:19090"
$env:EXECUTOR_GRPC_ADDR="127.0.0.1:50051"
$env:NATS_URL="nats://127.0.0.1:4222"
$env:RECOVERY_TIMEOUT_SEC="180"
$env:STORE_MODE="memory"   # или postgres
# НЕ задавать EXECUTOR_MODE=stub
go run ./cmd/server
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `HEALTH_ADDR` | `:9090` | HTTP API (dev: `:19091`) |
| `GRPC_ADDR` | `:50050` | gRPC orchestrator |
| `OBSERVER_HTTP_URL` | `http://127.0.0.1:19090` | observer REST |
| `EXECUTOR_GRPC_ADDR` | `localhost:50051` | executor gRPC |
| `PROVISIONER_MODE` | `stub` | `http` для phone-provisioner |
| `PROVISIONER_HTTP_URL` | `http://127.0.0.1:19092` | REST provisioner |
| `NATS_URL` | `nats://localhost:4222` | брокер |
| `RECOVERY_TIMEOUT_SEC` | `120` | таймаут NATS solve |
| `STORE_MODE` | postgres | `memory` для локальных тестов |
| `ORCHESTRATOR_TICK_SEC` | `2` | период FSM-цикла |

## Тесты

```bash
go test ./...
```

E2E bundle: `tests/e2e/`, скрипт `scripts/run-e2e-bundle.ps1`.

## Репозиторий

https://github.com/Progery222/AF-phone-orchestrator — ветка `main`.
