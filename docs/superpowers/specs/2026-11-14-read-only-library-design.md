# Read-only NAS library, metadata and release tracking

Status: implementation authorized by the user's active goal, 2026-11-14.
Builds on the approved Lookout design and existing local foundation. The goal is
not complete until all requested media is visible and the required behavior is verified.

## Goal and boundaries

Import all playable media under the four user-selected NAS roots over SFTP. The
WHOLE remote server is read-only: no uploads, mkdir, chmod, renames, deletions,
metadata writes, mount changes, account changes, shell jobs or remote configuration.
Only directory/stat and bounded file reads are used. Runtime credentials are local
secrets. Existing SSH config supplies connection identity; known-host validation
is mandatory. Do not send Telegram test messages; validate using read-only bot/chat
methods. New genuine episode notifications are optional and must have a baseline.

Roots for this installation are private runtime configuration, not hardcoded app
constants: Serien and Anime_Serien are episodic; Filme and Anime_Filme are films.
Keep all media visible even when metadata/numbering is unknown. Media is catalogued
in place, never copied in full. Small bounded sidecars/artwork may be cached locally.

## Selected approach and alternatives

The original design selected a standalone Go/Svelte/PostgreSQL service with optional
media-server adapters. Extend that service using native SSH/SFTP rather than shell
commands or an additional scanner service. A shell-driven importer would be quicker
but harder to constrain to read-only operations and integrate with progress/retries.
Relying only on Jellyfin/Plex would exclude SFTP-only sources and unknown titles.

## Storage and scanning

Separate sources, root scan state, physical files, catalogue titles, provider IDs,
episodes, release observations and outbox events. PostgreSQL migrations are ordered,
transactional and locked. A file identity is source+relative path, not a guessed title.
Group series by their top-level directory, films by film directory or direct file.
Record size/mtime, season/episode when parseable, source provenance, and unknowns.
Do not collapse multiple copies or editions into one physical file.

The scanner receives a narrow read-only filesystem interface with no write methods.
Skip symlinks, confine reads to validated roots, reject traversal, bound NFO/XML and
image reads, and serialize a source's scans. A complete scan gets a durable status.
Failed/partial scans preserve prior data. Suspicious empty/bulk-absence scans retain
inventory for review; no scan can delete source media or silently erase the catalogue.
Scans run in a bounded background worker so HTTP requests do not wait for full trees.
Initial ingestion establishes a baseline, not thousands of new-item notifications.

## Metadata

Use bounded local NFO/embedded provider tags first. Persist trusted TVDB/TMDB/IMDb
identities, title, year, description and provenance. Public TVmaze lookup/search is
an official keyless option for series/anime television data. Use stable provider
IDs once chosen; fuzzy results are suggestions requiring review. Episode numbering
schemes remain distinct; absolute-numbered anime files must not be invented as
season-one TV episodes. A provider air date is reported aired, not proof of a local
file, German availability or download availability.

TMDB/TVDB credentialed adapters require approved credentials; never borrow another
application's key. Missing keys must not block NAS import. Offer optional Jellyfin
and Plex connections through their documented read-only APIs even when direct
metadata works. Both adapters import IDs/descriptions/images and retain media-server
provenance. Map paths explicitly; a title-only overlap does not prove file identity.
Store adapter tokens encrypted locally; API responses show configuration state,
never tokens. Cache artwork through bounded server-side reads rather than leaking
credentials into image URLs. Preserve attribution and provider URLs.

## API and UI

Add a paginated/searchable/filterable overview showing every imported title, four
categories, cover/placeholder, file/episode counts, provenance and match status.
Title detail shows observed files, season/episode grouping and unknown numbering.
Sources show scan progress, totals and failures. Connections offer Jellyfin/Plex
URL/token setup and Telegram verification state. Review/search permits correcting
metadata matches. Keep bilingual labels and mobile/desktop navigation.

Writes require JSON and same-origin protections inside the trusted-LAN deployment;
credentials stay server-side. Bound queries and page sizes. No remote filesystem
write endpoint exists. API retains the health and status contracts and documented
structured errors.

## Releases and Telegram

Follow selected series/anime titles and fetch provider episode schedules. Persist a
baseline, distinguish upcoming vs reported-aired episodes, and detect new seasons
from their regular first episode, excluding specials. Stable provider episode IDs
and durable event uniqueness prevent restart/refresh/date-correction duplicates.
Initial history does not generate release spam. Future dated episodes create an
event when their time/date is reached after the baseline. Unknown dates remain
unknown, and conflicting numbering is reviewable. Test future→aired, new-season,
late metadata, unknown dates, specials, repeated polls, restarts and provider errors.

Telegram configuration is an ignored local secret file. Verify bot/chat/member
using getMe/getChat/getChatMember; do not call sendMessage for testing. An optional
outbox sends only genuine new release events after explicit enabling, with retry
and correlation, never synthetic events or baseline history. UI explains that
connection validation is not proof of successful message delivery.

## Acceptance evidence

1. Remote I/O implementation has no mutating SFTP/shell operations; scanner safety
   tests reject outside-root/symlink reads and preserve inventory on failures.
2. Real complete scans cover each configured root; independent counts reconcile
   every playable file with catalogue rows and overview pagination totals.
3. Browser tests and live screenshots prove browsing/search/detail on phone/desktop.
4. Official-provider live lookups and fixture tests show correct ID handling,
   ambiguity and provider failures. Media-server options remain available either way.
5. Episode/season transition and dedup tests use controlled clocks; inspect real
   series/anime schedules with provenance without treating library absence as release.
6. Telegram live validation succeeds without messages. Optional genuine event
   delivery, if enabled, is audited separately and never simulated against the chat.
7. Run full checks with bounded tools/builds, restart local Compose, verify LAN
   readiness and persisted data. Commit small increments; preserve signing settings
   and record signing timeouts for end-of-work verification instead of stalling.
