# Foundation API

The [OpenAPI document](openapi.yaml) describes this milestone's read-only endpoints.
Responses are JSON with `Cache-Control: no-store`. GET and HEAD are supported;
unsupported methods on known endpoints return 405 with an Allow header.

| Endpoint | Success | Failure |
| --- | --- | --- |
| `/healthz` | 200 `{"status":"ok"}` | Process cannot respond if stopped. |
| `/readyz` | 200 `{"status":"ready"}` | 503 `{"status":"unavailable"}` after a database ping timeout/failure. |
| `/api/v1/status` | 200 `{"service":"lookout","stage":"foundation"}` | No database details exposed. |

Unknown `/api` or `/api/*` paths return HTTP 404 with
`{"error":{"code":"not_found","message":"Not found"}}`; they never return SPA HTML.
Unsupported methods use error code `method_not_allowed`. Traversal paths are
rejected with HTTP 400 and code `invalid_path`.

No credentials, absolute paths or connection strings are returned. There are no
configuration writes or catalogue APIs yet. Administrative authentication, scoped
integration tokens, pagination and event contracts belong to subsequent milestones.
