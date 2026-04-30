# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

**CMK (Customer-Managed-Keys)** is a Go microservice for the Key Management Service layer of OpenKCM. It manages cryptographic keys, certificates, workflows, tenants, and async tasks across multiple components.

Module: `github.com/openkcm/cmk`
Go: 1.25.6 (toolchain 1.26.1)

## Build & Development Commands

```bash
# Run unit tests (uses gotestsum, clears cache, produces coverage)
make test

# Run integration tests (spins up containers for PostgreSQL, Redis, RabbitMQ)
make integration_test

# Run linter (golangci-lint v2 with --fix)
make lint

# Format all Go files (gofmt + goimports + golines + gci)
make go-imports

# Format only changed files
make go-imports-changed

# Generate OpenAPI server code from spec
make codegen api=cmk

# Run the API server locally
make run

# Run benchmarks
make benchmark

# Full local K3d cluster setup (PostgreSQL, Redis, RabbitMQ, OTEL, CMK)
make start-cmk

# Build dev Docker image
make docker-dev-build

# Generate signing keys for local testing
make generate-signing-keys

# Create empty secret files from blueprints
make create-empty-secrets

# Build tenant-manager CLI locally
make build-tenant-cli

# Provision test tenants locally
make provision-tenants-locally

# Tidy Go modules
make tidy
```

### Running a single test

```bash
go test -run TestFunctionName ./internal/path/to/package/...
```

## Architecture

### Service Components (cmd/)

| Binary | Purpose |
|--------|---------|
| `api-server` | HTTP REST API server — the main service entry point |
| `task-scheduler` | Cron-based periodic task scheduling (Redis/Asynq) |
| `task-worker` | Async task processor (cert rotation, system refresh, HYOK sync, etc.) |
| `event-reconciler` | Processes events from AMQP message brokers via Orbital |
| `tenant-manager` | Tenant lifecycle management (listens to AMQP for tenant events) |
| `db-migrator` | Database schema and data migrations (Goose) |
| `tenant-manager-cli` | CLI for tenant CRUD operations |
| `task-cli` | CLI for async task management |

### Internal Packages (internal/)

| Package | Purpose |
|---------|---------|
| `api/` | Generated OpenAPI server code (from `apis/cmk/cmk-ui.yaml`) |
| `apierrors/` | Error constants and error-to-HTTP-status mappings per operation |
| `async/` | Async task framework (Asynq-based) with task handlers |
| `auditor/` | Audit logging to OTLP endpoint |
| `authz/` | RBAC authorization — policies for API endpoints and repos |
| `clients/` | gRPC clients for external services |
| `config/` | Configuration loading (YAML + env overrides) |
| `controllers/` | HTTP request handlers mapped to OpenAPI operations |
| `daemon/` | Server lifecycle, HTTP server setup, graceful shutdown |
| `db/` | Database connection, multitenancy setup, read replicas |
| `errs/` | Error types and mapping logic |
| `event-processor/` | Event factory, handlers, and reconciliation pipeline |
| `handlers/` | HTTP error response handlers |
| `log/` | Context-based logging via slogctx |
| `manager/` | Business logic managers (Key, Certificate, Workflow, System, Tenant, User, Group, Pool, etc.) |
| `middleware/` | HTTP middleware chain: tracing, request ID, multitenancy, panic recovery, logging, OAPI validation, client data, authz |
| `model/` | Data models and domain types |
| `notifier/` | Notification service client |
| `operator/` | Operator management |
| `pluginregistry/` | Plugin catalog — loads/manages keystore, cert-issuer, notification, identity plugins |
| `plugins/` | Plugin implementations |
| `repo/` | Data access layer (repository pattern over GORM) |
| `testutils/` | Test helpers: `NewTestDB`, `NewAPIServer`, `MakeHTTPRequest`, model factories |
| `workflow/` | Workflow state machine (FSM) for approval-based operations |

### Key Infrastructure

- **Database**: PostgreSQL with GORM, multitenancy via per-tenant schemas (`bartventer/gorm-multitenancy`), read replicas via `dbresolver`
- **Task Queue**: Redis-backed via Asynq — scheduler enqueues, worker processes
- **Events**: AMQP via Orbital for event distribution
- **Plugins**: Go plugin architecture via `openkcm/plugin-sdk` (keystores, cert issuers, notifications, identity management)
- **Observability**: OpenTelemetry (traces, metrics, logs), Prometheus metrics
- **Auth**: Signed client data headers (RSA), JWT validation, RBAC policy engine
- **API**: OpenAPI 3.0 spec → `oapi-codegen` generated server with validation middleware

## Database Migrations

Located in `migrations/` with shared (public schema) and tenant (per-tenant schema) directories. Each has:
- `schema/` — SQL migrations (run first, block cluster startup)
- `data/` — Go-based migrations using Goose `AddMigrationContext` (run in parallel, non-blocking)

Destructive changes use the **Expand & Contract** pattern across two releases.

## Error Mapping Pattern

Errors are mapped per operation in `internal/apierrors/`. Each `ErrorMap` pairs internal error(s) with an HTTP status and error code. The system matches by most specific error chain. To add a new error:
1. Define error constant in `apierrors`
2. Add `ErrorMap` entry with matching error(s) and `DetailedError` (code, message, status)

## Testing

- **Unit tests**: Use helpers from `internal/testutils/` — `NewTestDB`, `NewAPIServer`, `MakeHTTPRequest`, model factories (`New<ModelType>`)
- **Integration tests**: In `test/integration/` using testcontainers (PostgreSQL, Redis, RabbitMQ)
- **DB migration tests**: In `test/db-migration/`
- **Security tests**: In `test/security/`

## Conventions

- **Commits**: Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `build:`). Release-please automates versioning.
- **Import ordering** (gci): standard → default → `prefix(github.com/openkcm/cmk)` → blank → dot → alias → localmodule
- **Linting**: golangci-lint v2 with `default: all` minus disabled linters. JSON tags use `goCamel` case. Exhaustive switch requires `default` to count as exhaustive.
- **Logging**: Context-based via slogctx. Static info via values.yaml labels. Dynamic info injected into logger context. PII fields are masked.
- **Configuration**: `config.yaml` with environment variable overrides. Secrets loaded from files in `env/secret/`.
