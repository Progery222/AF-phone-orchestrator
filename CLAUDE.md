# CLAUDE.md

Руководство для AI-агентов в **AF-phone-orchestrator** — координатор сценариев Android Farm.

> Репозиторий: [Progery222/AF-phone-orchestrator](https://github.com/Progery222/AF-phone-orchestrator)

## Назначение

Управляет жизненным циклом телефонов (FSM), координирует connector, provisioner, observer, recovery-engine, executor. Не выполняет ADB, dump, tap и LLM сам.

## Стек

Go 1.22+, gRPC `:50050`, HTTP `:9090`, Postgres, NATS, in-memory lock (Redis — целевой).

## Архитектура

Hexagonal: `domain` → `port` → `service` → `adapter`.

| Слой | Ключевые файлы |
|------|----------------|
| domain | `internal/domain/phone.go` — FSM, Phone, stats |
| port | `internal/port/phone.go` — PhoneStore, PhoneLock, Connector, Provision |
| service | `orchestrator_service.go` — главный цикл FSM; `phone_service.go` — CRUD; `recovery_flow_service.go` — observer→recovery→executor |
| adapter | `phones_http.go`, `postgres_phone_store.go`, `nats_publisher.go`, driver stubs |

## FSM

`new → wifi_setup → proxy_setup → apps_install → auth → ready → working`, плюс `paused`, `error`, `retired`.

Главный цикл: `OrchestratorService.Run` — ticker (`ORCHESTRATOR_TICK_SEC`, по умолчанию 2), сразу первая итерация, per-phone goroutine + lock (`PHONE_LOCK_TTL_SEC`).

## HTTP API (ТЗ)

- `GET /phones`, `GET /phones/{serial}`, `POST /phones` (add)
- `POST /phones/{serial}/remove|pause|resume|reprovision`
- `GET /stats`

## NATS

| Subject | Направление |
|---------|-------------|
| `phone.state.changed` | out — смена состояния |
| `af.recovery.request` / `af.recovery.response` | recovery flow |

## Env

`POSTGRES_DSN`, `STORE_MODE=memory`, `ORCHESTRATOR_TICK_SEC`, `PHONE_LOCK_TTL_SEC`, `NATS_SUBJECT_STATE_CHANGED`, `RECOVERY_MODE=stub`, `OBSERVER_HTTP_URL`.

## GitHub Flow

Не пушить в `main` без PR. Conventional Commits.

## Связанные репозитории

- AF-phone-observer, AF-recovery-engine, AF-phone-connector, AF-phone-action-executor
