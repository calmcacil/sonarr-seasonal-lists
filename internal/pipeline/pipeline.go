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
	MaxPerYear     int
	AheadMonths    int
	Formats        []string
}

type Stats struct {
	Season    string
	Year      int
	Fetched   int
	Filtered  int
	Resolved  int
	Unmatched int
}

func Run(ctx context.Context, deps Deps, years []int, seasons []string) (map[model.SeasonKey][]output.Show, map[model.SeasonKey][]output.Show, []Stats, []error) {
	allSeries := map[model.SeasonKey][]output.Show{}
	allNew := map[model.SeasonKey][]output.Show{}
	var allStats []Stats
	var errs []error

	for _, year := range years {
		slog.Info("fetching year", "year", year)
		shows, err := deps.AniClient.FetchYear(ctx, year, deps.MaxPerYear, deps.Formats)
		if err != nil {
			slog.Error("fetch year failed", "year", year, "error", err)
			for _, season := range seasons {
				allStats = append(allStats, Stats{Season: season, Year: year})
				errs = append(errs, fmt.Errorf("year %d: %w", year, err))
			}
			continue
		}

		slog.Info("fetched shows from AniList", "year", year, "count", len(shows))

		if deps.WinterOverflow {
			shows = winterOverflow(ctx, deps.AniClient, year, deps.MaxPerYear, deps.Formats, shows)
		}

		bySeason := groupBySeason(shows)

		for _, season := range seasons {
			seasonShows := bySeason[season]

			stats := Stats{Season: season, Year: year, Fetched: len(seasonShows)}
			filteredCount := 0

			if season == "WINTER" {
				before := len(seasonShows)
				seasonShows = filterWinterMonth(seasonShows, "winter shows")
				filteredCount += before - len(seasonShows)
			}

			series, newOnly := splitSeriesNew(seasonShows)

			before := len(series) + len(newOnly)
			series = filter.Filter(series, deps.FilterConfig)
			newOnly = filter.Filter(newOnly, deps.FilterConfig)
			filteredCount += before - len(series) - len(newOnly)

			series = filter.FilterFuture(series, deps.AheadMonths)
			newOnly = filter.FilterFuture(newOnly, deps.AheadMonths)

			stats.Filtered = filteredCount

			resolvedAll := deps.Resolver.Project(series)
			resolvedNew := deps.Resolver.Project(newOnly)

			key := model.SeasonKey{Season: season, Year: year}
			allSeries[key] = resolvedAll
			allNew[key] = resolvedNew

			stats.Resolved = len(resolvedAll) + len(resolvedNew)
			stats.Unmatched = len(series) + len(newOnly) - stats.Resolved

			allStats = append(allStats, stats)
		}
	}

	return allSeries, allNew, allStats, errs
}

func winterOverflow(ctx context.Context, client *anilist.Client, year, maxPerYear int, formats []string, shows []model.Show) []model.Show {
	overflowYear := year - 1
	overflow, err := client.FetchSeason(ctx, "WINTER", overflowYear, maxPerYear, formats)
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

func groupBySeason(shows []model.Show) map[string][]model.Show {
	m := map[string][]model.Show{
		"WINTER":  {},
		"SPRING":  {},
		"SUMMER":  {},
		"FALL":    {},
		"UNKNOWN": {},
	}
	for _, sh := range shows {
		code := sh.SeasonCode()
		m[code] = append(m[code], sh)
	}
	return m
}


