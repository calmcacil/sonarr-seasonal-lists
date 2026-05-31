package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/config"
	"github.com/calmcacil/anilistgen/internal/filter"
	"github.com/calmcacil/anilistgen/internal/logging"
	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/output"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath string
		dryRun     bool
		outputDir  string
		verbose    bool
		help       bool
		showVer    bool
	)

	flags := flag.NewFlagSet("anilistgen", flag.ContinueOnError)
	flags.StringVar(&configPath, "config", "", "path to config file")
	flags.StringVar(&configPath, "c", "", "path to config file (shorthand)")
	flags.BoolVar(&dryRun, "dry-run", false, "print results without writing files")
	flags.StringVar(&outputDir, "output", "", "output directory (overrides config)")
	flags.StringVar(&outputDir, "o", "", "output directory (shorthand)")
	flags.BoolVar(&verbose, "v", false, "verbose logging")
	flags.BoolVar(&verbose, "verbose", false, "verbose logging")
	flags.BoolVar(&help, "h", false, "print help")
	flags.BoolVar(&help, "help", false, "print help")
	flags.BoolVar(&showVer, "version", false, "print version and exit")
	flags.BoolVar(&showVer, "V", false, "print version and exit (shorthand)")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
			return nil
		}
		return fmt.Errorf("%w", err)
	}

	if help {
		printUsage()
		return nil
	}

	if showVer {
		fmt.Println(version)
		return nil
	}

	args := flags.Args()
	subcommand := ""
	if len(args) > 0 && args[0] != "help" && args[0] != "h" {
		subcommand = args[0]
	}

	switch subcommand {
	case "init-config":
		return runInitConfig(configPath)
	case "validate":
		return runValidate(configPath, verbose)
	default:
		return runGenerate(configPath, dryRun, outputDir, verbose)
	}
}

func printUsage() {
	fmt.Println(`anilistgen — generate Sonarr-compatible seasonal anime lists from AniList

Usage:
  anilistgen [flags]                    Generate JSON files (default)
  anilistgen init-config [flags]        Generate default config file
  anilistgen validate [flags]           Validate config

Flags:
  -config, -c PATH      Path to config file
  -dry-run              Print results without writing files
  -output, -o DIR       Output directory (overrides config)
  -v, -verbose          Verbose logging
  -h, -help             Print this help
  -version, -V          Print version`)
}

func runInitConfig(cliPath string) error {
	path := config.ResolveConfigPath(cliPath)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists at %s", path)
	}
	if err := config.WriteDefaultConfig(path); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	fmt.Printf("Default config written to %s\n", path)
	return nil
}

func runValidate(configPath string, verbose bool) error {
	cfg, cfgPath, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	closeLog, err := setupLogging(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeLog()

	fmt.Printf("Config: %s\n", cfgPath)
	fmt.Printf("  Years: %v\n", cfg.AniList.YearsOrDefault())
	fmt.Printf("  Seasons: %s\n", cfg.AniList.Season())
	fmt.Printf("  Max per season: %d\n", cfg.AniList.MaxPerSeason)
	fmt.Printf("  Include ONA: %t\n", cfg.AniList.IncludeONA)
	fmt.Printf("  Output dir: %s\n", cfg.OutputDir)
	fmt.Printf("  Log level: %s\n", cfg.Logging.Level)

	if err := cfg.Validate(); err != nil {
		fmt.Printf("\nConfig validation failed:\n  %s\n", err)
		os.Exit(1)
	}
	fmt.Println("\nConfig is valid")

	fmt.Print("Testing AniList API... ")
	aniClient := anilist.New()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = aniClient.Ping(ctx)
	cancel()
	if err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Println("reachable")
	}

	fmt.Println("\nValidation complete")
	return nil
}

func runGenerate(configPath string, dryRun bool, outputDir string, verbose bool) error {
	cfg, cfgPath, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	closeLog, err := setupLogging(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeLog()

	if cfgPath != "" {
		slog.Info("loaded config", "path", cfgPath)
	}

	if outputDir == "" {
		outputDir = cfg.OutputDir
	}

	cm, err := mapping.LoadCommunityMapping(cfg.CommunityMappingPath)
	if err != nil {
		return fmt.Errorf("load community mapping: %w", err)
	}

	alm, err := mapping.LoadAnimeListsMapping(cfg.AnimeListsPath)
	if err != nil {
		return fmt.Errorf("load anime-lists mapping: %w", err)
	}

	aniClient := anilist.New()
	jikan := mapping.NewJikanClient(cfg.CommunityMappingPath + ".jikan_cache.json")
	var tmdb *mapping.TMDBClient
	if cfg.TMDBAPIKey != "" {
		tmdb = mapping.NewTMDBClient(cfg.TMDBAPIKey)
		slog.Info("TMDB search enabled for movie fallback")
	}
	resolver := mapping.NewResolver(cm, alm, jikan, tmdb)

	formats := []string{"TV", "MOVIE", "OVA", "SPECIAL"}
	if cfg.AniList.IncludeONA {
		formats = append(formats, "ONA")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	years := cfg.AniList.YearsOrDefault()
	seasons := cfg.AniList.Season()

	type category struct {
		label string
		shows map[string][]anilist.Show
		new   map[string][]anilist.Show
	}
	series := category{label: "series", shows: map[string][]anilist.Show{}, new: map[string][]anilist.Show{}}
	movies := category{label: "movies", shows: map[string][]anilist.Show{}}

	for _, year := range years {
		for _, season := range seasons {
			slog.Info("fetching season", "season", season, "year", year)

			shows, err := aniClient.FetchSeason(ctx, season, year, cfg.AniList.MaxPerSeason, formats)
			if err != nil {
				slog.Error("fetch failed", "season", season, "year", year, "error", err)
				continue
			}

			if cfg.AniList.WinterOverflow && season == "WINTER" {
				overflowYear := year - 1
				overflow, err := aniClient.FetchSeason(ctx, season, overflowYear,
					cfg.AniList.MaxPerSeason, formats)
				if err != nil {
					slog.Warn("winter overflow fetch failed, continuing without overflow",
						"year", overflowYear, "error", err)
				} else if len(overflow) > 0 {
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
				}
			}

			slog.Info("fetched shows from AniList",
				"season", season, "year", year, "count", len(shows))

			var seriesShows, movieShows []anilist.Show
			var seriesNew []anilist.Show
			for _, sh := range shows {
				if sh.IsSeries() {
					seriesShows = append(seriesShows, sh)
					if sh.IsNew() {
						seriesNew = append(seriesNew, sh)
					}
				} else {
					movieShows = append(movieShows, sh)
				}
			}

			seriesShows = filter.Filter(seriesShows, filter.Config{
				Blacklist:   cfg.Blacklist,
				ExcludeTags: cfg.AniList.ExcludeTags,
				AheadMonths: cfg.AniList.AheadMonths,
			})
			seriesShows = filter.FilterFuture(seriesShows, cfg.AniList.AheadMonths)

			seriesNew = filter.Filter(seriesNew, filter.Config{
				Blacklist:   cfg.Blacklist,
				ExcludeTags: cfg.AniList.ExcludeTags,
				AheadMonths: cfg.AniList.AheadMonths,
			})
			seriesNew = filter.FilterFuture(seriesNew, cfg.AniList.AheadMonths)

			movieShows = filter.Filter(movieShows, filter.Config{
				Blacklist:   cfg.Blacklist,
				ExcludeTags: cfg.AniList.ExcludeTags,
				AheadMonths: cfg.AniList.AheadMonths,
			})
			movieShows = filter.FilterFuture(movieShows, cfg.AniList.AheadMonths)

			key := fmt.Sprintf("%s-%d", season, year)
			series.shows[key] = seriesShows
			series.new[key] = seriesNew
			movies.shows[key] = movieShows
		}
	}

	if len(years) > 0 && len(seasons) == 4 {
		lastYear := years[len(years)-1]
		nextWinter := lastYear + 1
		slog.Info("all seasons enabled, also fetching next winter",
			"season", "WINTER", "year", nextWinter)

		shows, err := aniClient.FetchSeason(ctx, "WINTER", nextWinter, cfg.AniList.MaxPerSeason, formats)
		if err != nil {
			slog.Warn("next winter fetch failed, continuing without it",
				"year", nextWinter, "error", err)
		} else {
			shows = filter.Filter(shows, filter.Config{
				Blacklist:   cfg.Blacklist,
				ExcludeTags: cfg.AniList.ExcludeTags,
				AheadMonths: cfg.AniList.AheadMonths,
			})
			shows = filter.FilterFuture(shows, cfg.AniList.AheadMonths)

			var seriesShows, movieShows []anilist.Show
			var seriesNew []anilist.Show
			for _, sh := range shows {
				if sh.IsSeries() {
					seriesShows = append(seriesShows, sh)
					if sh.IsNew() {
						seriesNew = append(seriesNew, sh)
					}
				} else {
					movieShows = append(movieShows, sh)
				}
			}

			key := fmt.Sprintf("WINTER-%d", nextWinter)
			series.shows[key] = seriesShows
			series.new[key] = seriesNew
			movies.shows[key] = movieShows
		}
	}

	type catResult struct {
		label string
		data  map[string][]output.Show
	}
	results := []catResult{
		{label: "series", data: resolveCategory(ctx, resolver, series.shows, dryRun, false)},
		{label: "series-new", data: resolveCategory(ctx, resolver, series.new, dryRun, false)},
		{label: "movies", data: resolveCategory(ctx, resolver, movies.shows, dryRun, true)},
	}

	if dryRun {
		return nil
	}

	for _, r := range results {
		if err := output.WriteAllJSON(outputDir, r.label, r.data); err != nil {
			return fmt.Errorf("write %s JSON: %w", r.label, err)
		}
	}

	var total int
	for _, r := range results {
		for _, shows := range r.data {
			total += len(shows)
		}
	}
	slog.Info("output written", "dir", outputDir, "total_resolved", total)

	return nil
}

func resolveCategory(ctx context.Context, resolver *mapping.Resolver, all map[string][]anilist.Show, dryRun bool, isMovies bool) map[string][]output.Show {
	out := map[string][]output.Show{}
	for key, shows := range all {
		parts := strings.SplitN(key, "-", 2)
		season := parts[0]
		var year int
		fmt.Sscanf(parts[1], "%d", &year)

		rs := resolver.ResolveBatch(ctx, shows, isMovies)

		var resolved []output.Show
			var unmatched int
			for _, show := range shows {
				if r, ok := rs[show.ID]; ok && r.Resolved {
					if isMovies && r.TMDBID <= 0 {
						unmatched++
						continue
					}
					s := output.Show{Title: r.Title}
					if isMovies {
						s.TMDBID = r.TMDBID
					} else {
						s.TVDBID = r.TVDBID
					}
					resolved = append(resolved, s)
				} else {
					unmatched++
				}
			}

			if dryRun {
				fmt.Printf("\n[%s %d] %d shows (%d resolved, %d unmatched)\n",
					season, year, len(shows), len(resolved), unmatched)
				for _, s := range resolved {
					if s.TMDBID > 0 {
						fmt.Printf("  TMDB %d — %s\n", s.TMDBID, s.Title)
					} else {
						fmt.Printf("  TVDB %d — %s\n", s.TVDBID, s.Title)
					}
				}
				continue
			}

		out[key] = resolved
	}
	return out
}

func setupLogging(cfg *config.Config, verbose bool) (func(), error) {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	return logging.Setup(level, cfg.Logging.File)
}
