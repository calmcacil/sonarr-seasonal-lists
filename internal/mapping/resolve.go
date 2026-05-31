package mapping

import (
	"context"
	"log/slog"

	"github.com/calmcacil/anilistgen/internal/anilist"
)

type Resolver struct {
	community  *CommunityMapping
	animelists *AnimeListsMapping
	jikan      *JikanClient
	tmdb       *TMDBClient
}

type ResolvedShow struct {
	MALID    int
	TVDBID   int
	TMDBID   int
	Title    string
	Resolved bool
}

func NewResolver(cm *CommunityMapping, alm *AnimeListsMapping, jc *JikanClient, tc *TMDBClient) *Resolver {
	return &Resolver{
		community:  cm,
		animelists: alm,
		jikan:      jc,
		tmdb:       tc,
	}
}

// Resolve tries each mapping source in order:
// 1. Community mapping (MAL → TVDB, instant)
// 2. Jikan + anime-lists (MAL → AniDB → TVDB/TMDB, requires API call)
// 3. TMDB search (by title, for movies only, requires API key)
// Returns tvdbID, tmdbID, resolved.
func (r *Resolver) Resolve(ctx context.Context, malID int, anilistID int, title string, isMovie bool) (tvdbID int, tmdbID int, resolved bool) {
	if malID <= 0 && !isMovie {
		return 0, 0, false
	}

	// Step 1: Community mapping (instant, highest coverage)
	if malID > 0 {
		if t, ok := r.community.Lookup(malID); ok {
			slog.Debug("resolved via community mapping",
				"title", title, "mal", malID, "tvdb", t)
			return t, 0, true
		}
	}

	// Step 2: Jikan → AniDB → anime-lists lookup
	if malID > 0 && r.jikan != nil && r.animelists != nil {
		anidbID, err := r.jikan.MALToAniDB(ctx, malID)
		if err == nil {
			tvdbID, hasTVDB := r.animelists.Lookup(anidbID)
			tmdbID, hasTMDB := r.animelists.LookupTMDB(anidbID)
			if hasTVDB || hasTMDB {
				slog.Debug("resolved via anime-lists",
					"title", title, "mal", malID, "anidb", anidbID,
					"tvdb", tvdbID, "tmdb", tmdbID)
				return tvdbID, tmdbID, true
			}
		} else {
			slog.Debug("jikan lookup failed", "title", title, "mal", malID, "error", err)
		}
	}

	// Step 3: TMDB search (movies only, final fallback)
	if isMovie && r.tmdb != nil && title != "" {
		tmdbID, err := r.tmdb.SearchMovie(ctx, title, 0)
		if err != nil {
			slog.Debug("tmdb search failed", "title", title, "error", err)
		} else if tmdbID > 0 {
			slog.Debug("resolved via tmdb search",
				"title", title, "tmdb", tmdbID)
			return 0, tmdbID, true
		}
	}

	return 0, 0, false
}

func (r *Resolver) ResolveBatch(ctx context.Context, shows []anilist.Show, isMovies bool) map[int]ResolvedShow {
	result := make(map[int]ResolvedShow, len(shows))

	for _, show := range shows {
		malID := 0
		if show.IDMal != nil {
			malID = *show.IDMal
		}

		rs := ResolvedShow{
			MALID: malID,
			Title: show.DisplayTitle(),
		}

		tvdbID, tmdbID, ok := r.Resolve(ctx, malID, show.ID, rs.Title, isMovies)
		if ok {
			rs.TVDBID = tvdbID
			rs.TMDBID = tmdbID
			rs.Resolved = true
		}

		result[show.ID] = rs
	}

	return result
}
