# Dependency baseline — 2026-11-14

Verified directly against Go releases, the Go module proxy, npm's `latest` tags,
GitHub release APIs and the official PostgreSQL Docker image tags.

| Component | Version |
| --- | --- |
| Go | 1.27.4 |
| pgx/v5 | 5.13.0 |
| Node stable | 26.13.1 (27 entries were alpha prereleases) |
| npm (production build tool) | 12.2.2 |
| PostgreSQL (disposable tests) | 18.7 (19 tags were release candidates) |
| Svelte | 5.57.3 |
| Vite | 8.3.8 |
| Svelte Vite plugin | 7.3.1 |
| svelte-check | 4.7.8 |
| TypeScript | 6.0.3; user-approved compatibility exception to latest 7.0.2 |
| @tendstead/ui | 0.1.2 |
| Vitest | 5.1.0 |
| Playwright package and browser image | 1.65.1 |
| Prettier / Svelte plugin | 3.10.1 / 4.1.1 |
| @types/node | 26.6.6 |
| govulncheck | 1.9.0 |
| actionlint | 1.7.12 |
| actions/checkout | 7.0.1, release commit pinned |
| actions/upload-artifact | 7.0.2, release commit pinned |

Sources: [Go](https://go.dev/dl/), [npm registry](https://registry.npmjs.org/),
[Node releases](https://nodejs.org/dist/index.json),
[PostgreSQL image](https://hub.docker.com/_/postgres),
[checkout releases](https://github.com/actions/checkout/releases),
[upload-artifact releases](https://github.com/actions/upload-artifact/releases),
[Dependabot ecosystems](https://docs.github.com/en/code-security/reference/supply-chain-security/supported-ecosystems-and-repositories).

`go get -u ./...` resolved the latest transitive versions available at setup;
frontend lockfiles retain upstream compatibility constraints. `npm audit` and
`govulncheck` run in the full check. "Latest" describes this verified baseline,
not an immutable promise about future upstream releases.

## Audit result

The foundation check found no Go vulnerabilities. npm reported two low-severity
build-tool findings for `deepmerge` and its parent `@sveltejs/vite-plugin-svelte`
([GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4)), with no
fix available at verification time. Both are development/build dependencies;
the production container has no npm packages or Node runtime. The audit still runs
on every full check, failing at high severity. No advisory was suppressed.
