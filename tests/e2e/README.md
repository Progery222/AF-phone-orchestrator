# E2E: связка orchestrator + observer + recovery + executor

## Поток

```
Orchestrator (FSM /pause, GET /phones/{serial}/screen)
    → Observer HTTP: GET /screen/{serial}, GET /ui/{serial}
    → Recovery NATS: af.recovery.request → af.recovery.response
    → Executor (stub): ExecutePlan
    → Observer: повторный screen
    → Recovery NATS: af.recovery.outcome
```

Executor в orchestrator пока **stub** (жесты не уходят на `:50051`), но health executor проверяется в e2e.

## Сценарии (`tests/e2e/bundle_e2e_test.go`)

| # | Тест | Что проверяет |
|---|------|----------------|
| 1 | `TestBundle_HealthAllServices` | `/health` и `/ready` всех сервисов |
| 2 | `TestBundle_OrchestratorScreenViaObserver` | `GET /phones/{serial}/screen` → orchestrator → observer → `minio_key` + `screenshot_url` |
| 3 | `TestBundle_RecoveryRun_LoginScenario` | `POST /recovery/run` → LLM/stub план с `error_hash` |
| 4 | `TestBundle_RecoveryRun_CachedFromDB` | повторный solve → тот же hash, source `db` |
| 5 | `TestBundle_OrchestratorSetupPauseRecovery` | FSM: add phone → working → pause → recovery → working + `last_error_hash` |
| 6 | `TestBundle_RecoveryOutcomeAccepted` | `POST /recovery/outcome` |

## Запуск

```powershell
cd AF-orkestrator
make e2e
# или
powershell -ExecutionPolicy Bypass -File scripts/run-e2e-bundle.ps1
```

Скрипт поднимает: NATS (`cmd/natsdev`), observer, recovery-engine, orchestrator e2e (`:19092`).  
Executor должен быть запущен отдельно (`:9091`) или health-тест упадёт.

Тестовый serial: **`stub`** (без реального ADB). Serial с префиксом `E2E-` в observer тоже работает как stub.

## Env для ручного прогона

| Переменная | По умолчанию |
|------------|--------------|
| `E2E_ORCH_URL` | `http://127.0.0.1:19092` |
| `E2E_OBSERVER_URL` | `http://127.0.0.1:19090` |
| `E2E_RECOVERY_HEALTH` | `http://127.0.0.1:9094` |
| `E2E_EXECUTOR_HEALTH` | `http://127.0.0.1:9091` |

```bash
go test -tags=e2e ./tests/e2e/... -v
```
