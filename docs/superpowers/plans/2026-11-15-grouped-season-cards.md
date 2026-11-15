# Grouped Season Cards Implementation Plan

> **For agentic workers:** Execute inline with superpowers:executing-plans and a fresh final read-only review.

**Goal:** Show one series card per tab with selectable seasons, and auto-fit cards to available width.
**Architecture:** Opt-in grouped store/API output before pagination; selected-season detail inside existing card; fluid CSS Grid and wider Releases container.
**Tech Stack:** Existing Go/PostgreSQL/Svelte/TS6, no new dependencies.
**Spec:** ../specs/2026-11-15-grouped-season-cards-design.md

## Constraints / review focus

Do not group merely by display name or split seasons across pages. Preserve flat API compatibility and original per-season counts; grouped pagination counts titles. Selected coverage/episode numbers must belong to the labeled season, and selection must recover when polling removes that season. Review states stay uncertain. Verify one/two/three-column layouts by resizing the same window; long names and phone widths must fit.

- [ ] Add failing store/API tests for grouped paging, ordering, totals, compatibility and invalid grouping.
- [ ] Add failing browser tests for one grouped card, selecting another season, polling selection recovery and dynamic resize.
- [ ] Implement grouped output and responsive selectable-season cards; verify relevant checks/full suite and commit small changes.
- [ ] Fresh review, deploy tested image, inspect live grouped cards and different widths, update docs/screenshots and commit locally.
