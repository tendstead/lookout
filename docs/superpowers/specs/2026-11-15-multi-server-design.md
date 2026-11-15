# Multi-server catalogue and Wanted

## Intent and agreed scope

Lookout answers whether a movie or episode exists on any connected server before
the user buys it. Support SFTP-only, Jellyfin-only, Plex-only and explicitly linked
SFTP/media-server connections. One shared catalogue entry has independent inventory
evidence per server. Never infer that two hosts share storage from their names.

Server display names are editable in Lookout and used in cards, filters, details
and Wanted notifications. Suggested initial names use the media server name when
available, otherwise the hostname. User names survive imports/restarts.

Scans default to six hours; stale after 24 hours without a successful complete
scan, including an unchanged scan. Both are configurable per server. Errors,
partial scans and suspicious inventory reductions never establish absence.
Retain last known copies and expose freshness. Stale/offline means uncertain,
not missing. Independent bounded workers prevent one unavailable host blocking
others. SFTP remains read-only: no shell, writes, full video reads or hashing.

Overview/search show unified titles with server badges and a server filter.
Details list copies and season/episode coverage by server. Missing-season views
combine coverage by default and support a selected server. Trust provider IDs
within movie/series namespaces, reject conflicting IDs, and offer explicit
merge/unmerge review for ambiguous filename matches. Do not silently merge by name.
Keep original title IDs/evidence so existing follows, metadata and links survive.

Wanted supports movies and series, optional year/provider IDs, whole-title,
season or episode targets. Users can add absent titles without a public search.
Show available, missing or uncertain with per-server evidence. Durable deduplicated
arrival notifications identify the server and newly present requested content.
Adding an already available wanted item shows availability without sending a
historical arrival message. Subsequent new server/content arrivals generate events;
unchanged scans/restarts do not. Respect Telegram's existing delivery switch;
test sending only against mocks, keep real delivery disabled.

## Approaches

Chosen: a shared catalogue projection over preserved source titles and file
observations, explicit logical servers with separate adapters. This preserves
existing evidence and allows reversible identity decisions. Separate catalogues
per host would repeat cards and cannot answer combined coverage. Replacing all
existing title IDs would unnecessarily endanger follows and existing links.

## Migration and constraints

Migrate the existing legacy SSH configuration to one logical server without
changing source/title IDs, paths, keys, tokens, metadata or release baselines.
Legacy mapped Jellyfin remains metadata-only until explicitly linked; do not
invent a second inventory for its existing mapped files. Unmapped new media
connections can create their own inventory without requiring SFTP.
Existing PostgreSQL, Go, Svelte and TypeScript remain; no new dependencies planned.
Keep EL2, current third-party notices, resource bounds and phone/LAN access.
No remote publication, purchases, media transfer or store integration.

## Builder decisions

Server IDs are stable strings; adapter credentials stay in existing protected
configuration/encrypted storage. SFTP setup initially uses private configuration
and mounted key/known-hosts files, with safe inventory settings editable in UI.
Scan metrics include duration and counts; transferred bytes must be described as
measured only where the transport can actually count them. Unknown identity or
numbering cannot produce a definite purchase recommendation.

The user explicitly requested implementation after the design discussion; proceed
without another approval round, recording further routine choices in the ledger.

## Builder review resolutions

Freshness belongs to each source; verifying an API cannot refresh SFTP inventory.
Canonical identities govern browsing, follows, release scheduling and outbox claims.
Wanted whole-series means any episode, season means any episode in that season,
and episode means the exact explicit pair; completeness remains a separate schedule
comparison. Initial known copies are baselined; newly connected servers may announce
previously created requests. Only sources in the relevant category affect absence.
API-only movie/series scopes are explicitly scanned even when empty.
