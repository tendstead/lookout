# API

See the [OpenAPI endpoint reference](openapi.yaml). The app has no login and must
be limited to trusted users. Writes require POST, `Content-Type: application/json`
and a same-origin browser request. Actions with no parameters take `{}`. Queued
work returns 202; a busy/disabled worker returns 409. Errors have the form
`{"error":{"code":"...","message":"..."}}`, without credentials or upstream bodies.

| Endpoint | Method | Result / input |
| --- | --- | --- |
| `/healthz` | GET, HEAD | Process liveness. |
| `/readyz` | GET, HEAD | Database readiness; 503 when unavailable. |
| `/api/v1/status` | GET, HEAD | Service identity; legacy stage identifier `foundation`. |
| `/api/v1/titles` | GET, HEAD | `{items,total,limit,offset}`; `q`, `category`, `server` (stable server ID), `limit` (1–100, default48), `offset`. |
| `/api/v1/titles/suggestions` | GET, HEAD | Local-only `{items:[{id,name,category,year?}]}`; `q` (2–300 characters), optional `category`, `server`; at most 8 observed/stable titles. |
| `/api/v1/titles/{id}` | GET, HEAD | Title metadata and observed files. |
| `/api/v1/servers` | GET, HEAD | `{items:[{id,name,scan_hours,stale_hours,status,last_success}]}`. |
| `/api/v1/servers/{id}` | POST | `{name,scan_hours,stale_hours}`; editable display name and inventory timing. |
| `/api/v1/wanted` | GET, HEAD | `{items}` with aggregate status and per-server matching content. |
| `/api/v1/wanted` | POST | `{name,category,year?,provider_ids?,season?,episode?}`; category movie/series; quiet baseline. |
| `/api/v1/wanted/{id}/remove` | POST | Remove request and its pending arrivals; `{}`. |
| `/api/v1/titles/{id}/identity` | POST | `{target_id}`; merge into compatible title, or empty target to split. |
| `/api/v1/sources` | GET, HEAD | Configured sources and scan health. |
| `/api/v1/sources/{id}/scan` | POST | Queue read-only scan; `{}`. |
| `/api/v1/connections` | GET, HEAD | Redacted connections, capabilities and worker progress. |
| `/api/v1/connections` | POST | Save encrypted connection; `{kind,url,token,path_mappings,server_id?}`. |
| `/api/v1/connections/{id}/verify` | POST | Read-only credential/server check; `{}`. |
| `/api/v1/connections/{id}/import` | POST | Queue API inventory or mapped metadata import; `{}`. |
| `/api/v1/metadata/official` | POST | Queue consent-gated TVmaze refresh; `{}`. |
| `/api/v1/titles/{id}/metadata/search` | GET, HEAD | TVmaze candidates; optional `q` (≤300 characters). |
| `/api/v1/titles/{id}/metadata` | POST | Manual identity: `{"provider":"tvmaze","id":"..."}`. |
| `/api/v1/titles/{id}/artwork` | GET, HEAD | Bounded local image proxy/cache, ETag support. |
| `/api/v1/releases` | GET, HEAD | `{data,progress}`: follows, unmatched titles, upcoming episodes and recent events; `limit`, `offset`. |
| `/api/v1/releases/gaps` | GET, HEAD | `{items,total,missing,partial,review,limit,offset}`; `kind` = `missing` (default), `partial`, `review`; optional `group_by=title`; `q`, `category`, `server` (stable server ID), `limit`, `offset`. |
| `/api/v1/releases/refresh` | POST | Queue schedule refresh; `{}`. |
| `/api/v1/titles/{id}/follow` | GET, HEAD | Follow and baseline state. |
| `/api/v1/titles/{id}/follow` | POST | `{"enabled":true}` or false. |
| `/api/v1/telegram/status` | GET, HEAD | Configuration/verification/delivery booleans; no secrets. |
| `/api/v1/telegram/verify` | POST | Bot/chat verification only; `{}`; never sends a message. |

Category values are `movie`, `series`, `anime_movie`, `anime_series`. Source roots
and relative media paths are visible to authorized network users. `file_count`
counts catalogue media entries, including ZIP members. Each file's `storage` is
`file` or `zip`; archive members have the physical ZIP `relative_path` and separate
`archive_member`. Observations and numbering are distinct from provider metadata.

Connections support `kind` = `jellyfin` or `plex`. Each path mapping contains
`server_prefix`, `source_id`, `source_prefix`; see [setup](library.md). Responses
never contain tokens, SSH keys, database connection strings or encryption keys.

Unknown API routes return JSON 404 instead of SPA HTML. Unsupported methods return
405 with Allow; malformed/oversized requests return 400/413; public metadata calls
return 403 when permission is disabled. JSON uses no-store; artwork uses private
revalidation. There is no Telegram send/test endpoint and no remote-media write API.

Season-gap items contain `title_id`, `title`, `category`, `kind`, `season`,
`latest_aired_season`, `local_seasons`, `aired`, `present`, `missing_episodes`,
`reasons`, `checked_at`, `inventory_at`. Review items have no definitive season
claim (`season`, `aired`, `present` are zero); `reasons` explains uncertainty.
Counts `missing`/`partial` refer to seasons and `review` to titles for the current
search/category, before kind selection and pagination. Comparisons read one
consistent database snapshot and never create events or notifications.

With `group_by=title`, filtering is followed by grouping by stable title ID before
pagination. Each missing/partial item includes ordered `seasons` records with
`season`, `kind`, `aired`, `present`, `missing_episodes`. Top-level `season` is zero,
`aired`/`present` are sums over grouped seasons, and `missing_episodes` is empty
because episode numbers are season-specific. `total` counts series cards, while
`missing`/`partial` still count seasons and `review` still counts titles. Review
items remain one per title. Omitting `group_by` preserves flat per-season output;
unsupported values return 400. Titles with the same display name retain separate IDs.

Suggestion queries use only the local catalogue, never a metadata-provider search.
Shorter queries return an empty list. Category/query validation matches title
browsing; suggestions force limit 8 and offset 0 regardless of requested pagination.
Only titles with at least one observed/stable file are eligible, including numbered
ZIP inventory members. Missing/unconfirmed-only records remain visible in ordinary
library browsing, but are excluded from suggestions. Responses contain no artwork
URLs, provider credentials or remote-search data.

Unified title results have `servers:[{server_id,name,status,last_success,files,seasons}]`.
Detail results additionally include `members` (original source-local title IDs), and
files have `server_id`, `server_name`, `source_id`, `adapter`, optional `reported_path`.
Old source-title links resolve to their current canonical title. Identity projection
never deletes original inventory evidence. Availability states are `available`,
`missing`, `uncertain`; server health additionally uses ready/scanning/stale/error/pending.
Wanted POST accepts only movie/series categories; an episode requires a season and
only episodic requests accept season/episode. Provider IDs use imdb/tmdb/tvdb/tvmaze/anidb.
