# Discovery Layout Implementation Plan

> **For agentic workers:** Execute inline with superpowers:executing-plans; one fresh final review if available.

**Goal:** Distinct upcoming cards, consistent responsive galleries with saved width cap, and local-only type-ahead.
**Architecture:** Existing scheduled-release DTO→new date-led component; App-owned browser preference/shared gallery CSS; bounded local suggestion API/store filter→standalone combobox.
**Tech Stack:** Existing Go/PostgreSQL/Svelte/TS6, no new dependencies.
**Spec:** ../specs/2026-11-15-discovery-layout-design.md

## Constraints / review focus

No external search or credentials in client. Exclude titles with only missing/unconfirmed file evidence. Avoid stale-result races and keyboard traps. Upcoming dates do not imply a local copy or a released season; date-only never invents a time. Width preference belongs solely to this browser, survives navigation/reload, and does not break existing resize behavior. Artwork provenance remains series-level and proxied/cached.

- [x] RED→GREEN store/API regressions for observed local suggestions, query validation and bounded output; commit.
- [x] RED→GREEN browser regressions for date-led upcoming/premiere cards, keyboard/debounced local suggestions and failure/stale handling, shared width/reload/browser isolation and ultrawide resize.
- [x] Implement focused upcoming/search components and shared browser width setting; full checks and final review; fix meaningful issues.
- [x] Deploy tested image, inspect actual desktop/phone/ultrawide screenshots, refresh README/docs and commit locally.
