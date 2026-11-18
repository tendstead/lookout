# Tendstead Lookout

A self-hosted overview for your films, series and public videos.

Lookout combines multiple servers into one media catalogue. Inventory can come
from read-only SFTP, Jellyfin, Plex, or explicitly linked connections to the same
storage. Cards show editable server names; filters and detail pages show where
each movie or episode exists. Optional TVmaze lookups supply release schedules. Collections keep
their existing folders; individual films appear as separate cards. ZIP-contained
videos appear as archive members without extraction.

Follow series/anime for upcoming episodes and provider-reported season premieres.
Poster cards show missing and partial aired seasons, episode coverage and local
season chips. Multiple seasons share one series card, and the grid fits the
available window width. A browser-saved width limit keeps ultrawide galleries
readable. Search suggests local films, series and anime as you type; upcoming
releases have their own calendar-style cards. Uncertain numbering stays separated
for review. The first successful
schedule fetch establishes a silent baseline. Optional Telegram alerts apply to
future releases; verifying the bot/chat sends no test message.
A **Wanted** list tracks movies, series, seasons and episodes across servers.
Arrival notifications identify the server that added the requested content.
Scans default to six hours, with a configurable 24-hour stale threshold; offline
or stale inventory stays visible as uncertain rather than being treated as missing.

Lookout does not download, play, move, rename or delete media.

## Run

Requires Docker Engine with Compose/Buildx, Make, and an existing PostgreSQL server with a dedicated Lookout
database and login role. Follow [database setup](docs/deployment.md), then:

```sh
cp .env.example .env
# Set the database URL, password-file path and database Docker network in .env.
make build
docker compose up -d --no-build --wait
```

Open <http://localhost:8083>. To use Lookout on a phone, set `LISTEN_ADDR=0.0.0.0`
in `.env`, recreate the app with `docker compose up -d --no-build --wait`, and visit
`http://<your-computer-LAN-address>:8083` while connected to the same local network.
Lookout has no application login and is intended for a trusted network.

`docker compose down` stops Lookout; its dedicated database stays on your PostgreSQL
server. Language and theme are saved separately on each device.

## Screenshots

| Desktop | Phone |
| --- | --- |
| ![Lookout desktop library with metadata and cover artwork](docs/screenshots/desktop.png) | ![Lookout mobile upcoming releases with calendar dates and artwork](docs/screenshots/mobile.png) |

Configure [read-only library and integrations](docs/library.md). See [deployment](docs/deployment.md), [API](docs/api.md), and
[development and checks](docs/development.md) for more details.

## License

[Elastic License 2.0](LICENSE) (`Elastic-2.0`). Third-party components retain their
own licenses; see [third-party notices](docs/third-party/README.md).
