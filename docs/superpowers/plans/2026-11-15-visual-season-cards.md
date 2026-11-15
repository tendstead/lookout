# Visual Season Cards Implementation Plan

> **For agentic workers:** Execute inline with superpowers:executing-plans, followed by one fresh read-only review.

**Goal:** Make each missing/partial season visually scannable like the overview.
**Architecture:** Poster-led Svelte component and responsive grid; existing data/API/artwork proxy.
**Tech Stack:** Existing Svelte5/TS6; no new dependencies.
**Spec:** ../specs/2026-11-15-visual-season-cards-design.md

## Constraints / review focus

Never imply local season chips are complete collections. Do not infer present episode identities. Do not label series artwork as season-specific. Review items have no coverage bar. Respect dark/light themes, mobile width, missing artwork, translations and reduced motion (no animations). Keep ignored credentials and source/notification configuration untouched.

- [ ] Add failing browser behavior checks for poster/title identity, accessible 0/10 and3/10 coverage, local-season chips, expandable missing-number tiles, review states and image fallback.
- [ ] Extract typed SeasonGapCard; implement visual treatments and responsive grid; preserve search/pagination. Run types/browser/build/lifecycle checks and commit.
- [ ] Fresh final review; fix meaningful findings. Deploy tested image, inspect actual phone and desktop cards, refresh README screenshot/docs, commit locally.
