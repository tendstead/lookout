# Library and integrations

For the single-file TOML deployment, see [Configure Lookout](configuration.md).
The environment, JSON, and browser setup steps below describe the legacy deployment.

SFTP transfers files over an authenticated SSH connection. Lookout uses its native
SFTP client for directory listings, file attributes and bounded sidecar/archive
reads. It verifies the server against an existing known_hosts file, skips symlinks
and rejects paths outside configured roots. It issues no remote shell commands or
write operations. A server account restricted to reading those roots is recommended;
client read-only behavior does not change the permissions of your SSH account.

## Browsing, search and display width

Overview and Releases both use fluid grids, with compact poster cards on Overview
and larger information cards on Releases. At 1920px, Overview fits eight roughly 184px
cards across; phones typically show two. **Limit width** caps Overview at 1120px
(about six poster cards) and Releases at 1600px (about four information cards), so
both remain readable on ultrawide screens. Narrower windows keep adapting.
The toggle is shared between those pages and saved only in this browser's
localStorage (`lookout.limitWidth`). It survives reloads and browser sessions until
cleared; other devices/browsers are independent. If storage is blocked, it still
works for the current page session.

Overview search suggests up to eight locally catalogued titles after two characters
and a short debounce. Suggestions include films, series and anime and respect the
active category. Only titles with at least one observed/stable file are eligible;
titles with exclusively missing or unconfirmed files are excluded. The dropdown
uses text from Lookout's database only: typing does not contact metadata providers
or fetch suggestion artwork. Use arrows and Enter to open a title, Escape to dismiss,
or select with the pointer. The ordinary Search button remains available if the
suggestion request fails. Obsolete requests are cancelled/ignored.

### Where artwork lives

The browser loads every card image through Lookout's own artwork endpoint. On a
cache miss, Lookout fetches the image server-side from the configured Jellyfin/Plex
server or permitted TVmaze artwork host, validates and bounds the response, and
stores the bytes in PostgreSQL (`metadata_images`). Cached artwork needs no upstream
request. Images are loaded on demand rather than predownloaded for the whole library;
uncached images still need the upstream source. Connection tokens stay server-side.

## Source setup

Complete [database deployment](deployment.md) first. Create a private runtime
directory, e.g. `.implementation/runtime` (mode 700), with `catalogue.json`:

```json
{
  "ssh": {
    "host": "nas.example.lan", "port": 22, "user": "media-reader",
    "key_file": "/run/secrets/ssh-key",
    "known_hosts_file": "/run/secrets/ssh-known-hosts"
  },
  "roots": [
    {"id":"series","label":"Series","path":"/media/Series","category":"series"},
    {"id":"films","label":"Films","path":"/media/Films","category":"movie"}
  ]
}
```

Categories are `movie`, `series`, `anime_movie`, `anime_series`. Give each source a
stable ID and absolute, normalized root. Keep existing source IDs/roots stable.
The native client accepts an unencrypted private key; encrypted key/agent support
is not implemented. Preserve verified known_hosts entries rather than disabling
host-key verification.

Create `connection-key` as 32 random bytes in that directory (mode 600). This key
encrypts media-server tokens in PostgreSQL; back it up securely with the database.
For example: `openssl rand -out .implementation/runtime/connection-key 32`.
Do not regenerate it for an existing database.

Add these host settings to `.env`, using your actual absolute paths and nonroot UID/GID:

```dotenv
COMPOSE_FILE=compose.yaml:compose.integrations.yaml
LOOKOUT_UID=1000
LOOKOUT_GID=1000
LOOKOUT_CONFIG_DIR=/absolute/path/to/.implementation/runtime
LOOKOUT_SSH_KEY_FILE=/absolute/path/to/existing/private-key
LOOKOUT_KNOWN_HOSTS_FILE=/absolute/path/to/existing/known_hosts
ALLOW_PUBLIC_METADATA=false
FOLLOW_IMPORTED_SERIES=false
TELEGRAM_DELIVERY_ENABLED=false
```

The integration override runs as that nonroot UID so it can read existing private
files without copying or relaxing their permissions. All integration mounts remain
read-only. Recreate with `docker compose up -d --no-build --wait`. Sources scan on their server’s configurable interval (six hours by default); use the Sources screen to request the first scan or a rescan. Scans serialize within each SFTP host, with at most four simultaneous SFTP scans. Automatic API inventory has a separate two-job bound; the existing manual metadata worker processes one queued job. Failed, empty or suspiciously reduced scans retain the previous
inventory instead of silently removing it. Missing observations remain inspectable.

## Inventory interpretation

Series group by top-level series directory. Movies group by individual leaf/file,
including films inside collection folders. Relative paths retain those folders;
there is no separate editable collection entity. Explicit season/episode patterns
are recognized; ambiguous absolute anime numbering remains unknown.

ZIP support lists bounded central-directory metadata only, never extracts members
or reads their video bodies. Each media member has `storage: "zip"`, a physical
archive `relative_path`, and separate `archive_member`. The UI labels it as archived,
not directly playable. Other archive formats are not expanded. Nonmedia files are
excluded; bounded NFO parsing failures become scan warnings. Entry counts include
ordinary video files and ZIP media members, not just physical files.

## Jellyfin / Plex

Use **Settings → Connections** to choose either server, save its URL/token, verify it and import
metadata. These adapters use read-only HTTP GET requests; they do not change the
server library. Tokens are encrypted in Lookout's database and never returned by
its API. Browser artwork requests go through Lookout's bounded authenticated proxy.

Add explicit path mappings: server prefix (e.g. `/data/Series`), source ID (`series`),
and matching NAS prefix (`/media/Series`). Items outside those mappings or without
an observed local path are skipped; no title-only association is made.
A manual metadata choice is preserved during subsequent imports.

For a scripted initial import, `cmd/enrich-library` accepts `-config` pointing to
a private `{"kind":"jellyfin","url":"http://server:8096","token":"..."}` file,
`-mappings` pointing to a JSON array of path mappings, and `-key` pointing to the
same connection encryption key. It uses the database environment settings; add
`-official` only with consent for TVmaze lookups. Mounting a JSON credential file
alone does not import it; use the CLI or the **Settings → Connections** form.
The host and container must both resolve/reach your chosen URL; a LAN address can
be used when a local reverse-proxy hostname is unavailable inside Docker.

## Public metadata and releases

Set `ALLOW_PUBLIC_METADATA=true` only when you want Lookout to send series/anime
titles and TVDB/IMDb identifiers to TVmaze. This shares names/IDs, not credentials
or media. **Settings → Connections** can refresh official metadata; title detail supports searching
and selecting a manual match. TVmaze attribution links are retained.
Movies currently use local/Jellyfin/Plex metadata; TVmaze covers episodic titles.

Follow individual series/anime in title detail, or use `FOLLOW_IMPORTED_SERIES=true`
to seed follows for imported episodic titles. Schedule caches refresh every six
hours, with due dates checked locally each minute. Releases shows unmatched follows
for manual review rather than implying schedule coverage.

### Missing and partial seasons

Open Releases → **Seasons missing from your library**. Missing seasons have no
recognized local copies of their aired numbered episodes. Partial seasons have
some, with a list of the aired episode numbers still absent. Each series gets one
poster card per selected tab, with all of its missing or
partial seasons grouped together. Select a season button inside the card to
change the explicitly labeled coverage and missing-episode details. Each card
shows missing or partial season numbers as the same compact, selectable list
for one or many seasons, alongside the latest aired season and an accessible
coverage bar. The selected season is labeled above its episode coverage. The
poster is existing series artwork, not a claimed
season-specific cover. Chips show recognized local season numbers, without
implying those seasons are complete. Expand missing episode numbers to see
individual tiles, or Data freshness for schedule-check and source-scan times.
Cards automatically fit as many readable columns as the available Releases width
allows, and fall back to one width-safe column on phones. Artwork has an explicit
fallback when unavailable. Review cards show their uncertainty reasons without a
coverage bar. Search filters all three tabs; each has its own paginated series
results. Seasons
are grouped before pagination, so a series never splits across pages. Tab counters
still count missing/partial seasons, while page totals count series cards.

This comparison includes historical seasons and works independently of the quiet
notification baseline. It compares distinct season/episode pairs, so duplicate
copies do not inflate coverage and explicitly numbered ZIP members count. Counts
refer to aired episodes, including currently running seasons, not a guarantee that
a whole season has finished airing. Specials, unknown dates and future episodes
are excluded.

**Needs review** contains titles with missing/current-identity schedule data,
conflicting metadata, unrecognized local numbering, unconfirmed files, a source
scan older than three hours, or a source not in its ready state. Anime season
ordering always requires review because local/TVDB and TVmaze season structures
can differ. Those titles do not produce definitive missing/partial claims.
Refresh schedules or rescan the source as appropriate; changing data is reflected
by the comparison without generating historical Telegram alerts.

### Upcoming episodes and season premieres

Upcoming releases use calendar-style cards with a scheduled day/date, optional
reported time, small series poster and provider attribution. Regular-season episode 1
is labeled **Season premiere**; other episodes are labeled **Upcoming episode**.
They are visually separate from missing-season coverage cards and make no local-copy
claim. Date-only entries show the provider date with **Date only**, never an invented
air time. Unknown dates remain excluded from the scheduled list.

The first successful fetch establishes a silent baseline: old episodes generate no
alerts. A later reported air time creates a deduplicated episode event; episode 1
of a regular season also creates a season event, coalesced into one Telegram message.
Specials do not announce a new season. Unknown dates generate no event; date-only
records become due at the end of the reported UTC day. Changing provider identity
resets the baseline and cancels old pending alerts. Unfollowing cancels pending alerts.
A reported air date does not confirm a local copy or streaming availability.

## Telegram

Save `{"bot_token":"...","chat_id":"..."}` in a private mode-600 JSON file and
set `LOOKOUT_TELEGRAM_FILE` to its absolute host path. Recreate the app, then use
Releases → Verify bot/chat. Verification calls only getMe/getChat/getChatMember.
There is no test-send endpoint. Automated send tests use mock servers exclusively.

Delivery defaults off. Set `TELEGRAM_DELIVERY_ENABLED=true` and recreate to enable
genuine future notifications after verification and the silent schedule baseline.
Events observed while delivery is disabled are held without later historical replay.
The persistent outbox retries failures; a network failure after Telegram accepted a
message can still cause a duplicate because Telegram offers no idempotency key.

Keep this unauthenticated app on your trusted LAN or behind authenticated access.
Its write endpoints enforce JSON and same-origin browser requests, but LAN users
with access can manage Lookout settings. No downloader or public-video adapter is
implemented in this library milestone.

## Multiple servers and availability

Use **Servers** to rename a logical server and set its scan interval (1–168 hours)
and stale threshold (at least the scan interval, at most 720 hours). Defaults are
six-hour scans and 24-hour freshness. User names survive restart/import and appear
in badges, filters, detail pages and Wanted notifications. Suggested names come
from Jellyfin/Plex when available, otherwise the hostname. A successful unchanged
inventory scan refreshes freshness; credential verification does not.

The existing `ssh`/`roots` configuration remains supported as server `legacy`.
To add independent SFTP hosts, replace it with a `servers` array:

```json
{
  "servers": [
    {
      "id": "home", "name": "Home NAS",
      "ssh": {
        "host": "home.example.lan", "port": 22, "user": "media-reader",
        "key_file": "/run/secrets/ssh-key",
        "known_hosts_file": "/run/secrets/ssh-known-hosts"
      },
      "roots": [
        {"id":"home-films","label":"Films","path":"/media/Films","category":"movie"}
      ]
    },
    {
      "id": "family", "name": "Family NAS",
      "ssh": {
        "host": "family.example.lan", "port": 22, "user": "media-reader",
        "key_file": "/run/lookout/family-key",
        "known_hosts_file": "/run/lookout/family-known-hosts"
      },
      "roots": [
        {"id":"family-films","label":"Films","path":"/media/Films","category":"movie"}
      ]
    }
  ]
}
```

Keep your existing four root IDs and paths when converting the current setup, and
use `legacy` for its server ID. Source IDs are globally unique; identical absolute
paths are allowed on different servers. Mount each existing key/known-hosts file
read-only at the configured container path. Never weaken file permissions or host
verification. SFTP credential setup remains private-file configuration; the Servers
screen edits names and timing, not keys. Recreate the app after configuration changes.

For **Jellyfin/Plex only**, save a connection with no path mappings and choose
“Create a separate server.” Lookout imports real movie and episode entries through
read-only APIs without SSH. A series record alone does not establish season coverage.
API-reported paths are displayed when supplied; opaque stable item IDs are used when
paths are hidden. These are inventory observations, not a media download/player.

For **SFTP plus Jellyfin/Plex on the same storage**, select the existing server and
add explicit path mappings. The API enriches the SFTP observations without creating
another set of copies. Linking requires mappings; hostname resemblance is never
used to infer shared storage. Existing mapped connections migrate to the existing
server. Failed imports retry automatically with a one-hour minimum backoff.

One catalogue card groups compatible provider identities across sources, within
movie/series namespaces. All shared IDs must agree; matching names alone never
merge titles. Overview and season comparisons have an All servers / selected server
filter; detail pages list per-server copies, seasons and numbered files. Copies
across servers contribute to combined episode coverage. Title detail → Title identity
supports explicit merge by another title’s ID and splitting individual evidence back
out; original source records, file paths and links remain intact. Merging keeps a title followed if any copy was followed; changing its follow setting affects the whole group. Pending history and delivered-event receipts survive merging/splitting. Compatible provider IDs
and artwork enrich the representative. If any merged copy was followed, the unified
title stays followed; changing its follow switch affects all copies. Durable provider
event receipts prevent a merge/split from replaying already delivered alerts. Provider identity
conflicts cannot be manually overridden by a merge.

Last known copies remain visible after failures. Freshness is checked per inventory
source, not inferred from another adapter’s successful request. Stale, failed or
unconfirmed evidence is uncertain, never proof of absence. Servers without relevant
configured categories do not block an unrelated movie/series Wanted result.

## Tracking

**Watchlist** accepts a title or public HTTPS link. Select a library suggestion
to reuse its identity, or preview a link and confirm the title and episode scope
before saving. Try `Severance season 1` or `Severance S01E09` to preselect a
scope. The season and episode fields suggest known names and availability
and accept numbers you enter manually. A tracked title can exist without a library match. With public
metadata enabled, TVmaze provides schedules and resolves TVmaze and supported
TheTVDB links. Individual YouTube videos can fill in a title. Other URLs can
be saved with a manually entered title.

Watchlist cards reuse the Overview artwork and server tags. A server tag limits
tracking to that server's files. You can select a server before searching or
change it in a title's tracking settings. The stop icon pauses tracking while
keeping the title and its settings. Trash removes it from the Watchlist.

**Releases** shows upcoming and aired episodes alongside their observed library
availability. The Watchlist preview lists the expected episodes and the servers
that have each copy. A successful, fresh scan can confirm availability. Stale,
ambiguous or failed evidence leaves availability uncertain.

Adding a title establishes a baseline for copies already observed. Choose
Telegram alerts when an episode airs, when new episodes arrive on your servers,
or once a finished season is available in full. Series episode arrivals are
deduplicated across servers. A full-season alert can count copies from multiple
servers. If the provider lacks a complete season schedule, a manual final
episode count can be used for a specific finished season.

Delivery uses the existing Telegram switch and retry handling. Events observed
while delivery is paused are retained as held. Telegram can still duplicate a
message if it accepts it but the network loses its response.
