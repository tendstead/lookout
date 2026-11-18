# Multi-server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Unified multi-server availability, editable servers, and Wanted arrivals.
**Architecture:** Preserve source titles as evidence, add logical servers and a reversible canonical-title projection. Extend media adapters to inventory episodes independently; use existing bounded notification delivery.
**Tech Stack:** Existing Go/PostgreSQL/Svelte/TS6; no added dependencies.
**Spec:** docs/superpowers/specs/2026-11-15-multi-server-design.md

## Global Constraints

- Six-hour scan and 24-hour stale defaults, configurable per server.
- Read-only remote access; preserve current keys, data, metadata and follows.
- No real Telegram test sends; retain the delivery switch.
- No dependencies added; retain existing licenses and resource bounds.
- No push, merge, publication or purchases.

## Review Focus

- Same path on different hosts never collides; failed scans never establish absence.
- Shared provider IDs with contradictory identities stay separate; merge is reversible.
- API-only series require real episode observations, not assumed whole seasons.
- Stale unrelated servers and sparse identities cannot create confident absence.
- Wanted arrivals are deduplicated across scans/restarts and name the correct server.

### Task 1: Server model and multi-host scanning
**Files:** migration 006, internal/store/servers.go, internal/catalogue/service.go, internal/library/model.go; configuration and store tests.
**Interfaces:** Server records with ID, name, scan/stale hours; roots reference ServerID. Legacy config normalizes to one server. Store.ListServers/SaveServer and due-source scheduling.
- [x] Add failing tests for legacy config, two hosts with identical paths, settings validation and preserved names.
- [x] Run tests red, implement additive migration/config normalization and independent bounded scans, verify green.
- [x] Commit server foundation.

### Task 2: Unified identity and availability
**Files:** internal/store/identity.go, catalogue_read.go, season_gaps.go, metadata.go; store tests.
**Interfaces:** Canonical identity projection preserves original IDs; server-filtered TitleQuery; Title.Servers and detail server/file evidence; explicit identity review.
- [x] Test duplicate IDs, conflicting IDs, remakes, reversible manual merge, combined season coverage and stale absence.
- [x] Implement projection/read paths and selected-server season comparison; run store tests green.
- [x] Commit catalogue availability.

### Task 3: Independent Jellyfin/Plex inventory
**Files:** internal/metadata/media_server.go, internal/enrichment/inventory.go, store/connections.go, cmd/server/main.go; adapter/enrichment tests.
**Interfaces:** MediaServer.ServerID, inventory items with season/episode evidence; independent scheduled imports and explicit server linking.
- [x] Test API-only movies/episodes, pagination failures, mapped same-storage copies and no shell/write requests.
- [x] Implement read-only API inventory with successful-scan publication, linked-path reuse and automatic schedules; verify fixtures.
- [x] Commit API inventory.

### Task 4: Wanted and durable notifications
**Files:** migration 007, store/wanted.go, notify/service.go, httpapi/multi_server.go; store/delivery tests.
**Interfaces:** Wanted CRUD, target matching, per-server arrival events and existing Telegram authorization gate.
- [x] Test existing-item baseline, subsequent server/episode arrivals, failures/restarts/deduplication and server name in mocked Telegram text.
- [x] Implement Wanted state, transactional arrival outbox, API validation and delivery; verify green.
- [x] Commit Wanted.

### Task 5: Server and Wanted UI
**Files:** web/src/lib/Servers.svelte, Wanted.svelte, Library.svelte, Connections.svelte, MissingSeasons.svelte, App.svelte, i18n.ts; browser tests.
**Interfaces:** Existing APIs plus servers/wanted endpoints; canonical title cards with badges and server filters.
- [x] Add browser cases for rename/settings, badges/filter, per-server evidence, Wanted creation and narrow screens.
- [x] Implement accessible English/German controls and verify types/build/browser tests.
- [x] Commit UI.

### Task 6: Review, migration rehearsal and local deployment
**Files:** docs/library.md, api.md, openapi.yaml, README screenshots and verification notes.
- [x] Run full make check; review whole change with a fresh reviewer and fix consequential findings.
- [x] Back up dedicated Lookout database, rehearse migration against copied data, then deploy locally.
- [x] Verify existing inventory/follows, LAN UI, server freshness and disabled Telegram delivery; capture screenshots and resource observations.
- [x] Update documentation and commit results. Report remaining limitations honestly.
