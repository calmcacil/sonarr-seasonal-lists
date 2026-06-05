package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/config"
	"github.com/calmcacil/anilistgen/internal/filter"
	"github.com/calmcacil/anilistgen/internal/logging"
	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/model"
	"github.com/calmcacil/anilistgen/internal/output"
	"github.com/calmcacil/anilistgen/internal/pipeline"
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
	fmt.Printf("  Max per year: %d\n", cfg.AniList.MaxPerYear)
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

	am, err := loadMapping(cfg)
	if err != nil {
		return fmt.Errorf("load anibridge mapping: %w", err)
	}

	resolver := mapping.NewResolver(am)
	aniClient := anilist.New()

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

	deps := pipeline.Deps{
		AniClient:      aniClient,
		Resolver:       resolver,
		FilterConfig: filter.Config{
			Blacklist:   cfg.Blacklist,
			ExcludeTags: cfg.AniList.ExcludeTags,
		},
		WinterOverflow: cfg.AniList.WinterOverflow,
		MaxPerYear:     cfg.AniList.MaxPerYear,
		AheadMonths:    cfg.AniList.AheadMonthsOrDefault(),
		Formats:        formats,
	}

	seriesAll, seriesNew, stats, errs := pipeline.Run(ctx, deps, cfg.AniList.YearsOrDefault(), cfg.AniList.Season())

	expectedSeasons := len(cfg.AniList.YearsOrDefault()) * len(cfg.AniList.Season())

	if dryRun {
		printDryRun(seriesAll, "series")
		printDryRun(seriesNew, "series-new")
		return nil
	}

	for _, s := range stats {
		slog.Info("season stats",
			"season", s.Season, "year", s.Year,
			"fetched", s.Fetched, "resolved", s.Resolved, "unmatched", s.Unmatched)
	}

	if len(errs) > 0 {
		for _, e := range errs {
			slog.Warn("pipeline error", "error", e)
		}
		if len(errs) == expectedSeasons {
			return fmt.Errorf("all %d seasons failed", len(errs))
		}
	}

	if err := output.WriteAllJSON(outputDir, cfg.BaseURL, "series", seriesAll, cfg.IndexYears); err != nil {
		return fmt.Errorf("write series JSON: %w", err)
	}
	if err := output.WriteAllJSON(outputDir, cfg.BaseURL, "series-new", seriesNew, cfg.IndexYears); err != nil {
		return fmt.Errorf("write series-new JSON: %w", err)
	}

	var total int
	for _, shows := range seriesAll {
		total += len(shows)
	}
	for _, shows := range seriesNew {
		total += len(shows)
	}
	slog.Info("output written", "dir", outputDir, "total_resolved", total)

	return nil
}

func printDryRun(data map[model.SeasonKey][]output.Show, label string) {
	for key, shows := range data {
		fmt.Printf("\n[%s] %s %d — %d shows\n", label, key.Season, key.Year, len(shows))
		for _, s := range shows {
			fmt.Printf("  TVDB %d — %s\n", s.TVDBID, s.Title)
		}
	}
}

func setupLogging(cfg *config.Config, verbose bool) (func(), error) {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	return logging.Setup(level, cfg.Logging.File)
}

func loadMapping(cfg *config.Config) (*mapping.AnibridgeMapping, error) {
	if cfg.AnibridgeMappingMaxAge != "" {
		maxAge, err := time.ParseDuration(cfg.AnibridgeMappingMaxAge)
		if err != nil {
			return nil, fmt.Errorf("parse anibridge_mapping_max_age %q: %w", cfg.AnibridgeMappingMaxAge, err)
		}
		return mapping.LoadAnibridgeMappingWithAge(cfg.AnibridgeMappingPath, maxAge)
	}
	return mapping.LoadAnibridgeMapping(cfg.AnibridgeMappingPath)
}
