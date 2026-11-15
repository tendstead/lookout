# Library and integrations

SFTP transfers files over an authenticated SSH connection. Lookout uses its native
SFTP client for directory listings, file attributes and bounded sidecar/archive
reads. It verifies the server against an existing known_hosts file, skips symlinks
and rejects paths outside configured roots. It issues no remote shell commands or
write operations. A server account restricted to reading those roots is recommended;
client read-only behavior does not change the permissions of your SSH account.

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
read-only. Recreate with `docker compose up -d --no-build --wait`. Sources scan hourly; use the Sources screen to request the first scan or a rescan. A single worker
serializes scans. Failed, empty or suspiciously reduced scans retain the previous
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

Use Connections to choose either server, save its URL/token, verify it and import
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
alone does not import it; use the CLI or the Connections form.
The host and container must both resolve/reach your chosen URL; a LAN address can
be used when a local reverse-proxy hostname is unavailable inside Docker.

## Public metadata and releases

Set `ALLOW_PUBLIC_METADATA=true` only when you want Lookout to send series/anime
titles and TVDB/IMDb identifiers to TVmaze. This shares names/IDs, not credentials
or media. Connections can refresh official metadata; title detail supports searching
and selecting a manual match. TVmaze attribution links are retained.
Movies currently use local/Jellyfin/Plex metadata; TVmaze covers episodic titles.

Follow individual series/anime in title detail, or use `FOLLOW_IMPORTED_SERIES=true`
to seed follows for imported episodic titles. Schedule caches refresh every six
hours, with due dates checked locally each minute. Releases shows unmatched follows
for manual review rather than implying schedule coverage.

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
