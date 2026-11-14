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
