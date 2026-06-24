# CLAUDE.md

Руководство для AI-агентов в **AF-phone-orchestrator** — координатор сценариев Android Farm.

> Репозиторий: [Progery222/AF-phone-orchestrator](https://github.com/Progery222/AF-phone-orchestrator)

## Назначение

Связывает observer, recovery-engine, executor, connector. Не выполняет ADB, dump и LLM сам — только оркестрация.

## Стек

Go 1.22+, gRPC, NATS, HTTP health `:9090`, gRPC `:50050`.

## Архитектура

Hexagonal: `domain` → `port` → `service` → `adapter`.

## Ключевой поток

`RecoveryFlowService.RunRecovery`: observer screen+ui → NATS solve → executor (stub).

## GitHub Flow

Не пушить в `main` без PR. Conventional Commits.

## Связанные репозитории

- AF-phone-observer, AF-recovery-engine, AF-phone-connector, AF-phone-action-executor
