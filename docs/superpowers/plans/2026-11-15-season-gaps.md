# Missing Seasons Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline; one fresh final reviewer.

**Goal:** See which aired seasons are absent or partial in the local library.
**Architecture:** Pure comparison plus read-consistent aggregated PostgreSQL query, paginated read-only API and Releases component. Existing cache and source inventory; no schema or dependency additions.
**Tech Stack:** Existing Go/PostgreSQL/Svelte/TypeScript6.
**Spec:** ../specs/2026-11-15-season-gaps-design.md

## Constraints and review focus

Keep NAS read-only and Telegram unchanged. Include historical aired seasons; exclude future/unknown dates and specials. Deduplicate copies/multi-episode numbering. Unknown numbering, metadata mismatch, source failures and anime ordering must not produce definitive gaps. Provider identity changes must exclude old schedules. Search, filter and pagination totals must match results; mobile must fit without horizontal scrolling.

### 1. Comparison and persistence boundary
- [ ] Add failing controlled-time comparison and isolated-PostgreSQL tests.
- [ ] Implement internal/releases/gaps.go and internal/store/season_gaps.go with exact numbered-set comparison and review reasons.
- [ ] Run domain and real database tests; commit.

### 2. API and interface
- [ ] Add failing API and desktop/mobile browser tests for missing, partial and uncertain results.
- [ ] Implement paginated /api/v1/releases/gaps?kind=missing|partial|review with q/category/limit/offset; add bilingual component above existing Releases sections.
- [ ] Verify filters, empty/error states, detail links and mobile layout; commit.

### 3. Deployment and review
- [ ] Full make check and fresh focused reviewer; fix important findings with regressions.
- [ ] Deploy tested image to existing LAN service, audit API against local read-only database, refresh screenshots if changed and update docs. Keep local commits; no push.
