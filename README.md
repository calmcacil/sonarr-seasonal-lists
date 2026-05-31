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
https://calmcacil.github.io/anilistgen/series-winter-2026.json
https://calmcacil.github.io/anilistgen/series-spring-2026.json
https://calmcacil.github.io/anilistgen/series-summer-2026.json
https://calmcacil.github.io/anilistgen/series-fall-2026.json
https://calmcacil.github.io/anilistgen/series-2026.json
https://calmcacil.github.io/anilistgen/movies-winter-2026.json
https://calmcacil.github.io/anilistgen/movies-2026.json
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

See [Docs/SELFHOSTING.md](./Docs/SELFHOSTING.md) for running your own
setup (local cron, fork on GitHub, other hosting).

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

See [Docs/CONFIGURATION.md](./Docs/CONFIGURATION.md) for the full config
reference, all fields, and environment variables.

Quick summary of the config file (`anilistgen.yaml`):

```yaml
anilist:
  years: [2026]
  seasons: [all]
  max_per_season: 100
  include_ona: false
  winter_overflow: true
  ahead_months: 3
  exclude_tags: []

blacklist: []

output_dir: ./sonarr-lists

community_mapping_path: /tmp/anilistgen_tvdb.yaml
anime_lists_path: /tmp/anime-list-full.xml

logging:
  level: info
  file: ""
```

---

## Output format

Files are bare JSON arrays — Sonarr's Custom List import requires this.
Output is split by category:

| Prefix | Contents |
|---|---|
| `series-` | TV and ONA shows |
| `series-new-` | New IPs only (no sequels or spin-offs) |
| `movies-` | Movies, OVAs, and specials |
| `movies-new-` | New movies/OVAs/specials only |

**`series-winter-2026.json`** (and `series-new-winter-2026.json`):

```json
[{"tvdbId":377543,"title":"..."},{"tvdbId":424536,"title":"..."}]
```

JSON is minified. Sonarr reads `tvdbId`; `title` is cosmetic.
Empty categories produce no file.

---

## Season timing

| Season | Months | Typical premiere window |
|---|---|---|
| **WINTER** | Dec–Feb | Early January |
| **SPRING** | Mar–May | Early April |
| **SUMMER** | Jun–Aug | Early July |
| **FALL** | Sep–Nov | Early October |

---

## Further reading

| Document | Contents |
|---|---|
| [Docs/CONFIGURATION.md](./Docs/CONFIGURATION.md) | Full config reference, env vars |
| [Docs/EXPLANATION.md](./Docs/EXPLANATION.md) | Architecture, pipeline, data sources |
| [Docs/SELFHOSTING.md](./Docs/SELFHOSTING.md) | Local, fork, and custom hosting setups |
| [Docs/AGENTS.md](./Docs/AGENTS.md) | LLM agent guide for the codebase |

## Contributing

1. Read [`Docs/AGENTS.md`](./Docs/AGENTS.md) for architecture.
2. Run `go vet ./...` and `go test ./...` before submitting.
3. Config files are in `.gitignore` — never commit API keys.
4. Build with `go build ./cmd/anilistgen`.
