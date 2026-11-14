# Tendstead Lookout — design proposal

Date: 2026-11-13. Status: draft for discussion, not approved for implementation.
Product name selected on 2026-11-14: Tendstead Lookout. No GitHub repository, deployment or publication
is implied. The original workspace-level proposal is preserved as provenance.

## Intent and confirmed requirements

Provide one overview of films, series, episodes and archived public videos across
independently operated servers. The user and friends already use WireGuard and
SSH-key SFTP, with Jellyfin locally and Plex remotely. Reciprocal media-server
accounts are not available and must not become a prerequisite.

The user wants metadata-rich browsing, periodic source checks, notifications for
new remote additions and new episodes/seasons, public-video subscriptions linked
to a downloader, and a documented API for other applications. Reduce routine
checking and maintenance. Existing Seerr requests and players remain useful.

## Corrections and confirmed direction, 2026-11-13

- Query metadata providers directly. Seerr must not be a metadata gateway or a
  required dependency. Include this service's own application credentials so users
  do not need individual metadata-provider keys. TMDB, TVmaze and TVDB are the
  selected provider set; activate credentialed providers when our own access and
  distribution terms are established. No other application's key may be reused.
- Use the existing Gather downloader through an optional integration. Pinchflat
  is a behavioural reference, not a required deployment. Low memory consumption
  is a priority. No implementation language alone establishes actual RAM usage.
- New service should own public-source discovery/polling and catalogue recognition;
  delegate download execution to a connected downloader. Support public sources
  beyond YouTube through explicit adapters.
- Work independently of a downloader; when connected, offer one interface and
  reuse its configuration, shared volumes and destinations instead of asking for
  duplicate configuration. Standalone mode catalogues, discovers and notifies;
  download execution is provided only by an optional connected Gather instance.
- Reuse the user's existing shared PostgreSQL server with a dedicated database
  and role for this service. Do not add SQLite or a separate database container.
- Passwordless organizational profiles, per-profile subscriptions/download actions,
  selectable destination directories and filesystem-based directory suggestions.
  These profiles are not separate authenticated identities or a privacy boundary.
- User reports existing local files are not recognized as episodes in Gather.
  This is a requirement and a reported defect, not a diagnosed or fixed bug.

The separate service is selected for planning. Remaining technical choices below
are proposals. This revision supersedes the earlier Seerr-gateway and shared-only
follows assumptions. No product code, repository scaffold or deployment is authorized.

## Approaches and recommendation

1. Modify/fork Seerr: reuse discovery and request UI, but add a substantially new
   inventory model and scanner to an upstream application. No documented general
   plugin interface was found for this use. Upstream compatibility becomes ours.
2. Assemble existing catalogue/tracking tools: less original code for individual
   functions, but no verified combination covers per-source SFTP inventory,
   catalogue matching and release alerts with a single maintained overview.
3. Separate API-first overview: own catalogue, locations, scans, follows and alerts;
   use direct metadata providers, optional Jellyfin inventory, and the existing Gather downloader. Recommended
   and selected direction for this draft. A separate service still has an ongoing
   maintenance cost; narrower ownership is the intended benefit.

## Boundaries

- Own identities/mappings, source observations, history, follows, release events,
  notification delivery state and API. Own database and migrations.
- Seerr remains an optional request integration. Do not write its DB.
- Jellyfin owns local playback and its existing metadata corrections. Import via
  API. Friend-side Plex API access is optional, not required for SFTP support.
- The overview owns subscriptions, polling schedules, review decisions and source-to-work
  relationships. Gather owns download jobs, extraction during download, recipes,
  output finalization and retries. Do not run two schedulers for the same subscription.
- No player, transcoder, torrent/indexer automation, automatic remote-file copying,
  or media renaming/deletion. Obtaining project credentials is release preparation,
  not permission to register accounts or accept provider agreements automatically.
- Recommendations and LLM assistance are deferred. Filename matching is not an
  LLM-dependent task. Viewing history is not obtainable from SFTP.

## Proposed foundation and deployment

Use Go with an embedded static Svelte 5/TypeScript UI and the existing external
PostgreSQL server, as selected by the user. Give this service its own database,
role and migrations; sharing a PostgreSQL server does not mean sharing Gather's
tables or database ownership. No additional database container or SQLite backend.
Bound the connection pool for the shared server and the low-memory requirement.
Use the shared UI package, purple design tokens, outline
icons, mobile-first layouts, and reactive English/German labels. One scheduled
worker pool; no Redis, message broker, separate scheduler or LLM runtime.

Profiles have no passwords and organize follows, subscriptions, defaults and
destinations. Choose a profile without login inside the trusted deployment. They
are not separate authenticated identities or a privacy boundary. Per-profile
ownership checks prevent accidental cross-profile actions, not malicious access.
Default deployment binds locally; WireGuard/reverse-proxy access can provide the
installation boundary. Offer an installation integration token for external API
clients; no password per profile. Multi-user private histories/roles are deferred.
Exact deployment authentication choices remain open rather than forcing another
identity service. No anonymous public-internet administrative interface is assumed.

## Direct metadata providers and credential constraints

No Seerr dependency. Separate adapters for TV/episode metadata, films, and public
video source metadata. Persist provider-namespaced identities and provenance.

- TVmaze: public keyless JSON API for series, episodes, seasons, schedules and
  artwork. Additional television coverage, subject to German-show coverage and
  localization checks. Respect its caching/rate limits and CC BY-SA data terms.
  It does not solve the film catalogue requirement.
- TMDB: direct film/series lookup requires an application API key or read token.
  Use this service's project-level credential for zero-setup access, subject to
  issuance/distribution terms. Proposed primary provider for films and localized
  series metadata; no borrowed upstream application key.
- TVDB: requires an API key under its licensed/subscriber-supported access models.
  Use this service's project access when obtained; support the adapter without
  promising zero-setup TVDB activation before the provider grants that access.
- Existing NFO/embedded metadata and optional Jellyfin imports remain useful even
  without online credentials. Do not present those as a full online film provider.

Verified clarification: Seerr's TMDB client and Jellyfin's TMDB provider include
application-level keys in their source. Their users therefore do not need to
register individual TMDB keys, although the upstream calls are authenticated.
The user selected the same experience using our own provider-approved application
credentials; never reuse Seerr/Jellyfin's key. Allow optional installation-key
overrides and rotation without requiring every profile to configure credentials.
Application keys in distributed software are not user-authentication secrets and
cannot be made unextractable. Keep them distinct from installation integration
tokens, SSH keys and Telegram credentials. Confirm provider terms before shipping.

Support multiple provider adapters with per-provider IDs, provenance, capability
flags, locale preferences and configurable precedence. Alternate providers can
fill missing data, but conflicting episode numbering or uncertain cross-provider
identity requires explicit matching; do not merge solely by title or SxxExx.

Implement adapters against fixtures while project credentials are being obtained.
Do not block unrelated implementation on a live provider account. If project access
is unavailable, show that provider as disabled and retain cached/local data and
other enabled providers; do not silently require personal keys. A web page accessible
without login does not establish an unauthenticated API contract.

Search yields candidates; persist a selected provider ID rather than repeating
name matching. Use internal UUIDs and external aliases. Keep locale, attribution,
source and refresh timestamp on cached fields. Do not silently merge provider
numbering schemes. Preserve manual corrections separately. Bound refreshes to
followed/catalogued works instead of mirroring whole upstream catalogues.

If a provider is unavailable, cached data remains usable and source scans continue.
Unknown release dates remain unknown. Film/series metadata and public-source
availability are separate evidence. An air date does not prove ARD availability.

## Catalogue model and matching

Principal entities: Title (movie/series/public-video collection), Season, Episode,
Source, FileObservation, Match, ScanRun, Follow, ReleaseObservation, Event and
NotificationDelivery. Public videos use extractor/source IDs and original URLs;
retain explicit channel/playlist membership and publication dates. Offer a channel/episode
browsing model without inventing equivalence with TVDB/TMDB television episodes.

Separate a work from each physical copy. Locations include source ID, relative
path, size, modification time, observed status, first/last seen and scan ID.
Multiple editions/qualities may reference one work. No whole-file downloads or
hashing are needed for ordinary catalogue polling.

Match using trusted external IDs/NFO metadata first, then filename/title/year and
season/episode parsing. Sidecars are bounded untrusted input. Ambiguous matches
remain in a review queue. Record numbering schemes; multi-episode files and
ambiguous specials must not silently be assigned to an incorrect episode.
Manual corrections survive rescans. Language/codec/quality stay unknown unless
observed through existing metadata; do not imply SFTP listings reveal them.

## Source scanning and changes

Initial inventory adapters: local/mounted folders and read-only SFTP, with optional
Jellyfin API import. Local scanning is first-class so standalone operation and
shared Gather volumes do not depend on a media server. It does not replace SFTP.
Use native Go SFTP with SSH host-key verification; rclone is optional rather than
another required runtime. Configure allowed roots, schedules and concurrency.
Credentials remain server-side and are excluded from normal API responses/logs.

Persist a scan before comparing observations. The first successful scan establishes
a baseline without sending an alert for every existing file. Only a complete,
successful scan of the configured scope can establish absence. Failed, partial,
unreachable or disconnected scans retain the last known inventory and show stale
status. Use configurable confirmation of absences before marking files missing.
Never delete physical files in response to a scan.

A successful listing alone does not prove a backing mount is available. Use a
configured root marker or equivalent availability check for mounted/SFTP roots.
When no reliable check is available, quarantine suspicious bulk absences for
review; repeated empty listings must not automatically erase known inventory.

Recognize temporary uploads and require stable size/mtime across checks before
declaring new media available; this is a heuristic, not proof of playable media.
Avoid following paths/symlinks outside configured roots. Overlapping scan jobs
for a source serialize. Record additions, changes and confirmed missing items.
Same-source renames with uncertain identity are not automatically new content.

Proposed defaults: source checks hourly with jitter, configurable per source;
metadata checks daily for followed titles. Use timeouts, bounded concurrency,
backoff and Retry-After handling. Tune defaults after a real-library pilot.

## Follows, releases and notifications

Keep distinct events: episode/season metadata discovered; scheduled air date
reached; public-source video discovered; file newly available on a named server.
Label air-date events as reported aired, never as verified downloadable, purchasable,
available in German, or present on a server. Announcement discovery and premiere
are different events. Missing/changed dates and unknown numbering stay visible.

Follow selected titles for release updates and selected sources/titles for library
additions. Defaults are future-only after a baseline. Metadata-derived release
events are deduplicated by work/event identity; date corrections do not repeatedly
announce the same premiere. First episode/season-boundary rules must be defined
and verified during the compatibility check; include late provider updates.

Telegram is the initial outbound channel, sent directly by this service. No
additional notification daemon required. Persist events and an outbox transactionally,
retry with backoff, and show failed deliveries. At-least-once sending can duplicate
a message after an ambiguous transport failure; do not promise exactly-once Telegram
delivery. Batch additions by source/series; optional daily digest. Source failure
and recovery alerts are deduplicated independently of media additions.

## Source discovery, recognition and optional Gather integration

Implement the Pinchflat-style workflow independently: resolve a submitted channel
or playlist to a stable source identity, establish an initial baseline, poll on a
schedule, detect new items by stable IDs, apply filters, reconcile existing files,
then queue for review or submit an authorized automatic download. Keep channel,
playlist and video identities separate; one video can belong to multiple playlists.
Support regular videos/Shorts selection and explicit backfill handling.
Proposed initial adapters: YouTube channels/playlists and explicitly selected ARD
seasons. Existing Gather supports channel-shaped subscriptions and explicit ARD
seasons; playlist subscriptions are new work, not an existing verified capability.
Future-season discovery on ARD
needs separate validation; do not claim it follows from polling one season.
Other public services require individually scoped adapter work.

Pinchflat is AGPL-3.0. Behavioural study does not imply copying code or translating
its implementation. Retain an original implementation unless licensing is explicitly
reconsidered. Generic public-site discovery needs a tested adapter; yt-dlp support
for a known URL does not guarantee discovery of new seasons or uploads.

Standalone mode must provide source discovery, catalogue scans, profiles, follows,
review inbox and alerts. No Gather dependency for these functions. Without Gather,
offer original-source links and notification/review actions; do not execute downloads
or imply candidates are queued for execution. Connecting Gather must not trigger
downloads of the accumulated inbox automatically: require an explicit activation
and initial/backfill policy. No built-in download engine is planned.

Metadata-only extraction may still be needed for discovery in standalone mode;
this is distinct from downloading media. Keep source polling and scheduling here,
and reuse a bounded original-code discovery/identity module if both applications
need the same extraction logic. Gather's Go internal packages cannot simply be
imported by a sibling repository: define a versioned shared module if extracting
common helpers, or use an API boundary where that preserves standalone operation.
Confirm that extraction-tool memory fits the budget.

Connected mode submits jobs to Gather and tracks completion through documented APIs.
One interface manages subscriptions; do not also create equivalent independently
scheduled Gather subscriptions. Existing Gather subscriptions require an explicit
migration/ownership choice, not silent disabling or duplication. Gather remains
independently useful for manual downloads and its other supported content types.

Reuse Gather recipe IDs, profile IDs and destination root IDs through an adapter.
Map local overview profiles to Gather profiles explicitly; names are not identity.
Read configuration once from its owner instead of sharing databases or importing
secret-bearing environment files. A failed executor must not stop scans or polling.
Job submission needs an idempotency key and a stored correlation to the candidate.
Snapshot the selected recipe/destination so later changes do not redirect queued work.
Verify Gather's submission contract can select a recipe revision or accept an
immutable settings snapshot; a local snapshot alone cannot constrain the executor.
Proposed cross-profile duplicate policy: all profiles can see the shared catalogue.
An existing confirmed source identity marks an item archived with its actual locations.
Multiple subscriptions can reference it without extra jobs. Concurrent requests for
identical source/recipe/destination coalesce, preserving each interested profile.
Catalogue presence, destination presence and subscription policy are separate.
A remote-only copy never fulfills a requested local destination or silently
suppresses an automatic local download. Coalesced jobs need explicit ownership
and per-profile status; confirm Gather can support this before offering it.
An existing local copy at a different destination or format is not silently treated
as fulfilled: offer reuse or an explicit extra copy/variant choice. Automatic mode
defaults to review for a conflicting local copy; remote-only copies do not invoke
this rule. Never redownload
solely because a mount is unavailable. Persist the policy on each subscription.

Shared volumes require named roots and relative paths, not identical container paths:
Gather /downloads and overview /library can map to the same root ID. Validate the
mapping against known completed outputs. Do not duplicate media files. Detect mount
loss and report unavailable. Use read-only overview mounts where possible.

Profiles select a default recipe, default destination and allowed named destinations;
each manual submission/subscription may select a permitted destination. The downloader
must validate destinations independently. Filesystem autocomplete browses directories
on demand within configured roots (bounded, paginated, symlinks confined), not the
whole host filesystem. Return named-root/relative-path choices. Offline directories
remain configured but are marked unavailable. Creating directories is a separate
explicit operation, not a side effect of suggestions. Gather's current API models a
recipe destination as a root ID: subdirectory overrides/browsing require a planned
API extension rather than assuming they exist today.

Recognition is a first-class prerequisite, not just downloader bookkeeping. Persist
extractor/video identity, canonical URL, channel/playlist memberships, upload date,
work/episode mapping, and physical file location separately. Match reliable IDs from
sidecars/embedded metadata first, then bounded filename-ID parsing and explicit
manual association. Fuzzy title matches are review suggestions. Ambiguous files
must not silently count as missing or trigger redownloads.

Code inspection: Gather imports bounded info.json/embedded source IDs and URLs,
including YouTube and ARD identity normalization; its current File model has no
explicit channel/playlist/season/episode relationship. It has tests for some identity
imports, which do not prove the user's deployed files work. Diagnose actual failing
examples and deployed versions separately before prescribing a fix. New download
completion should supply durable source identity and optional descriptive sidecars
so either application can recover after database loss. Choose one authoritative
source-identity contract; do not maintain divergent recognition rules in both apps.

## Resource requirements

Target an idle Go process with a bounded in-memory working set; use incremental DB
queries and batched listings rather than a full-library snapshot per request. No
permanent Elixir runtime, Redis, extra scheduler, metadata server or LLM dependency.
Keep discovery/download/ffprobe concurrency separately configurable, with low defaults.
Avoid full-file hashes or ffprobe on every periodic scan. Record measured idle RSS,
peak discovery RSS, scan time and downloader peaks on representative libraries.
Establish a baseline and proposed numeric targets in an early bounded spike.
Low RAM is a requirement, not a benchmark result or a reason to promise a number
without testing. Ask for a product decision only if measured tradeoffs require it.

## User interface and public API

Proposed views: unified library; title detail with episodes and per-source copies;
sources with health/last successful scan; activity; follows/upcoming; match review;
integration settings; a profile switcher, per-profile subscriptions/inbox, and
filesystem-backed destination selection. The catalogue remains usable while a source is offline.
Explain missing/stale/unknown instead of collapsing them into unavailable.

Versioned /api/v1 API with published OpenAPI, stable internal IDs, pagination,
filters, timestamp conventions and structured errors. Here public means documented
for other applications, not necessarily anonymous or exposed to the internet.
Read endpoints return catalogue, metadata, source health, availability, activity
and follows. Write endpoints configure follows, mappings and sources, trigger
scans and acknowledge activity. Do not put writes behind GET endpoints.

Offer name search and ID-based details, e.g. GET /api/v1/search?query=... and
GET /api/v1/titles/{id}. Cached local reads do not require upstream calls per
request. Paths/credentials are not exposed in generic metadata responses.
Optional outbound webhooks can follow after durable Telegram/event handling;
document event IDs so consumers can deduplicate.

## Proposed delivery order and acceptance gates

1. Verify provider coverage/access and establish a
   resource baseline. Verify direct TV data against Feuer und Flamme and
   provider numbering; capture a real failing local-file recognition example.
2. Define canonical identity, file-membership and named-root contracts. Diagnose
   Gather recognition with disposable representative files before assuming reuse.
3. Catalogue MVP: read-only SFTP/local scans, optional Jellyfin import, direct
   metadata adapters/cache, matching review, source health and read API. Verify
   multiple copies, absent sidecars, ambiguous names and unavailable mounts.
4. Profile follows and core remote-addition/release alerts: durable outbox,
   per-profile Telegram preferences, restart/retry handling, corrected air dates
   and baseline rules. This is the first useful standalone milestone.
5. Profile source subscriptions: channel/playlist/source discovery, initial/backfill
   rules and notification/review inbox. Verify stable IDs and no duplicate schedulers.
6. Optional Gather execution: contract spike, then idempotent job submission,
   profile/root mappings, directory suggestions, recipe revisions, progress/completion import,
   shared-volume path mappings, disconnected executor handling and recovery. Keep
   manual Gather workflows working. Coordinate any Gather API changes separately.
7. Benchmark idle/scan/discovery peaks, perform backup/restore and mobile UI/API
   checks, document setup from scratch. Recommendations remain a later decision.

Use deterministic fixtures and disposable resources for tests. No destructive
production-library tests, deployment, publishing or sibling-code edits are implied.

## Decisions still needing agreement

- Product name is Tendstead Lookout. Application license is Elastic License 2.0
  (`Elastic-2.0`), confirmed by the user on 2026-11-14 after comparison with
  Roadbook. Establish numeric RAM
  targets from the initial baseline rather than blocking foundation work on a guess.
- Whether automatic future-season discovery on ARD is required initially remains
  open. Other duplicate/source defaults are proposed builder choices above.
- Exact existing-file recognition failure needs evidence before a fix is planned.

## Independent pre-implementation review

Reviewed by a fresh subagent on 2026-11-13 at the user's request. The reviewer
found the design implementable. Incorporated local scanning as a first-class
adapter, mount-health checks, remote-vs-destination fulfillment, earlier core
notifications, explicit Gather contract verification, playlist/shared-module
scope and measured memory targets. The outstanding user decisions are initial
ARD future-season discovery scope and product/repository name/license. Existing-file
recognition and provider access remain verification work, not guessed guarantees.

## Sources and inspected local context

- TVmaze keyless API and data license: https://www.tvmaze.com/api
- TMDB application authentication:
  https://developer.themoviedb.org/docs/authentication-application
- TMDB terms/attribution overview: https://developer.themoviedb.org/docs/faq
- TVDB API access models: https://github.com/thetvdb/v4-api
- Seerr bundled TMDB application credential (value intentionally not copied):
  https://github.com/seerr-team/seerr/blob/develop/server/api/themoviedb/index.ts
- Jellyfin bundled TMDB application credential (value intentionally not copied):
  https://github.com/jellyfin/jellyfin/blob/master/MediaBrowser.Providers/Plugins/Tmdb/TmdbUtils.cs
- Pinchflat reference and AGPL license: https://github.com/kieraneglin/pinchflat
- Gather README.md, docs/api.md, internal/library/metadata.go,
  internal/library/library.go, internal/jobs/discovery.go and
  internal/domain/model.go inspected 2026-11-13. Not a live deployment verification.


## Local start, 2026-11-14

The user authorized starting locally and publishing later. Use shopping-list as
an architectural reference: one Go executable with embedded Svelte 5/TypeScript
assets, shared UI, container-based development and checks, external PostgreSQL
with a dedicated database/role and bounded pool. Implement original Lookout code;
do not copy shopping-list application code. Use Elastic License 2.0 for original
Lookout code; preserve third-party licenses and notices separately.

The first implementation plan covers the deployable foundation only. It does not
represent the complete media catalogue. Subsequent plans cover identities and
scanning, catalogue matching/providers, follows/notifications, discovery, and
optional Gather execution in the delivery order above. Existing database servers,
NAS files and deployed applications must not be changed by foundation tests.

Builder choices for the first milestone: module path `github.com/tendstead/lookout`
(reserving a future location does not require that the remote exists); an initial
`feat/lookout-foundation` branch; `/healthz` for process liveness and `/readyz` for
database readiness; integration tests on disposable PostgreSQL. Numeric memory
targets remain to be established from observations. Preserve existing Git signing
settings. Do not add a remote or publish until requested.
