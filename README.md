# Tendstead Lookout

A self-hosted overview for your films, series and public videos.

**Current version: foundation preview.** The app provides a responsive English/German
interface, persistent language/theme preferences and live service/database checks.
Media sources, library browsing, matching, subscriptions and notifications are planned
for the following milestones. No media files are scanned or changed in this version.

## Run

Requires Docker Compose and an existing PostgreSQL server with a dedicated Lookout
database and login role. Follow [database setup](docs/deployment.md), then:

```sh
cp .env.example .env
# Set the database URL, password-file path and database Docker network in .env.
docker compose up -d --build --wait
```

Open <http://localhost:8083>. To use Lookout on a phone, set `LISTEN_ADDR=0.0.0.0`
in `.env`, recreate the app with `docker compose up -d --wait`, and visit
`http://<your-computer-LAN-address>:8083` while connected to the same local network.
Lookout has no application login and is intended for a trusted network.

`docker compose down` stops Lookout; its dedicated database stays on your PostgreSQL
server. Language and theme are saved separately on each device.

## Screenshots

| Desktop | Phone |
| --- | --- |
| ![Lookout desktop overview with live database status](docs/screenshots/desktop.png) | ![Lookout mobile system status with language and theme controls](docs/screenshots/mobile.png) |

See [deployment](docs/deployment.md), [API](docs/api.md), and
[development and checks](docs/development.md) for more details.

## License

[Elastic License 2.0](LICENSE) (`Elastic-2.0`). Third-party components retain their
own licenses; see [third-party notices](docs/third-party/README.md).
