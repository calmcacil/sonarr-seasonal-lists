# anilistgen

Sonarr-compatible seasonal anime lists from AniList. Zero infrastructure —
just static JSON files hosted on GitHub Pages.

Sonarr has no built-in "all shows airing this season" source. This tool
fetches seasonal anime from AniList, resolves TVDB IDs, and outputs compact
JSON files you can import directly into Sonarr as a Custom List.

---

## Quick start

### Use the hosted lists (easiest)

The [calmcacil/anilistgen](https://github.com/calmcacil/anilistgen) repo
publishes lists every Sunday to GitHub Pages. Add any of these URLs to
Sonarr:

```
https://calmcacil.github.io/anilistgen/winter-2026.json
https://calmcacil.github.io/anilistgen/spring-2026.json
https://calmcacil.github.io/anilistgen/summer-2026.json
https://calmcacil.github.io/anilistgen/fall-2026.json
https://calmcacil.github.io/anilistgen/2026.json
```

Sonarr → Settings → Import Lists → Add → Custom List → paste URL.

### Run locally

```bash
# Build
go build ./cmd/anilistgen

# Generate default config
./anilistgen init-config

# Preview
./anilistgen -dry-run

# Generate JSON files
./anilistgen -output ./sonarr-lists
```

See [SELFHOSTING.md](./SELFHOSTING.md) for running your own setup.

---

## Commands

| Command | Description |
|---|---|
| `anilistgen` | Generate JSON files (default) |
| `anilistgen init-config` | Write a default config file |
| `anilistgen validate` | Validate config + test AniList connectivity |

## Flags

| Flag | Description |
|---|---|
| `-config PATH`, `-c PATH` | Config file path (overrides default search) |
| `-dry-run` | Print results without writing files |
| `-output DIR`, `-o DIR` | Output directory (overrides config) |
| `-v`, `-verbose` | Debug logging |
| `-h`, `-help` | Print help |
| `-version`, `-V` | Print version |

---

## Configuration

Config loaded from (first found wins):
1. `-config` flag path
2. `./anilistgen.yaml`
3. `~/.config/anilistgen/anilistgen.yaml`

All settings can also be set via `ALG_`-prefixed environment variables.

### Config reference

```yaml
anilist:
  years: [2026]
  seasons: [all]            # or: winter, spring, summer, fall
  max_per_season: 100
  include_ona: false
  winter_overflow: true
  ahead_months: 3
  exclude_tags: []          # e.g. ["Hentai"]

blacklist: []               # MAL ID or title substring

output_dir: ./sonarr-lists

community_mapping_path: /tmp/anilistgen_tvdb.yaml   # auto-downloaded
anime_lists_path: /tmp/anime-list-full.xml           # auto-downloaded

logging:
  level: info
  file: ""
```

### Environment variables

| Env var | Config field | Default |
|---|---|---|
| `ALG_ANILIST_YEARS` | `anilist.years` | current year |
| `ALG_ANILIST_SEASONS` | `anilist.seasons` | `all` |
| `ALG_ANILIST_MAX_PER_SEASON` | `anilist.max_per_season` | `100` |
| `ALG_ANILIST_INCLUDE_ONA` | `anilist.include_ona` | `false` |
| `ALG_ANILIST_WINTER_OVERFLOW` | `anilist.winter_overflow` | `true` |
| `ALG_ANILIST_EXCLUDE_TAGS` | `anilist.exclude_tags` | `""` |
| `ALG_ANILIST_AHEAD_MONTHS` | `anilist.ahead_months` | `3` |
| `ALG_BLACKLIST` | `blacklist` | `""` |
| `ALG_OUTPUT_DIR` | `output_dir` | `./sonarr-lists` |
| `ALG_LOG_LEVEL` | `logging.level` | `info` |
| `ALG_LOG_FILE` | `logging.file` | `""` (stderr) |

Lists are comma-separated (`2026,2027`). Booleans accept `true`/`1` or `false`/`0`.

### Filters

- **Duration** — skips shows ≤ 10 min per episode
- **Format** — TV only by default; ONA via `include_ona: true`
- **Blacklist** — exclude by MAL ID or title substring (case-insensitive)
- **Tags** — exclude by AniList content tag (e.g. `"Hentai"`)
- **Ahead** — skip shows starting more than N months in the future

---

## Output format

Both per-season and yearly files are bare JSON arrays — Sonarr's Custom
List import expects this exactly.

**`winter-2026.json`** (and `2026.json`):
```json
[{"tvdbId":377543,"title":"..."},{"tvdbId":424536,"title":"..."}]
```

JSON is minified. Sonarr reads `tvdbId`; `title` is cosmetic.

---

## Season timing

| Season | Months | Typical premiere window |
|---|---|---|
| **WINTER** | Dec–Feb | Early January |
| **SPRING** | Mar–May | Early April |
| **SUMMER** | Jun–Aug | Early July |
| **FALL** | Sep–Nov | Early October |

---

## Contributing

1. Read [`EXPLANATION.md`](./EXPLANATION.md) for architecture and design.
2. Run `go vet ./...` and `go test ./...` before submitting.
3. Config files are in `.gitignore` — never commit API keys.
4. Build with `go build ./cmd/anilistgen`.
