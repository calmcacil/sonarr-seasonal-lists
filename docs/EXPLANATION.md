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
    │         Anibridge mapping (MAL/AniList → TVDB)
    │
    ▼
  Output — write compact JSON:
              • Per season: winter-2026.json
              • Yearly aggregate: 2026.json
```

## Data source

**[anibridge/anibridge-mappings](https://github.com/anibridge/anibridge-mappings)**
(`mappings.json.zst`, ~1.6 MB compressed).

A cross-provider anime ID dataset with episode-level granularity covering
8 providers (AniDB, AniList, MAL, TMDB, TVDB, IMDB). Downloaded on first
run and cached locally. From the frozen id-graph anilistgen extracts:

- **~8,900** MAL → TVDB season-1 mappings
- **~9,100** AniList → TVDB season-1 mappings

Combined, these cover **~98%** of seasonal anime (up from ~78% with the
previous source).

### Resolution

```
Show from AniList → MAL ID → Anibridge → TVDB? → Done
                      └→ AniList ID → (if no MAL or MAL not in mapping)
```

No external API calls during resolution — the mapping file is local.

### Known gap

The anibridge dataset is built daily from multiple upstream sources but
may lag behind very recent MAL entries for upcoming-season shows. During
the migration from the previous source (shinkro/community-mapping),
7 shows (all upcoming 2026 summer) were identified as having MAL→TVDB
links in the old source but not yet in anibridge:

| MAL | TVDB | Title |
|---|---|---|
| 61483 | 462561 | Tenmaku no Jaadugar |
| 62430 | 467841 | BanG Dream! Yume∞Mita |
| 62535 | 468399 | Hanaori-san Still Wants to Fight in the Next Life |
| 62876 | 470406 | Rich Girl Caretaker |
| 63537 | 474490 | "Kimi wo Aisuru Ki wa nai" |
| 63752 | 475631 | The Forsaken Saintess |
| 63802 | 475980 | Mobius Dust |

These should be verified before their season starts (summer 2026 airing)
by running `go test ./...` and checking that `unmatched` count for
summer 2026 is no longer elevated. If the anibridge dataset still lacks
these by airing time, consider re-adding the previous shinkro source as a
fallback layer beneath anibridge (the old `community_mapping_path` config
key and loader are preserved in git history).

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

- **No MDBList** — anilistgen uses a local cross-provider mapping file.
  No API keys, no rate limits, no external service dependencies.

- **Static files only** — Output is JSON. No server needed.
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

- **Auto-downloaded mapping** — The anibridge mapping file downloads on
  first run and caches locally. No manual setup needed.
