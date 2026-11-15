# Foundation resource observation — 2026-11-14

Development machine, Linux amd64, one Lookout app using the existing external
PostgreSQL server. This is a small foundation observation, not a catalogue benchmark.

| Observation | Value |
| --- | --- |
| Container working-set memory after desktop/mobile browser checks | 4.129 MiB |
| Process RSS / high-water RSS after browser checks | 13,324 KiB / 13,324 KiB |
| Container CPU at that idle sample | 0.00% |
| Current Lookout database connections | 1 (pool maximum 4) |
| App memory / CPU limits | 128 MiB / 1 CPU |
| Go soft memory limit | 96 MiB |
| Verified BuildKit hard memory / swap / CPU budget | 2 GiB / 2 GiB total / 2 CPUs |
| Browser tests against the LAN instance | 8 passed |
| Scan / discovery / downloader peaks | Unmeasured; features not implemented |

Commands: `docker stats --no-stream lookout-app-1`, `docker inspect lookout-app-1`,
and a count of `pg_stat_activity` rows filtered to database `lookout`. The running
container is nonroot, read-only, drops all capabilities and has the configured
memory/CPU limits. Container working-set usage is distinct from process RSS. Process counters were
read from the running Lookout PID’s `/proc/<pid>/status`; the observed high-water
RSS includes startup and browser checks but is not a sampled startup timeline.

The first startup revealed Compose secret permission behavior; after verifying
nonroot readability and correcting the private host-directory/file modes, the
instance reached healthy readiness and responded through the LAN IP. Test-only
PostgreSQL outage and startup/shutdown checks passed on disposable containers.

Before the catalogue milestone: verify direct metadata provider access/coverage,
capture the reported existing-file recognition failure using disposable copies,
and define canonical identity/source-health contracts. No NAS media or public
source polling was used for this baseline.

The dedicated `lookout-builder` container was inspected after its bounded build:
Docker Memory and MemorySwap were both 2,147,483,648 bytes, CpuQuota 200,000 and
CpuPeriod 100,000. It was stopped after building. Runtime and builder budgets are
separate; see `scripts/check-builder-budget.sh`.

## Library follow-up — 2026-11-15

With 435 titles, 9,590 media entries and 11,079 cached episode records, a live idle
sample reported 7.52 MiB container working set and 0.00% CPU. Hard limits remain
128 MiB memory/total swap and one CPU. The integration override runs as nonroot
1000:1000 to read the existing host key; read-only filesystem and mounts remain
active. This sample is not a scan peak measurement or capacity promise.

## Compact overview and current service — 2026-11-15

Current dataset: 435 titles, 9,590 media entries, 11,079 episode cache rows. The app was
sampled during ordinary live browsing/search and screenshot verification, not a
synthetic stress test or a complete source-scan peak measurement.

| Observation | Measured value |
| --- | --- |
| Docker working-set range over 8 samples | 9.648–14.11 MiB |
| CPU across those samples | 0.00% idle; 0.06% and 1.84% activity samples |
| Go process RSS / high-water RSS after browsing | 25,176 KiB / 25,176 KiB (about 24.6 MiB) |
| Dedicated PostgreSQL database | 28,587,711 bytes (about 27.3 MiB) |
| Cached image bytes | 4,297,599 bytes across 114 images (about 4.1 MiB, included in database size) |
| Runtime Docker image, uncompressed | 15,827,796 bytes (about 15.1 MiB) |
| Lookout database connections at sample | 4 (pool maximum 4) |
| App hard memory / total memory+swap / CPU limits | 128 MiB / 128 MiB / 1 core |
| Go soft memory limit | 96 MiB |

Docker working set and process RSS use different accounting; they are not additive.
These app RAM/CPU figures exclude the existing shared PostgreSQL server, browser,
and temporary build/test containers. Database size refers only to Lookout's own
database; the image figure excludes build caches. Runtime limits were inspected,
and the healthy deployed image matched the tested build.

The overview adjustment is presentation-only: actual 1920px checks measured 8 cards
at 184px instead of the previous 4 at 384px; limited mode shows 6 at 173.3px, release
cards remain 4 at 384px, and a 390px phone shows 2 at 169px. Types and all 36 browser cases,
including lifecycle checks, passed; the same 36 cases passed on the live LAN service.
