# Group seasons by series and use available width

User asks for one card per series instead of one per missing season, plus a grid that automatically adjusts its column count while resizing. Existing poster/coverage design and comparison confidence stay intact.

Add opt-in `group_by=title` to the gap endpoint. Group by stable title ID after kind filtering and before pagination; complete per-title season arrays remain on the same page. Keep season totals for Missing/Partial tab counts, but grouped `total` counts series cards. Flat default API remains compatible. Missing and partial tabs keep separate series groups; review items stay one per title. Aggregate counts describe only seasons in the selected tab, and the current season's coverage remains explicitly labeled.

One shared poster/title per series; multiple missing/partial seasons appear as accessible selectable season chips inside that card. Selecting a season changes its coverage and missing-number tiles, preserving every season's details. Single-season cards keep their existing presentation. Series with identical names but different IDs remain distinct.

Use CSS Grid auto-fill with a readable minimum card width, fluid columns and a width-safe single column on narrow phones. Expand only the Releases page beyond the old880px cap so wide displays can fit three or more cards. Other views keep their current widths. No new dependencies or NAS/provider/Telegram behavior changes.
