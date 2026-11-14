# Read-only Library Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans task by task. Maintain an execution ledger and small commits; final review after implementation.

**Goal:** Import the complete authorized NAS collection into Lookout's overview, enrich through official/media-server metadata, verify episode/season updates and Telegram connectivity.
**Architecture:** Native read-only SFTP, Go background workers, existing dedicated PostgreSQL, embedded bilingual Svelte UI. All NAS state remains unchanged.
**Spec:** [Design](../specs/2026-11-14-read-only-library-design.md).

## Global constraints

- WHOLE NAS read-only. No mutation methods, remote shell jobs or secret logging.
- Preserve all discovered media, unknown metadata and ambiguous anime numbering.
- Official metadata APIs; no reused application keys or personal data uploads.
- Optional Jellyfin and Plex metadata connections, independent SFTP inventory.
- Telegram validation without test sends. No notification for initial history.
- Latest stable dependencies, TypeScript6 approved exception, bounded containers/builds.
- Local LAN deployment, small commits. Signing timeouts do not stop implementation.

## Review focus

- Traversal/symlink and malicious sidecar inputs cannot escape roots or write remotely.
- Missing mounts/partial scans cannot erase the known inventory.
- Ambiguous translated titles and anime numbering remain visible and correctable.
- Release date changes, first baseline and restarts do not produce duplicate spam.
- Connections and artwork handling never reveal tokens or private SSH key content.

## Tasks

### 1. Native read-only scan and parsing
Files: `internal/library/{model,parse,nfo,scan,sftp}.go` and focused tests; update go.mod/go.sum.
Interfaces: Reader exposes ReadDir/Lstat/Open/Close only; Scanner emits FileObservation records with root/category/path/title-group and optional numbering/provider IDs. Connect uses SSH key and strict known-host checking, no remote shell.
- [ ] Write parsing/safety tests first; observe failures, implement and verify.
- [ ] Verify latest SSH/SFTP dependencies and use only read-only protocol operations.
- [ ] Run live complete scans into a local ignored evidence manifest; reconcile counts per root, inspect bounded sidecars. Commit.

### 2. Persistent catalogue and scan worker
Files: ordered migration runner, `migrations/002_catalogue.sql`, `internal/store/catalogue*.go`, `internal/library/service.go`, runtime configuration/main wiring.
Interfaces: Store persists sources, scan runs, titles, files and provider mappings; read API queries paginate. Background worker serializes source scans and retains failures/status. Runtime source config seeds user's four roots with credentials as local read-only secrets.
- [ ] RED→GREEN real PostgreSQL tests for idempotence, files/copies, partial scan preservation and totals.
- [ ] Add resumable jobs, source state, root confinement and bounded startup/scheduled scans. Commit.
- [ ] Deploy locally, perform complete imports and independently reconcile real counts.

### 3. Catalogue API and bilingual overview
Files: `internal/httpapi/catalogue*.go`, frontend catalogue/detail/source components and translations, OpenAPI/docs/browser tests.
Interfaces: `/api/v1/titles` pagination/search/category, title detail, `/api/v1/sources` status and scan actions. Safe structured errors; same-origin checks on writes; never expose secret fields.
- [ ] Tests first for pagination, unknown titles, request validation, source failures and secret exclusion.
- [ ] Replace foundation empty view with real overview, title detail and scan status; retain system checks.
- [ ] Verify complete real inventory visible via UI pagination and mobile/desktop screenshots. Commit.

### 4. Official metadata and review
Files: `internal/metadata/{tvmaze,tmdb,match,artwork}.go`, metadata store/API/UI, fixture tests.
Interfaces: provider-namespaced IDs, search candidates, selected match, cached descriptions/artwork and episode schedule. NFO/tag IDs precede search; ambiguous matches remain reviewable.
- [ ] Test ID lookup/search, alias ambiguity, failed provider requests, numbering and bounded artwork handling.
- [ ] Try live official metadata for real imported series/anime, without unauthorized credentials.
- [ ] Add metadata search/review/correction and source attribution; retain unmatched films. Commit.

### 5. Jellyfin and Plex connections
Files: media-server adapters/config encryption, connection API/UI and fixtures.
Interfaces: paginated read-only Jellyfin Items and Plex sections/items, provider IDs and explicit path mappings; encrypted tokens, cached image proxy and public configured-state responses.
- [ ] Tests for both documented APIs, paging, root/path mapping, secret exclusion and failures.
- [ ] Implement optional setup and import controls; test real supplied server if credentials are available. Commit.

### 6. Release following, seasons and episodes
Files: provider episode cache, follows, durable release observations/events/outbox, API/UI and deterministic tests.
Interfaces: baseline and clock-aware observations, unique work/provider episode event IDs, upcoming/reported-aired distinctions and new regular-season premiere events.
- [ ] RED→GREEN scenarios: baseline silence, future→aired, new regular season, specials, late/corrected dates, unknown dates, duplicate polls/restarts and unavailable provider.
- [ ] Verify real-series/anime schedules against official evidence and show upcoming/new release views. Commit.

### 7. Telegram and completion verification
Files: `internal/notify/telegram.go`, configuration/status/outbox integration and fixture tests; docs and live evidence.
Interfaces: read-only getMe/getChat/getChatMember verification, optional genuine release-event delivery via durable outbox with retry. No live sendMessage test.
- [ ] Validate supplied local bot/chat credentials without sending messages; persist safe status.
- [ ] Test delivery/retry/dedup using a local mock server only. Enable genuine notifications only if configured by user.
- [ ] Run full checks and fresh review; deploy final Compose and audit every goal requirement against real counts/runtime/browser/provider/Telegram evidence. Commit and preserve goal until all required items are proven.
