# Deployment

Run one Lookout app container. It serves its embedded static interface and uses an
existing PostgreSQL server; production Compose does not create a database service.
The foundation exposes read-only status endpoints and no administrative writes.

## Dedicated database

On your PostgreSQL server, use an administrator connection to create a dedicated
role and database. The following is a template; replace the password locally:

```sql
CREATE ROLE lookout LOGIN PASSWORD '<a-new-random-password>';
CREATE DATABASE lookout OWNER lookout;
REVOKE ALL ON DATABASE lookout FROM PUBLIC;
```

Lookout applies only its own migrations to that database. It must not use a sibling
application's database or administrator credentials. The pool is limited to four
connections. Concurrent startup migrations serialize under a transaction lock;
newer schemas cause startup to fail rather than being downgraded.

## Configuration

Copy `.env.example` to `.env`. Store the role password in a separate ignored file,
for example `.implementation/database-password`. Restrict its parent directory to
your account (`chmod 700 .implementation`) and make the file readable to the
container UID (`chmod 644 .implementation/database-password`). Host users cannot
traverse that private directory. Local Compose secrets are bind mounts and preserve
host permissions; a host file with mode 600 cannot be read by UID 65532 in the
container. Compose mounts the file as a read-only secret;
its contents are not embedded in the image or frontend.

| Variable | Meaning |
| --- | --- |
| `DATABASE_URL` | PostgreSQL URL including user, host and database; omit its password when using the password file. |
| `DB_PASSWORD_FILE` | Host path to the password file for Compose. When running the binary directly, this is the path it can read. |
| `POSTGRES_NETWORK` | Existing Docker network shared with the PostgreSQL container, default `postgres_net`. |
| `LISTEN_ADDR` | Compose host bind address: default `127.0.0.1`; `0.0.0.0` enables LAN access. |
| `PORT` | Compose host port, default `8083`. |

The binary's own `LISTEN_ADDR` includes the port and defaults to `127.0.0.1:8080`.
Compose explicitly sets it to `:8080` inside the container while controlling the
host exposure through the port mapping above.

Use `sslmode=verify-full` with appropriate CA configuration for a database reached
across an untrusted network. The local example uses `sslmode=disable` only for a
trusted private Docker network. A database Docker network can expose other services
to Lookout; retain separate database roles and permissions.

## Access and operation

The current app has no login. Keep it on a trusted LAN, WireGuard, or behind your
existing authenticated reverse proxy. No router port forwarding is needed for LAN
use. HTTP works for this preview; it does not rely on service-worker offline support.

```sh
docker compose up -d --build --wait
docker compose ps
docker compose logs --tail=50 app
docker compose down
```

The container runs as a nonroot user with a read-only filesystem, `/tmp` tmpfs,
dropped capabilities, no-new-privileges and restart-unless-stopped. Initial budgets:
one CPU, 128 MiB memory, no extra swap; Go soft limit 96 MiB. These are provisional
foundation budgets, not promises for later scanning/discovery workloads.

`/healthz` reports process liveness. `/readyz` performs a database ping bounded to
two seconds. Failed database startup exits; a later database outage leaves liveness
working and changes readiness to HTTP 503. Database errors and credentials are not
included in public responses or startup logs. SIGTERM allows ten seconds to stop.

Back up the dedicated PostgreSQL database using your existing PostgreSQL backup
process. Device preferences live in browser local storage. There are no media
mounts or media mutations in this milestone.
