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
**Tracking** combines a Watchlist with Releases. Add a film, series, season or
episode by title or public link, then see its schedule and server availability.
Choose alerts for airing, arrival on your servers, or a complete season.
Scans default to six hours, with a configurable 24-hour stale threshold; offline
or stale inventory stays visible as uncertain rather than being treated as missing.

Lookout does not download, play, move, rename or delete media.

## Run

Requires Docker Engine with Compose/Buildx, Make, and an existing PostgreSQL server with a dedicated Lookout database and login role.

[Configure Lookout](docs/configuration.md) with one `lookout.toml` file. Pick Jellyfin, Plex, SFTP, or SSH for each server. The TOML deployment runs with a read-only web UI and no application environment variables.

Existing environment-based installations can follow the [migration instructions](docs/configuration.md#move-an-existing-deployment). Lookout has no application login and is intended for a trusted network.

## Screenshots

Lookout shown with demo data.

| Desktop | Phone |
| --- | --- |
| ![Lookout desktop library with fictional titles and cover artwork](docs/screenshots/desktop.png) | ![Lookout on a phone showing episode gaps in a demo season](docs/screenshots/mobile.png) |

Configure [read-only library and integrations](docs/library.md). See [deployment](docs/deployment.md), [API](docs/api.md), and
[development and checks](docs/development.md) for more details.

## License

[Elastic License 2.0](LICENSE) (`Elastic-2.0`). Third-party components retain their
own licenses; see [third-party notices](docs/third-party/README.md).
