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

For each configured season and year:

```
AniList API
    │
    ▼
  Fetch — paginated GraphQL query (max 50 per page, follows hasNextPage)
    │
    ▼
  Winter overflow — if WINTER season, also fetch prior year's WINTER
    │                 and merge December-premiering shows
    ▼
  Filter — remove shows with:
    │         • Duration ≤ 10 min per episode
    │         • Blacklisted MAL ID or title
    │         • Excluded content tag
    │         • Start date > N months ahead
    ▼
  Resolve — map each show to a TVDB ID:
    │         1. Anime-Lists XML (AniList ID → TVDB)
    │         2. Community mapping YAML (MAL ID → TVDB)
    │
    ▼
  Output — write compact JSON:
              • Per season: winter-2026.json
              • Yearly aggregate: 2026.json
```

## Data sources

### Anime-Lists (primary)

**[Anime-Lists/anime-lists](https://github.com/Anime-Lists/anime-lists)**
(`anime-list-full.xml`, 1.7MB, ~10,688 entries).

Maps AniList internal IDs to TVDB IDs. Downloaded on first run, cached
locally. Used as the primary resolution path.

### Community mapping (secondary)

**[shinkro/community-mapping](https://github.com/shinkro/community-mapping)**
(`tvdb-mal.yaml`, 947KB, ~5,241 entries).

Maps MAL IDs directly to TVDB IDs. Covers ~78% of seasonal anime.
Used as fallback when anime-lists doesn't have an entry.

### Resolution order

```
Show from AniList
  │
  ├── Anime-Lists (local XML, instant)      → TVDB? → Done (~75%)
  │
  └── Community mapping (local YAML, instant) → TVDB? → Done (~78% of remainder)

  Not matched → silently skipped (not yet in TVDB)
```

No external API calls during resolution — both mapping files are local.

## Winter overflow

AniList tags WINTER shows by **calendar year**. A show that premieres in
December 2025 is tagged as WINTER 2025 — not WINTER 2026 — even though
it's only a few weeks before the January premieres.

With `winter_overflow: true`, the tool fetches the prior year's WINTER
season and merges any shows with a December start date that aren't already
in the current year's results.

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

- **No MDBList** — v2 replaces the MDBList API with local mapping files.
  No API keys, no rate limits, no external service dependencies.

- **Static files only** — Output is JSON on GitHub Pages. No server needed.
  Sonarr imports directly from the URL.

- **Weekly sync** — AniList seasonal data doesn't change daily. Weekly is
  sufficient and stays well within AniList's rate limits.

- **Paginated fetching** — AniList caps responses at 50 per page. The
  client follows `hasNextPage` to collect up to `max_per_season` results.

- **Auto-downloaded mappings** — Both mapping files download on first run
  and cache locally. No manual setup needed.
