# AGENTS.md

Инструкции для агентов в **AF-phone-orchestrator**.

- Координация, не ADB/dump/LLM.
- Hexagonal: service без импортов NATS/HTTP/gRPC.
- Recovery через NATS (`af.recovery.*`), observer через HTTP REST.
- Executor пока stub.
- Сообщения API на русском.
- Не пушить в `main` без PR.

Подробности: `CLAUDE.md`, `INTEGRATIONS.md`.
