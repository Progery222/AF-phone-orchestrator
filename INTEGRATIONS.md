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

`screenshot_key` = `minio_key` из observer (`GET /screen/{serial}`).

## Observer (HTTP)

| Endpoint | Данные |
|----------|--------|
| `GET /screen/{serial}` | `minio_key`, resolution |
| `GET /ui/{serial}?format=xml` | `xml_dump`, `package_name` |

Env: `OBSERVER_HTTP_URL` (dev: `http://127.0.0.1:19090`).

## Executor

Пока **stub** — план логируется, жесты не выполняются. Целевой транспорт: gRPC `:50051`.

## Connector

Планируется gRPC `:50052` для ADB-сессий перед сценариями.
