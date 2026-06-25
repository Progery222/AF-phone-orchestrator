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

## Content-distributor (HTTP + NATS)

Контент **уже в MinIO** (загрузили другие сервисы). Orchestrator командует доставку.

### Orchestrator HTTP

| Endpoint | Назначение |
|----------|------------|
| `POST /phones/{serial}/content/register` | Зарегистрировать `object_key` из MinIO |
| `POST /phones/{serial}/content/download` | Async NATS → push (`content_id` или `object_key`) |
| `GET /phones/{serial}/content` | Список |
| `DELETE /phones/{serial}/content` | Удалить всё |
| `DELETE /phones/{serial}/content/{content_id}` | Удалить один файл |

### NATS subjects

| Subject | Payload |
|---------|---------|
| `af.content.download` | `{"serial","object_key"?,"content_id"?}` |
| `af.content.delete` | `{"serial","content_id"?}` |
| `af.content.ready` | `{"serial","content_id","device_path","status"}` |

Env orchestrator:

| Переменная | По умолчанию |
|------------|--------------|
| `CONTENT_MODE` | `stub` (или `http` для живого distributor) |
| `CONTENT_DISTRIBUTOR_HTTP_URL` | `http://127.0.0.1:19094` |
| `CONTENT_NATS_MODE` | включён ( `off` → sync HTTP download/delete) |
| `NATS_SUBJECT_CONTENT_DOWNLOAD` | `af.content.download` |
| `NATS_SUBJECT_CONTENT_DELETE` | `af.content.delete` |

Distributor: `HTTP_ADDR=:19094`, `STORE_MODE=postgres`, `STORAGE_MODE=minio`, `NATS_URL`.

## Contacts-manager (gRPC)

Контакты на телефонах: orchestrator → gRPC `:50055` → contacts-manager → executor gRPC `:50051`.

### Orchestrator HTTP (прокси к gRPC)

| Endpoint | gRPC RPC |
|----------|----------|
| `POST /phones/{serial}/contacts/upload` | `Upload` |
| `POST /phones/{serial}/contacts/sync` | `Sync` |
| `POST /phones/{serial}/contacts/merge` | `Merge` |
| `POST /phones/{serial}/contacts/groups` | `ApplyGroups` |
| `GET /phones/{serial}/contacts` | `ListContacts` |
| `GET /phones/{serial}/contacts/export` | `Export` |
| `DELETE /phones/{serial}/contacts/{contact_id}` | `DeleteContact` |

Env orchestrator:

| Переменная | По умолчанию |
|------------|--------------|
| `CONTACTS_MODE` | `stub` (или `grpc` для живого сервиса) |
| `CONTACTS_GRPC_ADDR` | `localhost:50055` |

Contacts-manager: `GRPC_ADDR=:50055`, `EXECUTOR_MODE=grpc`, `EXECUTOR_GRPC_ADDR=localhost:50051`, `STORE_MODE=postgres`.

## Video-generator (gRPC)

Генерация и обработка видео: orchestrator → gRPC `:50056` → video-generator → MinIO / FFmpeg / Ollama.

### Orchestrator HTTP (прокси к gRPC)

| Endpoint | gRPC RPC |
|----------|----------|
| `POST /phones/{serial}/video/screenshots` | `CreateFromScreenshots` |
| `POST /phones/{serial}/video/ai` | `GenerateAI` |
| `POST /phones/{serial}/video/edit` | `EditVideo` |
| `GET /phones/{serial}/video/jobs/{id}` | `GetJob` |
| `DELETE /phones/{serial}/video/jobs/{id}` | `DeleteVideo` |

Пример `POST /phones/{serial}/video/screenshots`:

```json
{
  "screenshot_keys": ["R5CY331L8NF/screen1.png", "R5CY331L8NF/screen2.png"],
  "audio_key": "library/audio/track.mp3",
  "overlay_text": "Reels",
  "profile": {"width": 1080, "height": 1920, "frame_sec": 2}
}
```

Пример `POST /phones/{serial}/video/ai`:

```json
{
  "prompt": "котики на закате",
  "duration_sec": 5
}
```

Env orchestrator:

| Переменная | По умолчанию |
|------------|--------------|
| `VIDEO_MODE` | `stub` (или `grpc` для живого сервиса) |
| `VIDEO_GRPC_ADDR` | `localhost:50056` |

Video-generator: `GRPC_ADDR=:50056`, `MINIO_BUCKET=af-videos`, `AI_MODE=ollama`, `QUEUE_MODE=nats|sync`.
