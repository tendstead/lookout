# Consistent Season Lists Implementation Plan

> **For agentic workers:** Execute inline using superpowers:executing-plans, with a fresh final read-only review.

**Goal:** Same compact season-list presentation for one or many missing/partial seasons.
**Architecture:** Existing SeasonGapCard only, preserving API/comparison/grouping/selection.
**Spec:** ../specs/2026-11-15-consistent-season-lists-design.md

- [ ] Observe failing browser assertions for a single missing/partial season selector and accessible selected state.
- [ ] Remove header number/count branches; render numeric missing/partial chips consistently like local chips; preserve keyboard labels and coverage context.
- [ ] Run formatting/types/build/browser/lifecycle checks, review, deploy and visually inspect actual cards. Refresh README screenshot and document current server limitation; commit locally.
