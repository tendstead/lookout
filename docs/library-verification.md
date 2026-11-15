# Local library verification — 2026-11-15

The four requested NAS roots were read without moving, renaming, extracting,
writing or deleting NAS content. Existing SSH key/known_hosts files were mounted
read-only; their values and permissions were preserved. This verifies client
operations, not a server-side read-only restriction on the existing account.

| Root category | Ordinary video files | ZIP video members | Title cards |
| --- | ---: | ---: | ---: |
| Series | 6,937 | 52 | 72 |
| Films | 279 | 0 | 279 |
| Anime films | 25 | 0 | 25 |
| Anime series | 2,297 | 0 | 59 |
| Total | 9,538 | 52 | 435 |

An independent scoped read-only `find -P` inventory matched the SFTP set exactly:
zero missing/extra ordinary video paths. The app itself uses SFTP, not a shell.
A separate ZIP-directory audit found 52 video members; bounded scanner inspection
then imported those members as distinct, repeatable catalogue entries. The ZIP
was neither extracted nor rewritten. Other files such as subtitles/artwork are not
counted as videos. Non-XML NFO files generated nonfatal warnings; no valid NFO
metadata was available in this scan.

Jellyfin supplied metadata for 430/435 local titles, with zero identity conflicts;
three upstream records outside the observed/mapped scope were skipped. Both
Jellyfin and Plex adapters have mock-server coverage, while the user's live
connection is Jellyfin. The container used the explicitly authorized direct LAN
address because the host's local reverse-proxy hostname was unreachable inside it.

TVmaze resolved 116/131 episodic titles (64 series, 52 anime series). The remaining
15 retain their library entries and available Jellyfin metadata and are listed
for manual matching. Schedules cache 11,079 episode records, including 3,435 anime
episodes; nine future entries were reported at baseline. No future anime dates
were supplied at that time. Positive anime/series episode and season-premiere
transitions were tested with controlled clocks, independent of live schedules.

The initial fetch produced zero release events/outbox messages. Repeat polling,
restarts, date correction, specials, unknown dates, provider changes, unfollowing,
coalescing and retry behavior have automated coverage. Real Telegram verification
used getMe/getChat/getChatMember only. Real messages sent: zero. Optional future
delivery remains disabled; enable it as described in [library setup](library.md).

The full check covers Go vet/race/unit tests, disposable PostgreSQL migrations and
release/outbox tests, mock Telegram delivery, Svelte/TypeScript, 16 desktop/mobile
browser cases, production build, startup/shutdown/database outage behavior,
govulncheck, npm audit and actionlint. Latest stable dependency pins were rechecked
against official registries on this date; TypeScript 6 is the approved compatibility
exception. Two low build-only advisories have no upstream fix; no Go findings.

A fresh whole-feature review found three Important issues, reproduced by regression
tests: mutable-name pagination skipping metadata targets, startup verification
suppressing genuine notifications, and public artwork fetches ignoring the disabled
public-metadata flag. All three were corrected; a focused follow-up found no
remaining Important findings. Metadata conflict status was also made visible.

Only Lookout's own database/configuration and local repository were written.
No remote repository, push or release was created. README screenshots show the
actual LAN library and mobile release view; secrets and raw inventory evidence
remain in ignored local files.

## Missing-season comparison follow-up — 2026-11-15

The Releases view now compares historical and newly aired numbered episodes with
observed local season/episode sets. A live read-only query and complete API page
walk agreed on 45 missing seasons, 23 partial seasons and 84 titles needing review.
Unknown numbering, anime ordering, stale/unavailable sources and unmatched metadata
remain explicit review items. ZIP/loose duplicate copies do not inflate episode
coverage; old provider identities do not contribute current schedules.

The full check passed with 20 desktop/mobile cases and isolated database/domain/API
regressions. Fresh review found no Critical/Important issue and one pagination edge
case, reproduced with a controlled browser clock and corrected. Visual screenshot
inspection subsequently exposed inherited flex styling on cards; geometric browser
regressions reproduce it and require full-width titles above season details. The
card and review-list layouts now explicitly override that inherited styling.

No new dependency, migration, provider request, source scan or Telegram action is
introduced by computing gaps. The existing inventory and release workers retain
their schedules. Telegram delivery remains disabled, and this comparison generates
no historical notifications. See [library setup](library.md) for interpretation.

After the layout correction, Svelte/TypeScript, production build, all 20 browser
cases and lifecycle checks passed again. The same 20 cases passed against the
healthy LAN service, and the refreshed phone screenshot was visually inspected.
The running image digest matched the tested image; all API comparison counts
remained 45/23/84 after deployment.

## Visual season cards follow-up — 2026-11-15

Missing/partial seasons now use existing series posters, prominent season numbers,
state badges, proportional aired-episode coverage and recognized local-season chips.
Missing episode numbers expand into numbered tiles; timestamps move into Data
freshness. Review cards preserve uncertainty and omit numerical coverage claims.
Artwork failure keeps title links and coverage usable. No backend, comparison,
provider-data, dependency or notification configuration changed.

Browser regressions were observed failing before implementation, then passing for
artwork identity, zero/partial coverage, episode tiles, review states, long titles,
missing artwork, German/dark theme and narrow screens. A fresh focused review found
no Critical/Important issue. Its keyboard-scroll finding was resolved with named,
focusable number groups and visible focus outlines; Svelte's generic noninteractive
focus warning is narrowly documented at these two deliberate scroll containers.
Desktop/mobile screenshot inspection confirmed a two-column desktop grid and one
column on phones without horizontal overflow.

Final visual-card verification: Svelte/TypeScript reported zero errors/warnings;
all 22 desktop/mobile browser cases and disposable lifecycle checks passed. The
same 22 cases passed through the deployed LAN service. Desktop/light, desktop/dark
and phone screenshots were inspected with actual posters; the refreshed README
phone screenshot is readable. The healthy running image matched the tested image.

## Grouped series cards and fluid grid — 2026-11-15

The opt-in grouped endpoint and Releases UI now keep every missing/partial season
for one stable title ID in one series card before pagination. Per-season counts
remain unchanged: live verification reconciled 45 missing seasons into 15 series cards,
23 partial seasons into 8 cards, and84 review titles into 84 cards. Every flat season
identity was present exactly once in grouped output; sums, order and counts agreed.
SpongeBob's nine missing seasons appeared in one card with individual selection.

CSS Grid now automatically fits readable 340px minimum columns to the Releases
container, including wide displays. Actual viewport checks reported 4 columns at
1800px, 2 at 1280px, 1 at 1000px and 1 at 390px, with no horizontal overflow. Other pages
retain their existing content-width limits. The README phone screenshot shows an
actual grouped card.

Store/API/browser regressions first reproduced the missing behavior. Full make
check passed with 24 desktop/mobile tests, 0 Svelte/type errors or warnings, isolated
PostgreSQL grouping/paging checks and lifecycle/audit checks. A fresh read-only
review reported no findings. The same 24 browser cases passed on the LAN deployment;
actual grouped phone and wide-screen cards were visually inspected. No new
external dependency, migration or provider/notification behavior was introduced.

## Consistent season lists — 2026-11-15

Single and multiple missing/partial seasons now use the same labeled numeric chip
list. The previous large season-number/count variants were removed. Chips preserve
accessible Season N names, pressed state and explicitly labeled per-season coverage.
Review cards retain their uncertainty treatment. Browser regressions were observed
failing for the old single-season display, then all 24 desktop/mobile cases passed;
Svelte/TypeScript reported zero errors and warnings, and production build/lifecycle
checks passed. The independent reviewer was unavailable because of a service usage
limit; the small diff was reviewed inline instead. Existing server support was
verified from source and documented without adding inferred server identities.

The deployed LAN service passed all 24 browser cases, and the refreshed phone
screenshot was visually inspected. Read-only series/anime scans were refreshed
when the freshness guard correctly classified the old inventory as needing review;
normal comparison cards returned afterward. No NAS files were changed.
