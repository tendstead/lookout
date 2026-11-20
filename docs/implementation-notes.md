# Foundation execution notes — 2026-11-14 (historical)

The library milestone followed this foundation. See [integration setup](library.md) for current behavior.

## Implementation decisions

- Work in the new isolated feature repository rather than create another worktree:
  no prior implementation or main branch existed. No interference with sibling work.
- User authorized LAN Compose startup and screenshots after the original local-only
  plan. Port 8083 is bound on the LAN; trusted network users can view the status UI.
- Latest stable releases verified via official registries. TypeScript 6.0.3 is an
  explicit user-approved exception because latest svelte-check excludes TypeScript 7.
- Use static Vite/Svelte for this two-view shell instead of SvelteKit. This reduces
  the foundation's dependencies; more complex routing may need a later decision.
- Apply a version-checked local MIT-preserving locale patch to @tendstead/ui 0.1.2
  so shell controls translate reactively. Package upgrades require reviewing or
  removing the patch; sibling UI source was not modified.
- Original code is EL2 as requested. Generated dependency notices are carried in
  the image to retain third-party licensing without relabeling that code.

## Verification and deployment findings

Full `make check` passed: formatting, Go vet and race/unit tests, real PostgreSQL
migration tests, Svelte/TypeScript checks, eight desktop/mobile browser cases,
production build, database-outage and startup/shutdown checks, vulnerability audits
and actionlint. A clean committed-source export also passed Go tests without any
frontend files generated beforehand. Startup assets retain a tracked empty marker
for that case. The same eight browser cases passed through the live LAN URL.

The browser harness uses `lookout.test`; Chromium forces the bare hostname `app`
to HTTPS because `.app` is HSTS-preloaded. That was a harness-origin issue, resolved
without changing application HTTP behavior.

Compose preserves host-file permissions on local secret bind mounts. Verified an
EACCES failure as UID 65532, then used a private mode700 parent plus a mode644
secret file so the nonroot app can read its mount. See the deployment guide.

No Go vulnerabilities were reported. Two low-severity build-tool npm advisories
have no available fix and remain visible in audits; see dependency-versions.md.

Only the dedicated local Lookout database/role was provisioned on the shared
server. Tests used disposable databases. No NAS media, sibling applications,
GitHub repository, publication or release was changed.

## Final review

A fresh read-only review found two Important issues. Both were reproduced before
fixing: database URL query passwords could override the password file, and ordinary
Docker builds had no enforced build-stage resource budget. Configuration now rejects
query/userinfo password conflicts (including encoded query keys and invalid query
encoding), and the build entry point verifies a dedicated BuildKit container's
effective CPU/memory/swap limits before building.

Deferred minor: the concurrent migration test exercises the migration lock on an
already-initialized schema, rather than synchronizing concurrent first-ever schema
creation. The implementation serializes all migration initialization under the same
transaction lock; a fresh-schema contention fixture can strengthen future coverage.
