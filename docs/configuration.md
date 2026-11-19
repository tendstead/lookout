# Configure Lookout

Lookout reads one [TOML configuration file](../lookout.example.toml). Comments begin with `#`. Each `[[servers]]` block describes one library server. Passwords and tokens go directly in the file. SSH private keys and verified `known_hosts` entries can sit beside it, with paths relative to the TOML file.

## First setup

Create a dedicated [PostgreSQL database](deployment.md#dedicated-database), then prepare the configuration.

```sh
install -d -m 750 lookout-config
install -m 640 lookout.example.toml lookout-config/lookout.toml
```

Edit `lookout-config/lookout.toml` and set the database URL and password. Replace the Jellyfin example with your server or delete that block and uncomment the SFTP example. Add any referenced key and `known_hosts` files to the directory. Give the container's nonroot group read access. On a standard Docker install the image uses GID 65532, so use `sudo chgrp -R 65532 lookout-config` and give each credential file mode `640`. Keep the directory private and back it up with your database.

```sh
make build
docker compose -f compose.toml.yaml run --rm --no-deps app -check-config
docker compose -f compose.toml.yaml up -d --no-build --wait
```

Open Lookout at `http://localhost:8083`. The Compose file mounts `lookout-config/` read-only and needs no application environment variables. It defaults to the external Docker network `postgres_net`. Override `POSTGRES_NETWORK`, `LISTEN_ADDR`, or `PORT` only if your Docker setup needs different values.

## Add or edit servers

| Type | Minimum setup |
| --- | --- |
| `jellyfin` / `plex` | Server URL and token. Lookout uses the media server's existing movie and episode index. |
| `sftp` | Host, username, a verified `known_hosts` file, one or more library roots, and either `private_key_file` or `password`. The host must enable the SFTP subsystem. |
| `ssh` | The same fields as SFTP. The remote POSIX shell must be able to run `python3` over a non-interactive SSH session. Lookout runs read-only filesystem commands, including bounded reads for NFO sidecars and ZIP directory metadata. |

For filesystem access, use a dedicated account with read permission on the library roots. Key authentication uses an unencrypted private key file. Both SSH modes verify the host key.

Each block controls one inventory source. A filesystem server can optionally include `[servers.metadata]` with a Jellyfin or Plex URL, token, and explicit path mappings for enrichment. It continues to scan the filesystem for availability.

Restart Lookout after editing the file. Removing a server or root stops its scans. Its historical inventory remains in the database and ages past the freshness threshold.

```sh
docker compose -f compose.toml.yaml restart app
```

The Servers page shows each server's configured type, status, scan interval and last successful scan. Connections shows media API and Telegram health. In TOML mode, the web UI and its API are read-only. Wanted entries and follows already in the database remain visible, and scans and notifications run in the background.

## Move an existing deployment

Use the existing database URL and password in the new file. Keep your server IDs and filesystem root IDs, paths, and categories. For a top-level legacy `ssh`/`roots` JSON configuration, use `id = "legacy"` for that server. The existing Servers API lists media-only IDs under `/api/v1/servers`. Use those IDs and put the original Jellyfin or Plex tokens in the corresponding TOML blocks. For an existing mapped connection, use the filesystem server's optional metadata block.

For native installations, `lookout -config /path/to/lookout.toml` accepts an optional top-level `listen_addr` such as `"127.0.0.1:8080"`. Leave it at the default `:8080` for the supplied Compose file.

The new configuration uses its own tokens at runtime. PostgreSQL retains library history, and an existing connection retains its identity when imported under its original server ID and provider. Keep your existing `connection-key` and legacy settings for rollback. If you change a media server URL in TOML, re-enter its token before using that connection in the legacy UI.

Start with `compose.toml.yaml` once the config is ready. The old Compose files and environment-based startup remain available during migration.
