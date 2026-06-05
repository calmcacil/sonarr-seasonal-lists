# DEPRECATED — Seasonal Anime Lists for Sonarr (UNOFFICIAL AniList)

> **This project is superseded by [sonarr-anime-bridge](https://github.com/calmcacil/sonarr-anime-bridge),
> which provides a Docker container with the same functionality plus anime movie
> list support. Users are recommended to migrate. This repository remains
> available for reference but is no longer actively maintained.**

Sonarr-compatible seasonal anime lists from AniList.

Run `anilistgen -output ./out` to generate JSON files in `./out/YYYY/`.
The resulting JSON can be served via any static file server or imported
directly from a local path with Sonarr's Custom List import.

| File | What it includes |
|---|---|
| `YYYY/winter-series.json` | TV shows airing that season |
| `YYYY/series.json` | All TV shows for that year |
| `YYYY/series-new.json` | New TV shows only (no sequels) |

## Licenses

| Document | Contents |
|---|---|
| [LICENSE](./LICENSE) | MIT License for this project |
| [NOTICE](./NOTICE) | Third-party attribution notices |

## Further reading

| Document | Contents |
|---|---|
| [docs/CONFIGURATION.md](./docs/CONFIGURATION.md) | Config reference and environment variables |
| [docs/SELFHOSTING.md](./docs/SELFHOSTING.md) | Run locally |
| [docs/EXPLANATION.md](./docs/EXPLANATION.md) | Architecture, pipeline, and data sources |

Licenses and docs below.
