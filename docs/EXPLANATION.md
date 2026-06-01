# How anilistgen works

## Architecture

```
cmd/anilistgen/main.go    — CLI entry: fetch → filter → resolve → output
internal/anilist/            — AniList GraphQL client (retry, pagination)
internal/config/             — YAML config loading and validation
internal/mapping/            — TVDB ID resolution from local mapping files
internal/filter/             — Show filtering (duration, blacklist, tags, future)
internal/output/             — Compact Sonarr JSON generation
internal/logging/            — slog setup
```

## Pipeline

For each configured year:

```
AniList API
    │
    ▼
  Fetch year — paginated GraphQL query (no season filter, max 50 per page)
    │
    ▼
  Bucket by season — split results by AniList's `season` field
    │                 (WINTER / SPRING / SUMMER / FALL)
    │
    ▼
  Winter overflow — if WINTER in seasons, merge December-premiering
    │                 shows from the prior year (fetched if not in config)
    ▼
  Filter — for each season, remove shows with:
    │         • Duration ≤ 10 min per episode
    │         • Blacklisted MAL ID or title
    │         • Excluded content tag
    │         • Start date > N months ahead
    ▼
  Resolve — map each show to a TVDB ID:
    │         Community mapping YAML (MAL ID → TVDB)
    │
    ▼
  Output — write compact JSON:
              • Per season: winter-2026.json
              • Yearly aggregate: 2026.json
```

## Data source

**[shinkro/community-mapping](https://github.com/shinkro/community-mapping)**
(`tvdb-mal.yaml`, ~947KB, ~5,241 entries).

Maps MAL IDs directly to TVDB IDs. Downloaded on first run and cached
locally. Covers ~78% of seasonal anime.

### Resolution

```
Show from AniList → MAL ID → Community mapping → TVDB? → Done

Not matched → silently skipped (not yet in TVDB)
```

No external API calls during resolution — the mapping file is local.

## Winter overflow

AniList tags WINTER shows by **calendar year**. A show that premieres in
December 2025 is tagged as WINTER 2025 — not WINTER 2026 — even though
it's only a few weeks before the January premieres.

With `winter_overflow: true`, the tool fetches the prior year (if not
already configured) and merges any shows with a December start date into
the current year's WINTER bucket.

| Config | WINTER 2026 includes |
|---|---|
| `winter_overflow: true` (default) | Jan–Feb 2026 + December 2025 premieres |
| `winter_overflow: false` | Jan–Feb 2026 only |

## Output format

Both files are bare JSON arrays — Sonarr's Custom List import requires
a top-level array, not a wrapped object.

**`winter-2026.json`** (and `2026.json`):
```json
[{"tvdbId":377543,"title":"..."},{"tvdbId":424536,"title":"..."}]
```

JSON is minified (no whitespace). Sonarr reads `tvdbId`; `title` is
cosmetic.

## Key design decisions

- **No MDBList** — v2 replaces the MDBList API with a local community
  mapping file. No API keys, no rate limits, no external service dependencies.

- **Static files only** — Output is JSON on GitHub Pages. No server needed.
  Sonarr imports directly from the URL.

- **Weekly sync** — AniList seasonal data doesn't change daily. Weekly is
  sufficient and stays well within AniList's rate limits.

- **Year-level fetching** — Instead of one API call per season, the tool
  fetches the full year in a single query and splits internally. This
  reduces API round-trips by ~50% and speeds up multi-year backfills.

- **Bucket by season** — AniList returns a `season` field on each show.
  The tool groups by it client-side, then applies the WINTER start-month
  filter (December–March) to the WINTER bucket, matching the prior behavior.

- **Paginated fetching** — AniList caps responses at 50 per page. The
  client follows `hasNextPage` to collect up to `max_per_year` results.

- **Auto-downloaded mapping** — The community mapping file downloads on
  first run and caches locally. No manual setup needed.
