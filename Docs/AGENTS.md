# anilistgen — Agent Guide

## Overview

Go CLI tool that fetches seasonal anime from AniList, resolves TVDB IDs
from local mapping files, and outputs Sonarr-compatible JSON for GitHub Pages.

## Commands

```bash
go build ./cmd/anilistgen
go vet ./...
go test ./...
./anilistgen -dry-run
./anilistgen -output /tmp/x
./anilistgen validate
```

## Project structure

```
cmd/anilistgen/main.go    — fetch → filter → resolve → output
internal/anilist/            — AniList GraphQL client (paginated, retry)
internal/config/             — YAML config, env var overrides
internal/mapping/            — TVDB resolution (anime-lists then community)
internal/filter/             — duration, blacklist, tags, future-date filter
internal/output/             — compact JSON (per-season + yearly)
internal/logging/            — slog setup
```

## Key facts

- AniList API caps `perPage` at 50. The client paginates via `hasNextPage`.
- Mapping is local-only: Anime-Lists XML (AniList ID → TVDB) then community
  YAML (MAL ID → TVDB). No external API calls during resolution.
- Auto-downloads mapping files on first run to configurable paths.
- JSON output is minified. Sonarr reads `tvdbId`; `title` is cosmetic.
- Winter overflow fetches prior year's WINTER and merges December premieres.
- Config via YAML file or `ALG_*` env vars.

## Config precedence

1. `-config` flag path
2. `./anilistgen.yaml`
3. `~/.config/anilistgen/anilistgen.yaml`
4. Environment variables (`ALG_*`)

Unknown top-level config keys produce a stderr warning but don't prevent
startup.

## Stats

- Sync time: ~10–15s (4 AniList API calls for all seasons)
- Community mapping: ~5,241 entries (78% coverage)
- Anime-lists mapping: ~10,688 entries (~10,688 coverage)
