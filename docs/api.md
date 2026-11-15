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
| `/api/v1/titles` | GET, HEAD | `{items,total,limit,offset}`; `q`, `category`, `limit` (1–100, default48), `offset`. |
| `/api/v1/titles/{id}` | GET, HEAD | Title metadata and observed files. |
| `/api/v1/sources` | GET, HEAD | Configured sources and scan health. |
| `/api/v1/sources/{id}/scan` | POST | Queue read-only scan; `{}`. |
| `/api/v1/connections` | GET, HEAD | Redacted connections, capabilities and worker progress. |
| `/api/v1/connections` | POST | Save encrypted connection; `{kind,url,token,path_mappings}`. |
| `/api/v1/connections/{id}/verify` | POST | Read-only credential/server check; `{}`. |
| `/api/v1/connections/{id}/import` | POST | Queue path-mapped metadata import; `{}`. |
| `/api/v1/metadata/official` | POST | Queue consent-gated TVmaze refresh; `{}`. |
| `/api/v1/titles/{id}/metadata/search` | GET, HEAD | TVmaze candidates; optional `q` (≤300 characters). |
| `/api/v1/titles/{id}/metadata` | POST | Manual identity: `{"provider":"tvmaze","id":"..."}`. |
| `/api/v1/titles/{id}/artwork` | GET, HEAD | Bounded local image proxy/cache, ETag support. |
| `/api/v1/releases` | GET, HEAD | `{data,progress}`: follows, unmatched titles, upcoming episodes and recent events; `limit`, `offset`. |
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
