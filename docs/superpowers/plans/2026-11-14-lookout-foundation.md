# Lookout Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Status:** Implemented locally on 2026-11-14. `make check` passed, including eight desktop/mobile browser cases and disposable database lifecycle checks. LAN Compose startup and two README screenshots were additionally authorized and verified. No remote repository or publication.

**Goal:** Produce a locally runnable, tested Lookout foundation with an embedded bilingual UI, dedicated PostgreSQL persistence, documented health endpoints and a reproducible container build.

**Architecture:** One Go process serves static Svelte 5/TypeScript assets and a versioned HTTP API. An external PostgreSQL server supplies a dedicated database and role; migrations and a small connection pool belong to Lookout. This milestone establishes the runtime for later catalogue/scanning work, without pretending that those features exist.

**Tech Stack:** Go, pgx/v5, Svelte 5/TypeScript with static output, `@tendstead/ui`, Docker Compose, Go tests, Vitest and Playwright. Use shopping-list's inspected tool versions as the starting compatibility baseline; verify availability before pinning dependency lockfiles.

**Spec:** [Lookout design](../specs/2026-11-14-lookout-design.md).

## Global Constraints

- Product: Tendstead Lookout. Module path: `github.com/tendstead/lookout`; a remote repository is unnecessary for local builds.
- Application license: Elastic License 2.0 (`Elastic-2.0`); retain dependency notices separately.
- Use the existing external PostgreSQL server with Lookout's own database, role and migrations. Do not add a production database container or SQLite backend.
- No Redis, message broker, separate scheduler or LLM runtime.
- Mobile-first shared UI, purple tokens, outline icons and reactive English/German labels.
- Local-only work. No deployment, publishing, real media scans or sibling-repository edits.
- Build tools run in bounded Docker containers. PostgreSQL integration tests use disposable resources only.
- Preserve Git signing settings. Never bypass signing to obtain a commit.
- Foundation-only scope: no catalogue matching, external-provider calls, subscriptions, downloader execution or notifications yet.

## Review Focus

- Missing or malformed database configuration fails without logging credentials (Task 1).
- A database outage changes readiness but leaves process liveness available (Tasks 1–2).
- Concurrent startup and failed migrations do not leave a partly migrated schema (Task 2).
- Unknown API routes return JSON errors; SPA navigation never hides missing API endpoints (Task 3).
- A narrow screen or language change keeps navigation usable and labels reactive (Task 4).

## File Structure

- `cmd/server/main.go`, `assets.go`, `static/`: runtime wiring, embedded frontend and graceful shutdown.
- `internal/config/`: environment parsing and secret-file support.
- `internal/store/`: bounded PostgreSQL pool, migrations and readiness checks.
- `internal/httpapi/`: routes, JSON errors and health responses.
- `web/`: static Svelte frontend, translations and browser tests.
- `docs/api.md`, `docs/development.md`, `docs/deployment.md`: API and operational contracts.
- `Dockerfile`, `compose.yaml`, `compose.tools.yaml`, `compose.test.yaml`, `Makefile`, `.env.example`, `.gitignore`, `.dockerignore`: repeatable builds and isolated checks.

### Task 1: Runtime configuration and HTTP health contract

**Files:** Create `go.mod`, `go.sum`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/httpapi/api.go`, `internal/httpapi/api_test.go`, `compose.tools.yaml`, `.env.example`, `.gitignore`, `Makefile`.

**Interfaces:**
- `config.Load(lookup func(string) (string, bool), readFile func(string) ([]byte, error)) (Config, error)`.
- `Config`: `ListenAddr string`, `DatabaseURL string`, `DatabasePasswordFile string`; database configuration is consumed only by the store, never returned by the API.
- `httpapi.NewHandler(checkReady func(context.Context) error) http.Handler`.
- `GET /healthz`: HTTP 200, `{"status":"ok"}`. `GET /readyz`: HTTP 200, `{"status":"ready"}`, or HTTP 503, `{"status":"unavailable"}`. Both use JSON and `Cache-Control: no-store`.

- [x] Write `TestLoadConfig` cases: default listen address `127.0.0.1:8080`, explicit override, missing `DATABASE_URL`, malformed URL, password-file errors, and contradictory URL-password/file-password settings. Assert errors do not contain supplied passwords. Support PostgreSQL URLs only; keep credentials out of stringified diagnostics.
- [x] Write handler tests for health/readiness, cancelled readiness context and unsupported methods. Readiness receives a two-second timeout; errors return the generic unavailable body.
- [x] Run the tests in a bounded Go tools container. Confirm failures identify missing implementation before implementing the functions above.
- [x] Implement configuration and health routing. Require a configured database URL; support `DB_PASSWORD_FILE` for deployments that omit the URL password. Do not log the URL.
- [x] Run `make test` and `make fmt-check`; expect passing Go tests and no formatting diff. Record the commands in development documentation as it is added.
- [x] Commit this task with the existing signing configuration; if signing is unavailable, retain changes and report that limitation without disabling signing.

### Task 2: Dedicated PostgreSQL schema and startup lifecycle

**Files:** Create `internal/store/postgres.go`, `migrations.go`, `migrations/001_initial.sql`, `postgres_test.go`, `cmd/server/main.go`, `compose.test.yaml`, `scripts/test-service.sh`.

**Interfaces:**
- `store.Open(ctx context.Context, cfg config.Config) (*Store, error)` establishes a pool with at most four connections.
- `(*Store).Migrate(ctx context.Context) error`, `(*Store).Ping(ctx context.Context) error`, `(*Store).Close()`.
- The initial migration creates only Lookout's schema-version bookkeeping. Future catalogue tables belong to a later plan.

- [x] Write service tests `TestMigrateTwice`, `TestConcurrentMigrate`, `TestFailedMigrationRollsBack`, `TestPoolBound`, and `TestReadinessAfterDatabaseLoss`. Use a unique Compose project and a disposable PostgreSQL database; no real installation URL may be inherited.
- [x] Run `make service-test`; verify failures correspond to the missing store behavior.
- [x] Implement pgx pooling, transactional migrations with a transaction-level advisory lock, and schema-version checks. Reject a schema newer than this executable supports. Apply migrations before serving normal requests; database failure exits startup cleanly without credential output.
- [x] Wire the server with bounded HTTP timeouts, readiness via `Store.Ping`, signal cancellation and a ten-second graceful shutdown deadline. Listen on the configured address.
- [x] Verify migration tests and process tests: unavailable DB causes a nonzero startup exit; SIGTERM exits cleanly; post-start DB outage yields readiness 503 and liveness 200.
- [x] Commit the tested store/runtime changes.

### Task 3: Static frontend and API boundary

**Files:** Create `cmd/server/assets.go`, `internal/httpapi/static.go`, `static_test.go`, `web/package.json`, `web/package-lock.json`, `web/svelte.config.*`, `web/vite.config.ts`, `web/tsconfig.json`, `web/src/app.html`, `web/src/routes/+layout.ts`, `Dockerfile`, `.dockerignore`, `docs/api.md`.

**Interfaces:**
- Extend `httpapi.NewHandler` to `NewHandler(checkReady func(context.Context) error, assets fs.FS) http.Handler` and update Task 1 callers/tests together.
- `GET /api/v1/status`: `{"service":"lookout","stage":"foundation"}`; no credentials, paths or connection details.
- Unknown `/api/` routes: HTTP 404 JSON `{"error":{"code":"not_found","message":"Not found"}}`. Unsupported API methods: HTTP 405. No API response is substituted with HTML.

- [x] Add tests for missing API endpoints, unsupported methods, missing static files, client navigation, traversal attempts and cache policy. Unknown asset filenames return 404; extensionless UI navigation may serve the SPA entry point. Entry HTML must revalidate; content-hashed assets may be immutable.
- [x] Run the Go route tests and confirm they fail for the absent static/API implementation.
- [x] Configure static Svelte output, embed it in the Go executable, implement the status endpoint and routing precedence, and build a multi-stage nonroot image. Node is used only during builds/tests.
- [x] Add the documented endpoint schemas and response codes to `docs/api.md`; include an initial `docs/openapi.yaml` for these endpoints. Authentication for future administrative writes remains a later API-design task; this milestone exposes no writes.
- [x] Run Go tests and a production image smoke test: health/status work, UI HTML loads, unknown API returns JSON. The image must start without a Node runtime.
- [x] Commit the embedded frontend/API boundary.

### Task 4: Bilingual shared UI shell

**Files:** Create `web/src/routes/+layout.svelte`, `+page.svelte`, `web/src/lib/i18n.ts`, `i18n.test.ts`, `web/tests/foundation.spec.ts`, `web/playwright.config.ts`.

**Interfaces:**
- Use the installed `@tendstead/ui` package's public AppShell and theme APIs; inspect its exports/documentation before coding rather than inventing props.
- `locale` is a persisted `en | de` preference. `t(key)` reacts to its changes; translation dictionaries have identical key sets.
- Initial view states clearly that this is the foundation and no sources are configured. No fake media, fake integrations or active controls for unavailable features.

- [x] Add translation parity/unit tests and Playwright cases at 390px and 1280px widths. Assert no horizontal overflow, accessible navigation and reactive English/German labels without reloading.
- [x] Run unit/browser tests to establish the expected missing-UI failures.
- [x] Implement the shared shell and minimal landing/status view with purple tokens and outline icons. Display API failure as an unavailable status; keep navigation rendered. Persist language/theme preferences locally.
- [x] Run frontend format/lint/type checks, unit tests and mobile/desktop browser tests against the production build. Inspect screenshots for clipping and contrast.
- [x] Commit the validated shell.

### Task 5: Local operation, checks and resource observations

**Files:** Create `compose.yaml`, `README.md`, `docs/development.md`, `docs/deployment.md`, `docs/resource-baseline.md`; extend `Makefile` and `.env.example`.

**Interfaces:**
- `make check`: format/lint/types, Go vet, Go/frontend tests, isolated service tests, browser checks and production build.
- `make up` starts only Lookout against an explicitly configured external database. Tests alone may start disposable PostgreSQL.
- Runtime binds to loopback by default; no remote or release workflow is required to start locally.

- [x] Add production Compose configuration for one nonroot app, read-only root filesystem, `/tmp` tmpfs, dropped capabilities, no-new-privileges and a healthcheck. Add explicit CPU/memory budgets to build/test/app containers; record initial app budgets as provisional, not measured requirements.
- [x] Document dedicated database/role provisioning without executing it against the user's server. Explain `DATABASE_URL`, password-file configuration, TLS configuration and trusted-network access. Keep user setup in README and engineering commands in `docs/development.md`.
- [x] Run `make check` and a fresh local setup smoke test against disposable resources. Do not add tests that merely assert configuration text; verify effective container settings during the smoke test.
- [x] Observe idle RSS, startup peak and connection count on the disposable installation. Record image/commit, limits and commands in `docs/resource-baseline.md`. Mark scan/discovery/download peaks unmeasured until those features exist.
- [x] Record provider coverage/access checks, real-file recognition diagnosis and canonical identity/source contracts as prerequisites for the next catalogue milestone. No provider accounts, NAS scans or downloads are initiated here.
- [x] Review `git diff --check`, tracked files, dependency licenses and the complete changed behavior; ensure secrets/generated artifacts are absent. Commit verified work and report remaining scope honestly. No push or deployment.

## Following Milestones

Each gets its own implementation plan against the full design: canonical identities and source-health contracts; local/SFTP inventory with absence safeguards and match review; direct provider adapters and cached catalogue; profile follows and durable Telegram outbox; public-source discovery; optional Gather integration after its API contracts are verified. ARD automatic future-season discovery remains an open scope decision and does not block this foundation.

## Self-Review

The full proposal is preserved beside this plan. This first plan covers the shared architecture/runtime, local development, health API, shell, EL2 declaration and baseline measurements. Catalogue features are explicitly assigned to subsequent milestones. Review-focus failure modes have named tests in their owning tasks. Interface changes in Task 3 explicitly include updating earlier callers; no existing sibling source is changed or reused.
