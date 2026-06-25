# Интеграции AF-phone-orchestrator

## Recovery-engine (NATS)

| Subject | Направление | Назначение |
|---------|-------------|------------|
| `af.recovery.request` | orchestrator → recovery | solve |
| `af.recovery.response` | recovery → orchestrator | план |
| `af.recovery.outcome` | orchestrator → recovery | успех/неудача |

### Solve request

```json
{
  "serial": "R5CY331L8NF",
  "xml_dump": "<hierarchy>...</hierarchy>",
  "screenshot_key": "R5CY331L8NF/20260623-150820.png",
  "scenario": "Meta Terms",
  "context": "нажать CONTINUE"
}
```

`screenshot_key` = `minio_key` из ответа orchestrator (`GET /phones/{serial}/screen`) или observer (`GET /screen/{serial}`).

## Orchestrator (HTTP)

| Endpoint | Данные |
|----------|--------|
| `GET /phones/{serial}/screen?timeout_sec=30` | прокси к observer: `minio_key`, `screenshot_url`, resolution |

Клиент **не** обращается к observer напрямую — только через orchestrator.

## Observer (HTTP, внутренний)

| Endpoint | Данные |
|----------|--------|
| `GET /screen/{serial}` | `minio_key`, resolution |
| `GET /ui/{serial}?format=xml` | `xml_dump`, `package_name` |

Env: `OBSERVER_HTTP_URL` (dev: `http://127.0.0.1:19090`).

## Executor (gRPC)

Реальный транспорт: gRPC `EXECUTOR_GRPC_ADDR` (по умолчанию `localhost:50051`).

| Метод | Назначение |
|-------|------------|
| `Execute` | batch плана recovery (tap + wait) |
| `Tap` | прямой tap из `POST /phones/{serial}/tap` |

`EXECUTOR_MODE=stub` — только для локальной отладки без телефона.

Перед выполнением orchestrator **refine** координат tap из `permission_allow_button` в XML.

## Provisioner (HTTP)

Orchestrator вызывает **phone-provisioner** при состояниях `wifi_setup` … `auth`:

| Действие | Endpoint provisioner |
|----------|----------------------|
| Запуск настройки | `POST /provision` |
| Polling статуса | `GET /status?serial=` |

Env orchestrator:

| Переменная | По умолчанию |
|------------|--------------|
| `PROVISIONER_MODE` | `http` (или `stub` для локальных тестов) |
| `PROVISIONER_HTTP_URL` | `http://127.0.0.1:19092` |
| `PROVISIONER_DEFAULT_PROXY_IP` | — |
| `PROVISIONER_DEFAULT_PROXY_PORT` | `3128` |
| `PROVISIONER_DEFAULT_WIFI_SSID` | — |

При `ready` в provisioner orchestrator переводит телефон в `ready` → `working`.

`POST /phones` принимает опционально `wifi_ssid`, `proxy_ip`, `apps` — передаются в provisioner.

## Connector

Планируется gRPC `:50052` для ADB-сессий перед сценариями.
