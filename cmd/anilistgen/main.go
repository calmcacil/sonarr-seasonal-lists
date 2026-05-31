package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/config"
	"github.com/calmcacil/anilistgen/internal/filter"
	"github.com/calmcacil/anilistgen/internal/logging"
	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/output"
)

// version is set at build time via -ldflags, e.g.:
//
//	go build -ldflags="-X main.version=$(git describe --tags)" ./cmd/anilistgen
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
		return fmt.Errorf("parse flags: %w", err)
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
	if len(args) > 0 {
		subcommand = args[0]
	}

	switch subcommand {
	case "init-config":
		return runInitConfig(configPath)
	case "validate":
		return runValidate(configPath, verbose)
	case "help", "h":
		printUsage()
		return nil
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

	aniClient := anilist.New()
	resolver := mapping.NewResolver(cm)

	formats := []string{"TV"}
	if cfg.AniList.IncludeONA {
		formats = append(formats, "ONA")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.AniList.TimeoutMinutes)*time.Minute)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			slog.Warn("received signal, shutting down", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	aheadMonths := cfg.AniList.AheadMonthsOrDefault()
	years := cfg.AniList.YearsOrDefault()
	seasons := cfg.AniList.Season()

	seriesShows := map[string][]anilist.Show{}
	seriesNew := map[string][]anilist.Show{}

	for _, year := range years {
		for _, season := range seasons {
			seasonSeries, seasonNew, err := processSeason(ctx, aniClient, cfg, season, year, formats, aheadMonths)
			if err != nil {
				continue
			}

			key := fmt.Sprintf("%s-%d", season, year)
			seriesShows[key] = seasonSeries
			seriesNew[key] = seasonNew
		}
	}

	if len(years) > 0 && len(seasons) == 4 {
		fetchAndAppendNextWinter(ctx, aniClient, cfg, years[len(years)-1], formats, aheadMonths, seriesShows, seriesNew)
	}

	type catResult struct {
		label string
		data  map[string][]output.Show
	}
	results := []catResult{
		{label: "series", data: resolveBatch(resolver, seriesShows, dryRun)},
		{label: "series-new", data: resolveBatch(resolver, seriesNew, dryRun)},
	}

	if dryRun {
		return nil
	}

	for _, r := range results {
		if err := output.WriteAllJSON(outputDir, cfg.BaseURL, r.label, r.data, cfg.IndexYears); err != nil {
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

func processSeason(ctx context.Context, client *anilist.Client, cfg *config.Config, season string, year int, formats []string, aheadMonths int) ([]anilist.Show, []anilist.Show, error) {
	slog.Info("fetching season", "season", season, "year", year)

	shows, err := client.FetchSeason(ctx, season, year, cfg.AniList.MaxPerSeason, formats)
	if err != nil {
		slog.Error("fetch failed", "season", season, "year", year, "error", err)
		return nil, nil, err
	}

	if cfg.AniList.WinterOverflow && season == "WINTER" {
		shows = fetchWinterOverflow(ctx, client, year, cfg.AniList.MaxPerSeason, formats, shows)
	}

	if season == "WINTER" {
		shows = filterWinterMonth(shows, "winter shows")
	}

	slog.Info("fetched shows from AniList",
		"season", season, "year", year, "count", len(shows))

	seasonSeries, seasonNew := splitSeriesNew(shows)

	seasonSeries = filter.Filter(seasonSeries, filter.Config{
		Blacklist:   cfg.Blacklist,
		ExcludeTags: cfg.AniList.ExcludeTags,
	})
	seasonSeries = filter.FilterFuture(seasonSeries, aheadMonths)

	seasonNew = filter.Filter(seasonNew, filter.Config{
		Blacklist:   cfg.Blacklist,
		ExcludeTags: cfg.AniList.ExcludeTags,
	})
	seasonNew = filter.FilterFuture(seasonNew, aheadMonths)

	return seasonSeries, seasonNew, nil
}

func fetchWinterOverflow(ctx context.Context, client *anilist.Client, year, maxPerSeason int, formats []string, shows []anilist.Show) []anilist.Show {
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

func fetchAndAppendNextWinter(ctx context.Context, client *anilist.Client, cfg *config.Config, lastYear int, formats []string, aheadMonths int, seriesShows, seriesNew map[string][]anilist.Show) {
	nextWinter := lastYear + 1

	slog.Info("all seasons enabled, also fetching next winter",
		"season", "WINTER", "year", nextWinter)

	shows, err := client.FetchSeason(ctx, "WINTER", nextWinter, cfg.AniList.MaxPerSeason, formats)
	if err != nil {
		slog.Warn("next winter fetch failed, continuing without it",
			"year", nextWinter, "error", err)
		return
	}

	if cfg.AniList.WinterOverflow && nextWinter >= time.Now().Year() {
		shows = fetchWinterOverflow(ctx, client, nextWinter, cfg.AniList.MaxPerSeason, formats, shows)
	}

	shows = filterWinterMonth(shows, "next winter shows")

	shows = filter.Filter(shows, filter.Config{
		Blacklist:   cfg.Blacklist,
		ExcludeTags: cfg.AniList.ExcludeTags,
	})
	shows = filter.FilterFuture(shows, aheadMonths)

	var seasonSeries, seasonNew []anilist.Show
	seasonSeries, seasonNew = splitSeriesNew(shows)

	key := fmt.Sprintf("WINTER-%d", nextWinter)
	seriesShows[key] = seasonSeries
	seriesNew[key] = seasonNew
}

func resolveBatch(resolver *mapping.Resolver, all map[string][]anilist.Show, dryRun bool) map[string][]output.Show {
	out := map[string][]output.Show{}
	for key, shows := range all {
		parts := strings.SplitN(key, "-", 2)
		season := parts[0]
		var year int
		if y, err := strconv.Atoi(parts[1]); err == nil {
			year = y
		} else {
			slog.Error("invalid season key", "key", key)
			continue
		}

		rs := resolver.ResolveBatch(shows)

		var resolved []output.Show
		var unmatched int
		for _, show := range shows {
			if r, ok := rs[show.ID]; ok && r.Resolved {
				resolved = append(resolved, output.Show{
					TVDBID: r.TVDBID,
					Title:  r.Title,
				})
			} else {
				unmatched++
			}
		}

		if dryRun {
			fmt.Printf("\n[%s %d] %d shows (%d resolved, %d unmatched)\n",
				season, year, len(shows), len(resolved), unmatched)
			for _, s := range resolved {
				fmt.Printf("  TVDB %d — %s\n", s.TVDBID, s.Title)
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

func filterWinterMonth(shows []anilist.Show, label string) []anilist.Show {
	var filtered []anilist.Show
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

func splitSeriesNew(shows []anilist.Show) (series, seasonNew []anilist.Show) {
	series = make([]anilist.Show, 0)
	seasonNew = make([]anilist.Show, 0)
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
