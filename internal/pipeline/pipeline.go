package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/filter"
	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/model"
	"github.com/calmcacil/anilistgen/internal/output"
)

type Deps struct {
	AniClient      *anilist.Client
	Resolver       *mapping.Resolver
	FilterConfig   filter.Config
	WinterOverflow bool
	MaxPerSeason   int
	AheadMonths    int
	Formats        []string
}

type Stats struct {
	Fetched   int
	Filtered  int
	Resolved  int
	Unmatched int
}

type Result struct {
	Key     model.SeasonKey
	All     []output.Show
	NewOnly []output.Show
	Stats   Stats
	Err     error
}

func Run(ctx context.Context, deps Deps, years []int, seasons []string) (map[model.SeasonKey][]output.Show, map[model.SeasonKey][]output.Show, []Stats, []error) {
	allSeries := map[model.SeasonKey][]output.Show{}
	allNew := map[model.SeasonKey][]output.Show{}
	var allStats []Stats
	var errs []error

	for _, year := range years {
		for _, season := range seasons {
			result := Process(ctx, deps, season, year)
			allStats = append(allStats, result.Stats)
			if result.Err != nil {
				errs = append(errs, result.Err)
				continue
			}
			allSeries[result.Key] = result.All
			allNew[result.Key] = result.NewOnly
		}
	}

	if len(years) > 0 && len(seasons) == 4 {
		nextWinter := years[len(years)-1] + 1
		result := Process(ctx, deps, "WINTER", nextWinter)
		allStats = append(allStats, result.Stats)
		if result.Err != nil {
			errs = append(errs, result.Err)
		} else {
			allSeries[result.Key] = result.All
			allNew[result.Key] = result.NewOnly
		}
	}

	return allSeries, allNew, allStats, errs
}

func Process(ctx context.Context, deps Deps, season string, year int) Result {
	key := model.SeasonKey{Season: season, Year: year}
	slog.Info("fetching season", "season", season, "year", year)

	shows, err := deps.AniClient.FetchSeason(ctx, season, year, deps.MaxPerSeason, deps.Formats)
	if err != nil {
		slog.Error("fetch failed", "season", season, "year", year, "error", err)
		return Result{Key: key, Err: err}
	}

	stats := Stats{Fetched: len(shows)}

	if deps.WinterOverflow && season == "WINTER" {
		shows = winterOverflow(ctx, deps.AniClient, year, deps.MaxPerSeason, deps.Formats, shows)
	}

	if season == "WINTER" {
		before := len(shows)
		shows = filterWinterMonth(shows, "winter shows")
		stats.Filtered += before - len(shows)
	}

	slog.Info("fetched shows from AniList",
		"season", season, "year", year, "count", len(shows))

	series, newOnly := splitSeriesNew(shows)

	series = filter.Filter(series, deps.FilterConfig)
	stats.Filtered += stats.Fetched - len(series)
	series = filter.FilterFuture(series, deps.AheadMonths)

	newOnly = filter.Filter(newOnly, deps.FilterConfig)
	newOnly = filter.FilterFuture(newOnly, deps.AheadMonths)

	resolvedAll, allStats := resolveShows(deps.Resolver, series)
	stats.Resolved += allStats.Resolved
	stats.Unmatched += allStats.Unmatched

	resolvedNew, newStats := resolveShows(deps.Resolver, newOnly)
	stats.Resolved += newStats.Resolved
	stats.Unmatched += newStats.Unmatched

	return Result{
		Key:     key,
		All:     resolvedAll,
		NewOnly: resolvedNew,
		Stats:   stats,
		Err:     nil,
	}
}

func resolveShows(resolver *mapping.Resolver, shows []model.Show) ([]output.Show, Stats) {
	resolved := resolver.Project(shows)
	unmatched := len(shows) - len(resolved)
	return resolved, Stats{Resolved: len(resolved), Unmatched: unmatched, Fetched: len(shows)}
}

func winterOverflow(ctx context.Context, client *anilist.Client, year, maxPerSeason int, formats []string, shows []model.Show) []model.Show {
	overflowYear := year - 1
	overflow, err := client.FetchSeason(ctx, "WINTER", overflowYear, maxPerSeason, formats)
	if err != nil {
		slog.Warn("winter overflow fetch failed, continuing without overflow",
			"year", overflowYear, "error", err)
		return shows
	}

	if len(overflow) == 0 {
		return shows
	}

	seen := make(map[int]bool, len(shows))
	for _, sh := range shows {
		seen[sh.ID] = true
	}

	var added int
	for _, sh := range overflow {
		if sh.StartDate.Month != nil && *sh.StartDate.Month == 12 && !seen[sh.ID] {
			shows = append(shows, sh)
			seen[sh.ID] = true
			added++
		}
	}

	if added > 0 {
		slog.Info("winter overflow merged",
			"year", year, "overflow_year", overflowYear,
			"added", added, "total", len(shows))
	}

	return shows
}

func filterWinterMonth(shows []model.Show, label string) []model.Show {
	var filtered []model.Show
	for _, sh := range shows {
		if sh.IsWinterStart() {
			filtered = append(filtered, sh)
		} else {
			slog.Debug("skipped winter show outside season range",
				"title", sh.DisplayTitle(),
				"month", sh.StartDate.Month)
		}
	}
	if len(filtered) != len(shows) {
		slog.Info("filtered "+label+" by start month",
			"total", len(shows),
			"kept", len(filtered),
			"removed", len(shows)-len(filtered))
	}
	return filtered
}

func splitSeriesNew(shows []model.Show) (series, seasonNew []model.Show) {
	series = make([]model.Show, 0)
	seasonNew = make([]model.Show, 0)
	for _, sh := range shows {
		if sh.IsSeries() {
			series = append(series, sh)
			if sh.IsNew() {
				seasonNew = append(seasonNew, sh)
			}
		}
	}
	return
}

func ProcessBatch(resolver *mapping.Resolver, all map[model.SeasonKey][]model.Show, dryRun bool) map[model.SeasonKey][]output.Show {
	out := map[model.SeasonKey][]output.Show{}
	for key, shows := range all {
		resolved := resolver.Project(shows)
		unmatched := len(shows) - len(resolved)

		if dryRun {
			fmt.Printf("\n[%s %d] %d shows (%d resolved, %d unmatched)\n",
				key.Season, key.Year, len(shows), len(resolved), unmatched)
			for _, s := range resolved {
				fmt.Printf("  TVDB %d — %s\n", s.TVDBID, s.Title)
			}
			continue
		}

		out[key] = resolved
	}
	return out
}
