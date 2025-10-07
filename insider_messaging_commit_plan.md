# Insider Messaging Project – Commit-by-Commit Plan (Go)

> Goal: implement the Insider Assessment Messaging service in Go with clean architecture, docs, tests, and deploy artifacts. Each step lists an atomic commit you can push as a PR.

## Conventions
- **Branching**: `main` (protected), feature branches `feat/<slug>`.
- **Commit style**: Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `ci:`).
- **Go version**: 1.22+
- **Module**: `github.com/<org>/insider-messaging`
- **Packages**: `cmd/server`, `internal/{api,scheduler,service,repo,integration,domain}`, `pkg/{config,logger,metrics}`.

---

## Milestone A — Bootstrap & Tooling

1. **chore(repo): init go module & basic layout**
   - `go mod init ...`
   - dirs: `cmd/server`, `internal/...`, `pkg/...`
   - `.editorconfig`, `.gitignore`
   - `README.md` (skeleton)

2. **chore(tooling): add makefile, lint, vet, testing targets**
   - `Makefile` with `build`, `run`, `lint`, `test`, `cover`.
   - `golangci.yml` + `golangci-lint` config.
   - `go test ./...` smoke (empty packages).

3. **feat(config): wire env config & DI**
   - `pkg/config`: DB URL, Redis URL, WEBHOOK URL, INTERVAL(default 2m), BATCH(default 2), AUTOSTART.
   - `cmd/server/main.go`: read config, init logger.

4. **feat(api): minimal HTTP server + health route + swagger scaffolding**
   - Router (Gin/Fiber/chi).
   - `/healthz` returns 200.
   - Add `swaggo/swag` annotations & `docs/` stub.
   - `make swagger` target.

5. **ci: setup github actions (lint + unit tests)**
   - `.github/workflows/ci.yml`: lint, build, test, coverage artifact.

---

## Milestone B — Data Layer & Schema

6. **feat(db): add migrations & connect to Postgres**
   - Tool: `golang-migrate` (local), migration files: `messages` table (id uuid, phone_number, content, status, created_at, sent_at).
   - `internal/repo/sql/` SQL files for queries.
   - DB connection pool with context timeout.

7. **feat(repo): message repository (select/mark)**
   - `SelectUnsentForUpdate(limit)`: `SELECT ... FOR UPDATE SKIP LOCKED LIMIT $1`.
   - `MarkSent(id, sentAt, messageId?)`.
   - Unit tests with `sqlmock` or `dockertest/testcontainers` (integration later).

8. **feat(redis): optional cache repo**
   - `SetSentMeta(id, messageId, sentAt, ttl)` and `GetSentMeta(id)`.
   - Guard app to work if Redis disabled.

---

## Milestone C — Webhook & Domain Service

9. **feat(integration): webhook client**
   - `internal/integration/webhook_client.go`: POST JSON `{to, content}` with headers.
   - Treat **202** as success, parse `messageId`; 4xx non-retryable, 5xx retryable.
   - Timeouts, simple retry (will upgrade later).

10. **feat(service): send batch use-case**
    - `internal/service/sender.go`: `SendBatch(ctx, limit)`.
    - Flow: fetch locked → in parallel send → if success mark sent (+ cache) → return counts.
    - Validation: content length, phone format.

11. **test(unit): sender & webhook client**
    - Table-driven tests for success, 4xx, 5xx with backoff.

---

## Milestone D — Scheduler & APIs

12. **feat(scheduler): ticker engine with start/stop**
    - `internal/scheduler/engine.go`: `Start(ctx)`, `Stop()`, `2m ticker`, batch=2 configurable.
    - Graceful stop with context cancellation.
    - Metrics: gauge `scheduler_running`.

13. **feat(api): /scheduler/start & /scheduler/stop**
    - Handlers call engine start/stop, return `{running, interval, batch}`.
    - Swagger docs for endpoints.

14. **feat(api): /messages/sent (list with pagination)**
    - Query DB, optionally hydrate `messageId` from Redis.
    - Swagger models + examples.

15. **docs(swagger): fill request/response models & errors**
    - Add enums for `status`, examples for webhook 202 body.
    - Host `/swagger/index.html`.

---

## Milestone E — Containers & Local Stack

16. **chore(docker): Dockerfile & docker-compose (app+pg+redis)**
    - Multi-stage build, minimal runtime image.
    - Compose env: seeds for demo data; networks.

17. **docs(readme): quickstart & swagger url**
    - `docker-compose up -d`
    - Visit `/swagger/` and `/healthz`.

18. **docs(postman): add collection JSON**
    - Commit `Insider_Messaging_Postman_Collection.json` to `/postman/`.
    - README section on importing.

---

## Milestone F — Integration Tests & Reliability

19. **test(integration): e2e with testcontainers (pg+redis+httptest webhook)**
    - Scenarios: happy path, concurrency (two schedulers), retry 5xx, 4xx no retry, new rows between ticks, graceful shutdown.

20. **feat(retry): exponential backoff + jitter**
    - Extract backoff policy, add unit tests.
    - Configurable max attempts.

21. **feat(observability): structured logs & prometheus metrics**
    - Counters: `sends_total`, `retries_total`.
    - Histograms: `send_duration_seconds`.
    - `/metrics` endpoint.

22. **ci: add integration job & coverage thresholds**
    - Run e2e in Actions using services/testcontainers.
    - Enforce coverage ≥70% overall; badges in README.

---

## Milestone G — Load & Polish

23. **perf(k6): baseline & spike scripts**
    - `k6/baseline.js`, `k6/spike.js` (mock webhook / or target app).
    - Make targets `k6-baseline`, `k6-spike`.

24. **feat(config): runtime tunables**
    - Env for `BATCH_SIZE`, `INTERVAL`, `MAX_RETRIES`, `BACKOFF_MIN/MAX`, `REDIS_TTL`.

25. **docs(test-plan): add test plan & diagrams**
    - Commit `docs/insider_messaging_test_plan.md` and Mermaid diagrams.
    - Link to canvas sequence & C4 diagrams.

26. **docs(runbooks): ops guide & troubleshooting**
    - Common failures, retry policy, DB locks, how to drain/stop safely.

27. **release(v0.1.0): tag MVP**
    - App runs end-to-end with swagger, docker-compose, tests green.

28. **release(v0.2.0): tag production-ready**
    - Backoff, metrics, integration tests, k6 baseline in place.

---

## Example PR/Commit Messages

- `chore(repo): scaffold module, layout and makefile`
- `feat(config): env-driven configuration and DI wiring`
- `feat(api): health endpoint and swagger scaffolding`
- `feat(db): migrations for messages table and pg connection`
- `feat(repo): select unsent with FOR UPDATE SKIP LOCKED + mark sent`
- `feat(integration): webhook client with 202 handling`
- `feat(service): send batch flow with validation`
- `feat(scheduler): 2m ticker and start/stop control`
- `feat(api): endpoints /scheduler/start /scheduler/stop /messages/sent`
- `chore(docker): Dockerfile and docker-compose for app+pg+redis`
- `test(integration): e2e happy path + concurrency no-duplicates`
- `feat(retry): exponential backoff + jitter`
- `feat(observability): structured logs & prometheus metrics`
- `perf(k6): baseline and spike scenarios`
- `docs: test plan and architecture diagrams`
- `release: v0.1.0`
- `release: v0.2.0`

---

## File/Directory Snapshot (after v0.2.0)

```
.
├── cmd/server/main.go
├── internal/
│   ├── api/handlers.go
│   ├── scheduler/engine.go
│   ├── service/sender.go
│   ├── repo/{message_repo.go,cache_repo.go}
│   └── integration/webhook_client.go
├── pkg/{config,logger,metrics}/...
├── migrations/0001_messages.sql
├── docs/{swagger, test_plan.md, diagrams/*.md}
├── postman/Insider_Messaging_Postman_Collection.json
├── k6/{baseline.js, spike.js}
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── .github/workflows/ci.yml
```

---

## Definition of Done (per milestone)
- All unit & integration tests green (`-race`).
- Swagger routes available; example requests work.
- Docker Compose up with seeded demo data.
- No duplicates under two concurrent schedulers.
- Coverage thresholds met; CI green.
- README updated with Quickstart & Troubleshooting.