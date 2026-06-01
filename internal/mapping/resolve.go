package mapping

import (
	"log/slog"

	"github.com/calmcacil/anilistgen/internal/model"
	"github.com/calmcacil/anilistgen/internal/output"
)

type Resolver struct {
	community *CommunityMapping
}

func NewResolver(cm *CommunityMapping) *Resolver {
	return &Resolver{community: cm}
}

func (r *Resolver) Project(shows []model.Show) []output.Show {
	var result []output.Show
	for _, show := range shows {
		malID := 0
		if show.IDMal != nil {
			malID = *show.IDMal
		}
		if malID <= 0 {
			continue
		}
		if tvdbID, ok := r.community.Lookup(malID); ok {
			slog.Debug("resolved via community mapping",
				"title", show.DisplayTitle(), "mal", malID, "tvdb", tvdbID)
			result = append(result, output.Show{
				TVDBID: tvdbID,
				Title:  show.DisplayTitle(),
			})
		}
	}
	return result
}
