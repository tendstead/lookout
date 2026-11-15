# Development and checks

Build and test tools run in bounded Docker containers. `make build` uses a dedicated
BuildKit builder with an enforced 2 GiB RAM/no extra swap and 2 CPU budget, verifies
its effective Docker settings before building, and stops it after the build. Its
cache is retained for subsequent builds. `compose.builder.yaml` carries the BuildKit
image version for Dependabot. The global/default builder is not changed. No GitHub repository is
needed for local development. The Go module's future repository path does not
require publishing it before use.

```sh
make deps          # locked frontend install and Go module tidy
make fmt           # Go and frontend formatting
make check         # format, vet, race/unit, DB/browser tests, build, audits, workflow lint
make service-test  # disposable PostgreSQL migration tests
make e2e           # disposable app+PostgreSQL, desktop/mobile and lifecycle tests
make build
```

Tests use unique Compose projects with disposable PostgreSQL storage, never the
server configured in `.env`. Database tests without `TEST_DATABASE_URL` skip under
plain `go test`; `make check` always runs the actual service tests. Test cleanup
removes only that project's containers/volumes. Build and dependency caches persist
in the `lookout-tools_*` Docker volumes. Do not run `make check` concurrently with
another Lookout check against the same checkout.

Go packages: `internal/config` validates environment settings; `internal/store`
owns pooling/migrations; `internal/httpapi` owns routing, health and embedded-asset
boundaries. `cmd/server` handles startup/shutdown and assets. Vite builds the Svelte
frontend into the Go embedded directory; Node is absent from the final image.

`make e2e` covers 390px/mobile and desktop layouts, language/theme persistence,
API failure, missing API routes, database outage, rejected startup and graceful
shutdown. `make screenshots` refreshes the two README images from the running local
instance; it also runs the browser tests. Use `BASE_URL` to select another instance.
Screenshots show library names/artwork or release schedules, never credentials or file contents.

## Dependencies and updates

See [dependency versions](dependency-versions.md) for the registry-verified baseline.
Dependencies are locked for repeatable builds. TypeScript 6.0.3 is the explicit
user-approved compatibility exception: latest TypeScript is 7.0.2, while latest
svelte-check 4.7.8 declares support for TypeScript 5/6. No force/legacy peer flags.

`.github/dependabot.yml` configures weekly Go, npm, Docker, Docker Compose and GitHub
Actions updates; major changes remain separate from grouped npm/Go minor/patch
updates. It becomes active once the file reaches the remote default branch. The
GitHub checks workflow uses release-verified checkout v7.0.1 and upload-artifact
v7.0.2 commit pins, with read-only permissions and credentials persistence disabled.
There is no publishing workflow or automatic release in this milestone.

`@tendstead/ui` 0.1.2 lacks translated shell controls. The original MIT-licensed
package receives a small version-checked postinstall patch adding a reactive locale
prop. Its MIT notice remains in the distribution. An upstream package update makes
this patch fail deliberately until reviewed; remove the patch when native shell
translations become available. Lookout does not modify the sibling UI repository.

Browser runtime failures are collected in `web/test-results`. No external metadata
provider credentials, NAS access, downloader or production-library fixtures are
required for automated checks. Real Telegram verification uses only getMe/getChat/getChatMember;
notification sends are tested against mock HTTP servers with disposable database state.
