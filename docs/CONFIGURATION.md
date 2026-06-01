# Configuration

anilistgen reads settings from three sources (later overrides earlier):

1. **Config file**: `./anilistgen.yaml` → `~/.config/anilistgen/anilistgen.yaml`
2. **CLI flag**: `-config /path/to/config.yaml`
3. **Environment variables**: Every setting has an `ALG_`-prefixed env var

Run `anilistgen init-config` to generate a documented template.

---

## Config reference

```yaml
anilist:
  years: [2026]
  seasons: [all]            # winter, spring, summer, fall, or all
  max_per_season: 100
  include_ona: false
  winter_overflow: true
  ahead_months: 3
  exclude_tags: []          # e.g. ["Hentai"]

blacklist: []               # MAL ID or title substring

output_dir: ./sonarr-lists

base_url: https://lists.calmcacil.dev

community_mapping_path: /tmp/anilistgen_tvdb.yaml   # auto-downloaded

logging:
  level: info
  file: ""
```

---

## Fields

### `anilist`

| Field | Type | Default | Description |
|---|---|---|---|
| `years` | `[int]` | current year | Years to process |
| `seasons` | `[string]` | `["all"]` | `winter`, `spring`, `summer`, `fall`, or `all` |
| `max_per_year` | `int` | `400` | Max shows per year (fetched in one query, then split by season internally) |
| `include_ona` | `bool` | `false` | Include ONA format alongside TV in series output |
| `winter_overflow` | `bool` | `true` | Merge December premieres from prior year's WINTER |
| `ahead_months` | `int` | `3` | Skip shows starting more than N months ahead. `0` disables. |
| `exclude_tags` | `[string]` | `[]` | AniList content tags to exclude (case-insensitive) |

**Note on formats**: TV, MOVIE, OVA, and SPECIAL are always fetched.
`include_ona` adds ONA to the series category. Output is split:
- `series-*` files → TV + ONA (if enabled)
- `movies-*` files → MOVIE, OVA, SPECIAL

**Note on fetching**: Instead of fetching each season separately, the tool
now fetches the full year from AniList in one query (up to `max_per_year`
results, paginated 50 per page) and splits the results by season internally.
This reduces API round-trips by ~50% and speeds up backfills considerably.

### `blacklist`

| Type | Default | Description |
|---|---|---|
| `[string]` | `[]` | MAL IDs or title substrings to exclude. Case-insensitive. |

Each entry is either a numeric MAL ID (`16498`) or a title substring
(`"One Piece"`). Substring matches any show whose title contains the
text.

### `output_dir`

| Type | Default |
|---|---|
| `string` | `./sonarr-lists` |

Directory where JSON files are written.

### `community_mapping_path`

| Type | Default |
|---|---|
| `string` | `/tmp/anilistgen_tvdb.yaml` |

Path to the shinkro/community-mapping YAML file (MAL ID → TVDB ID).
Auto-downloaded if the file doesn't exist.

### `base_url`

| Type | Default |
|---|---|
| `string` | `https://lists.calmcacil.dev` |

Base URL used in the generated index page for copy-to-clipboard URLs.
Override when self-hosting on a custom domain.

### `logging`

| Field | Type | Default | Description |
|---|---|---|---|
| `level` | `string` | `"info"` | `debug`, `info`, `warn`, `error` |
| `file` | `string` | `""` | Log file path. Empty = stderr. |

---

## Environment variables

Every config field can be set via environment variables with the `ALG_`
prefix — useful for Docker, CI/CD, or when no config file is present.

| Env var | Config field | Default |
|---|---|---|
| `ALG_ANILIST_YEARS` | `anilist.years` | current year |
| `ALG_ANILIST_SEASONS` | `anilist.seasons` | `all` |
| `ALG_ANILIST_MAX_PER_YEAR` | `anilist.max_per_year` | `400` |
| `ALG_ANILIST_INCLUDE_ONA` | `anilist.include_ona` | `false` |
| `ALG_ANILIST_WINTER_OVERFLOW` | `anilist.winter_overflow` | `true` |
| `ALG_ANILIST_EXCLUDE_TAGS` | `anilist.exclude_tags` | `""` |
| `ALG_ANILIST_AHEAD_MONTHS` | `anilist.ahead_months` | `3` |
| `ALG_BLACKLIST` | `blacklist` | `""` |
| `ALG_OUTPUT_DIR` | `output_dir` | `./sonarr-lists` |
| `ALG_BASE_URL` | `base_url` | `https://lists.calmcacil.dev` |
| `ALG_COMMUNITY_MAPPING_PATH` | `community_mapping_path` | `/tmp/anilistgen_tvdb.yaml` |
| `ALG_LOG_LEVEL` | `logging.level` | `info` |
| `ALG_LOG_FILE` | `logging.file` | `""` |

**Format notes:**
- Lists (YEARS, SEASONS, BLACKLIST, EXCLUDE_TAGS) use comma separation:
  `2026,2027`
- Booleans accept `true`/`1` or `false`/`0`
