# Missing seasons

User approved adding the previously described missing-season comparison: latest aired season versus local seasons, absent versus partial seasons, uncertain anime numbering flagged. Build within the existing Releases page using cached TVmaze schedules and read-only NAS observations. No new dependencies, external disclosures, notifications or NAS operations are required.

Compare all cached, currently matched episodic titles, independent of their event baseline. Only provider episodes with a known elapsed air date and positive regular-season/episode numbers count. Local explicit season/episode pairs deduplicate copies and include numbered ZIP members. Exclude specials and future/unknown release dates. A season with zero matching local episodes is missing; one with some but fewer than aired is partial. Completeness refers to aired episodes, not a finished season.

Conservative uncertainty: alternate anime ordering, unknown local numbering, metadata conflicts, unavailable/stale source inventory or absent/current-provider schedule coverage become Needs review, never verified missing. Unknown files might contain a proposed missing season. Show latest aired and observed local seasons as context only when evidence permits.

Approach (implementation choice): derive gaps on demand from current database state; reuse schedule refresh and title-detail links. Separate baseline event history cannot identify historical missing seasons; persisting another gap cache would add invalidation complexity. Use one read-consistent database transaction, aggregate local numbering, bound API pages and show searchable missing/partial/review tabs in English/German on phone and desktop. No arbitrary matching guesses or downloads.
