# Local library verification — 2026-11-15

The four requested NAS roots were read without moving, renaming, extracting,
writing or deleting NAS content. Existing SSH key/known_hosts files were mounted
read-only; their values and permissions were preserved. This verifies client
operations, not a server-side read-only restriction on the existing account.

| Root category | Ordinary video files | ZIP video members | Title cards |
| --- | ---: | ---: | ---: |
| Series | 6,937 | 52 | 72 |
| Films | 279 | 0 | 279 |
| Anime films | 25 | 0 | 25 |
| Anime series | 2,297 | 0 | 59 |
| Total | 9,538 | 52 | 435 |

An independent scoped read-only `find -P` inventory matched the SFTP set exactly:
zero missing/extra ordinary video paths. The app itself uses SFTP, not a shell.
A separate ZIP-directory audit found 52 video members; bounded scanner inspection
then imported those members as distinct, repeatable catalogue entries. The ZIP
was neither extracted nor rewritten. Other files such as subtitles/artwork are not
counted as videos. Non-XML NFO files generated nonfatal warnings; no valid NFO
metadata was available in this scan.

Jellyfin supplied metadata for 430/435 local titles, with zero identity conflicts;
three upstream records outside the observed/mapped scope were skipped. Both
Jellyfin and Plex adapters have mock-server coverage, while the user's live
connection is Jellyfin. The container used the explicitly authorized direct LAN
address because the host's local reverse-proxy hostname was unreachable inside it.

TVmaze resolved 116/131 episodic titles (64 series, 52 anime series). The remaining
15 retain their library entries and available Jellyfin metadata and are listed
for manual matching. Schedules cache 11,079 episode records, including 3,435 anime
episodes; nine future entries were reported at baseline. No future anime dates
were supplied at that time. Positive anime/series episode and season-premiere
transitions were tested with controlled clocks, independent of live schedules.

The initial fetch produced zero release events/outbox messages. Repeat polling,
restarts, date correction, specials, unknown dates, provider changes, unfollowing,
coalescing and retry behavior have automated coverage. Real Telegram verification
used getMe/getChat/getChatMember only. Real messages sent: zero. Optional future
delivery remains disabled; enable it as described in [library setup](library.md).

The full check covers Go vet/race/unit tests, disposable PostgreSQL migrations and
release/outbox tests, mock Telegram delivery, Svelte/TypeScript, 16 desktop/mobile
browser cases, production build, startup/shutdown/database outage behavior,
govulncheck, npm audit and actionlint. Latest stable dependency pins were rechecked
against official registries on this date; TypeScript 6 is the approved compatibility
exception. Two low build-only advisories have no upstream fix; no Go findings.

A fresh whole-feature review found three Important issues, reproduced by regression
tests: mutable-name pagination skipping metadata targets, startup verification
suppressing genuine notifications, and public artwork fetches ignoring the disabled
public-metadata flag. All three were corrected; a focused follow-up found no
remaining Important findings. Metadata conflict status was also made visible.

Only Lookout's own database/configuration and local repository were written.
No remote repository, push or release was created. README screenshots show the
actual LAN library and mobile release view; secrets and raw inventory evidence
remain in ignored local files.
