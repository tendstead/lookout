# Upcoming cards, shared layout width and local suggestions

User approved visual upcoming releases, the same fluid card sizing on Overview and Releases, an ultrawide width-limiting button saved only in this browser, and live search suggestions. User clarified suggestions include all local titles (films, series and anime), never external discovery.

Upcoming releases use date-led cards with smaller series artwork, a visible scheduled date/time, and Upcoming episode or Season premiere labeling. A premiere means regular season>0 episode1. They do not use missing-season coverage controls or claim local availability. Existing cached schedules, date-only semantics and provider attribution remain intact.

Overview adopts the Releases340px-minimum auto-fill grid. Both gallery pages use available width by default; a shared Limit width toggle caps content at1600px (about4cards on wide screens). Save the boolean in localStorage only, safely falling back to in-memory state when unavailable. Phones remain width-safe. Other pages retain their current width.

Search uses a debounced keyboard-accessible combobox of up to8local title suggestions, respecting the active category. It queries only the Lookout database and requires observed/stable local-file evidence. Missing/unconfirmed-only records are excluded. No provider searches or artwork requests are triggered by typing. Cancel/ignore obsolete requests, support arrows/Enter/Escape, pointer selection, outside dismissal and a usable submitted search if suggestions fail.

Artwork answer verified from owning source: browser images always use Lookout's local artwork endpoint. Cache misses fetch from configured Jellyfin/Plex or approved TVmaze artwork, then cache bytes in the dedicated PostgreSQL database. Cached images require no upstream request; the whole library is not predownloaded. This change keeps that policy unchanged.
