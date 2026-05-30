# anilistgen — Agent Guide

## Overview

A Go CLI tool that fetches seasonal anime from **AniList** (GraphQL API) and
creates/updates **MDBList** lists for use with **Sonarr** (via MDBList
list import, which Sonarr supports natively).

## Architecture

```
cmd/anilistgen/main.go    — entry point, subcommand dispatch
internal/config/             — YAML config loading, validation, init-config gen
internal/anilist/            — AniList GraphQL client (retry/backoff)
internal/mdblist/            — MDBList API client (list CRUD, batch lookup)
internal/logging/            — slog setup
internal/sync/               — orchestration: fetch → filter → lookup → publish
deploy/                      — systemd unit/timer, Dockerfile, docker-compose
```

## Subcommands

| Command | Mode | Description |
|---|---|---|
| `anilistgen` | oneshot | Process all configured seasons, print URLs |
| `anilistgen daemon` | daemon | Loop at config interval, graceful shutdown on SIGINT/SIGTERM |
| `anilistgen init-config` | init | Generate default YAML config |
| `anilistgen validate` | validate | Check config + API connectivity to AniList and MDBList |

## External APIs

### AniList GraphQL

- **Endpoint**: `POST https://graphql.anilist.co` — no auth
- **Query**: Fetches `Media` with filters: `type: ANIME`, `season`, `seasonYear`, `format_in: [TV, ONA]`
- **Pagination**: One page of `max_per_season` results. Logs warning if truncated but does not paginate further.
- **Retry**: 3 attempts with exponential backoff (1s, 2s, 4s) on non-200 responses
- **Fields returned**: `id`, `idMal`, `title.{romaji,english}`, `format`, `episodes`, `duration`, `genres`, `status`, `relations{edges{node{id,idMal,title},relationType}}`
- **Relations used**: When a show's direct MAL ID doesn't resolve in MDBList, prequel relation chain is used as fallback to find an earlier season that MDBList has indexed.

### MDBList API

- **Endpoint**: `https://api.mdblist.com` — requires `apikey` query param
- **Endpoints used**:
  - `GET /user` — ping/connectivity check
  - `GET /lists/user` — list user's lists (for dedup by title)
  - `POST /lists/user/add` — create list with name, description, public
  - `DELETE /lists/{id}` — delete list (used for replace)
  - `POST /lists/{id}/items/add` — batch-add items by provider ID
  - `POST /{provider}/{type}` — batch lookup media by external IDs (e.g. `POST /mal/show` with `{"ids":["16498"]}`)
- **Rate limiting**: 1.1s throttle between calls; exponential backoff on 429 (2s, 4s, 8s)
- **Plan limits**: Free tier = 4 static lists. Pro/supporter plans have higher limits.

### Item Resolution

Items are added to MDBList lists using **IMDB IDs** (or TMDB/TVDB as fallbacks)
obtained via batch lookup by MAL ID:

1. AniList returns `idMal` per show (MyAnimeList ID)
2. MDBList batch lookup resolves MAL IDs → `{imdb, tmdb, tvdb, mal, ...}`
3. Shows are added to lists using the resolved IMDB ID
4. **Relation fallback**: If a show's direct MAL ID isn't in MDBList, the tool
   follows AniList `relations` (PREQUEL chains) to find a parent series that
   MDBList does have, and adds the show under that parent entry

## Sync Pipeline

For each season/year:

1. **Fetch** — Query AniList for TV/ONA anime
2. **Winter overflow** — For WINTER season, also fetches prior year's WINTER
   and merges only shows with `startDate.month == 12` (December premieres
   that AniList tagged under the prior calendar year but belong in the
   current winter viewing list). Filtered by `StartedInDecember()`.
3. **Filter** — Remove shows with duration ≤10 min, blacklisted shows, and
   tag-excluded shows
4. **Lookup** — Batch-resolve MAL IDs (+ relation MAL IDs) against MDBList
5. **Match** — Try direct MAL ID first, then fallback to relation chain
6. **Publish** — Find existing list by title → diff-update or delete+recreate
   (or create new if doesn't exist)

## Key Design Decisions

- **Delete-and-recreate** rather than diff-based updates. MDBList doesn't
  support replacing all items in a single call, and seasonal shows change
  significantly between runs. Simpler and more reliable.
- **Fallback matching via AniList relations** rather than Jikan API or other
  external services. Keeps dependencies minimal and latency low (one extra
  GraphQL field vs. N extra HTTP calls).
- **Intermediary MAL → IMDB resolution**. MDBList's list items endpoint accepts
  `imdb`, `trakt`, `tmdb`, `tvdb` provider keys but NOT `mal`. So MAL IDs from
  AniList are resolved to IMDB IDs via MDBList's batch lookup first.
- **Batch lookups are single-call per season** — all MAL IDs (direct + relation)
  are collected and looked up in one request to minimize API calls.

## Config File

Location (searched in order):
1. `-config` CLI flag path
2. `./anilistgen.yaml`
3. `$XDG_CONFIG_HOME/anilistgen/anilistgen.yaml` (defaults to `~/.config/...`)

Unknown top-level keys produce a warning on stderr but don't prevent startup.

## Testing

```bash
go build ./cmd/anilistgen
go vet ./...
./anilistgen -dry-run        # fetches AniList, no MDBList writes
./anilistgen -output /tmp/x  # writes JSON files instead of MDBList calls
./anilistgen validate        # checks config + API connectivity
```

## Security Notes

- **API keys in config** — The config file (`anilistgen.yaml`) is in
  `.gitignore` to prevent accidental commits. Use `anilistgen.yaml.example`
  as a template. The env var `ALG_MDBLIST_API_KEY` can also be used.
- **No auth on AniList reads** — AniList GraphQL is public, no credentials needed.
- **MDBList key transmitted as query param** — sent over HTTPS. Rotate if
  accidentally exposed.
