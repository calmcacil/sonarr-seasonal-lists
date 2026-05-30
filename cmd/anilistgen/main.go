package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/config"
	"github.com/calmcacil/anilistgen/internal/logging"
	"github.com/calmcacil/anilistgen/internal/mdblist"
	"github.com/calmcacil/anilistgen/internal/sync"
)

var version = "dev"      // set via ldflags at build time (e.g. -X main.version=$(svu next))

func main() {
	if err := run(); err != nil {
		var exitErr *exitCodeError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, "error:", exitErr.message)
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type exitCodeError struct {
	message string
	code    int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("exit %d: %s", e.code, e.message)
}

func newExitError(msg string, code int) *exitCodeError {
	return &exitCodeError{message: msg, code: code}
}

func run() error {
	// Parse global flags before subcommands
	var (
		configPath string
		dryRun     bool
		outputDir  string
		verbose    bool
		help       bool
	)

	flags := flag.NewFlagSet("anilistgen", flag.ContinueOnError)
	flags.StringVar(&configPath, "config", "", "path to config file")
	flags.StringVar(&configPath, "c", "", "path to config file (shorthand)")
	flags.BoolVar(&dryRun, "dry-run", false, "print what would be done without making API calls")
	flags.StringVar(&outputDir, "output", "", "write JSON files to directory instead of MDBList")
	flags.StringVar(&outputDir, "o", "", "write JSON files to directory (shorthand)")
	flags.BoolVar(&verbose, "v", false, "verbose logging")
	flags.BoolVar(&verbose, "verbose", false, "verbose logging")
	flags.BoolVar(&help, "h", false, "print help")
	flags.BoolVar(&help, "help", false, "print help")

	// Parse until we hit a subcommand or end
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
			return nil
		}
		return newExitError(err.Error(), 2)
	}

	if help {
		printUsage()
		return nil
	}

	// Determine subcommand from remaining args
	args := flags.Args()
	subcommand := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcommand = args[0]
		args = args[1:]
	}

	switch subcommand {
	case "init-config":
		return runInitConfig(configPath, args)
	case "validate":
		return runValidate(configPath, verbose)
	case "daemon":
		return runDaemon(configPath, dryRun, outputDir, verbose)
	case "", "oneshot", "anilistgen":
		return runOneshot(configPath, dryRun, outputDir, verbose)
	default:
		return newExitError(fmt.Sprintf("unknown subcommand: %q; see anilistgen -h", subcommand), 2)
	}
}

func printUsage() {
	fmt.Println(`anilistgen — generate seasonal anime lists on MDBList from AniList

Usage:
  anilistgen [flags]                    Oneshot mode (default)
  anilistgen daemon [flags]             Background daemon mode
  anilistgen init-config [flags]        Generate default config file
  anilistgen validate [flags]           Validate config and API connectivity

Global flags:
  -config, -c PATH      Path to config file (overrides default search paths)
  -dry-run              Print what would be done without making MDBList API calls
  -output, -o DIR       Write JSON files to DIR instead of MDBList (oneshot only)
  -v, -verbose          Verbose logging
  -h, -help             Print this help

Subcommand-specific flags:
  init-config:
    -force              Overwrite existing config file

  validate:
    (no extra flags)

  daemon:
    (uses interval from config file)

See the full spec at specs/anilist-seasonal-mdblist/PRODUCT.md for details.`)
}

// listCachePath derives the item-cache path from the state-file path.
// e.g. /tmp/anilistgen.lastrun → /tmp/anilistgen_listcache.json
func listCachePath(stateFile string) string {
	if stateFile == "" {
		return ""
	}
	return strings.TrimSuffix(stateFile, ".lastrun") + "_listcache.json"
}

// runOneshot processes all configured seasons once and exits.
func runOneshot(configPath string, dryRun bool, outputDir string, verbose bool) error {
	cfg, _, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if err := setupLogging(cfg, verbose); err != nil {
		return err
	}

	// If output dir is set, use it
	if outputDir != "" {
		// Override config
	}
	// If dry-run is set from CLI, it overrides
	// Actually, the CLI flags should override config but wait — config doesn't have
	// dry-run or output-dir, those are CLI-only per spec.

	apiKey := resolveAPIKey(cfg)

	if !dryRun && outputDir == "" && apiKey == "" {
		return newExitError(
			"MDBList API key required. Set mdblist_api_key in config or MDBLIST_API_KEY env var.",
			1)
	}

	aniClient := anilist.New()
	var mdbClient *mdblist.Client
	if apiKey != "" {
		mdbClient = mdblist.New(apiKey)
	}

	syncCfg := sync.SyncConfig{
		MaxPerSeason:          cfg.AniList.MaxPerSeason,
		IncludeONA:            cfg.AniList.IncludeONA,
		WinterOverflow:        cfg.AniList.WinterOverflow,
		TitleTemplate:         cfg.MDBList.TitleTemplate,
		DescriptionTemplate:   cfg.MDBList.DescriptionTemplate,
		Public:                cfg.MDBList.Public,
		DryRun:                dryRun,
		OutputDir:             outputDir,
		FallbackRelationTypes: cfg.AniList.FallbackRelationTypes,
		ExcludeTags:           cfg.AniList.ExcludeTags,
		ListCachePath:         listCachePath(cfg.StateFile),
	}

	syncer := sync.New(aniClient, mdbClient, syncCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	years := cfg.AniList.YearsOrDefault()
	seasons := cfg.AniList.Season()

	slog.Info("starting sync",
		"years", years,
		"seasons", len(seasons),
		"mode", oneshotMode(dryRun, outputDir))

	var allResults []sync.Result
	for _, year := range years {
		results := syncer.SyncAll(ctx, seasons, year)
		allResults = append(allResults, results...)
	}

	sync.PrintResults(allResults, dryRun || outputDir != "")

	return collectErrors(allResults)
}

// runDaemon runs the sync loop with configurable interval and signal handling.
func runDaemon(configPath string, dryRun bool, outputDir string, verbose bool) error {
	cfg, cfgPath, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if err := setupLogging(cfg, verbose); err != nil {
		return err
	}

	if cfg.Interval.Duration == 0 {
		return newExitError("interval must be non-zero in daemon mode; set it in "+cfgPath, 1)
	}

	apiKey := resolveAPIKey(cfg)
	if apiKey == "" {
		return newExitError(
			"MDBList API key required. Set mdblist_api_key in config or MDBLIST_API_KEY env var.",
			1)
	}

	aniClient := anilist.New()
	mdbClient := mdblist.New(apiKey)

	syncCfg := sync.SyncConfig{
		MaxPerSeason:          cfg.AniList.MaxPerSeason,
		IncludeONA:            cfg.AniList.IncludeONA,
		WinterOverflow:        cfg.AniList.WinterOverflow,
		TitleTemplate:         cfg.MDBList.TitleTemplate,
		DescriptionTemplate:   cfg.MDBList.DescriptionTemplate,
		Public:                cfg.MDBList.Public,
		DryRun:                dryRun,
		FallbackRelationTypes: cfg.AniList.FallbackRelationTypes,
		ExcludeTags:           cfg.AniList.ExcludeTags,
		ListCachePath:         listCachePath(cfg.StateFile),
	}

	syncer := sync.New(aniClient, mdbClient, syncCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("daemon started",
		"interval", cfg.Interval.Duration,
		"run_on_start", cfg.RunOnStart,
		"state_file", cfg.StateFile,
		"config", cfgPath)

	// On the first iteration, either run immediately (RunOnStart=true)
	// or sleep through the interval first (RunOnStart=false).
	mustSleep := !cfg.RunOnStart

	for {
		if mustSleep {
			mustSleep = false
			slog.Debug("run_on_start disabled, sleeping before first cycle")
			select {
			case sig := <-sigCh:
				slog.Info("received signal, shutting down", "signal", sig)
				return nil
			case <-time.After(cfg.Interval.Duration):
			}
		} else {
			// Normal sleep between cycles
			slog.Debug("sleeping", "duration", cfg.Interval.Duration)
			select {
			case sig := <-sigCh:
				slog.Info("received signal, shutting down", "signal", sig)
				return nil
			case <-time.After(cfg.Interval.Duration):
			}
		}

		// Sanity check: skip if last run was within MinInterval
		if !checkLastRun(cfg.StateFile, config.MinInterval) {
			slog.Debug("skipping sync — last run too recent (less than MinInterval)")
			continue
		}

		years := cfg.AniList.YearsOrDefault()
		seasons := cfg.AniList.Season()

		slog.Info("sync cycle starting",
			"years", years,
			"seasons", len(seasons))

		var allResults []sync.Result
		cycleCtx, cycleCancel := context.WithTimeout(ctx, 5*time.Minute)
		for _, year := range years {
			results := syncer.SyncAll(cycleCtx, seasons, year)
			allResults = append(allResults, results...)
		}
		cycleCancel()

		for _, r := range allResults {
			if r.Error != nil {
				slog.Error("sync failed",
					"season", r.Season,
					"year", r.Year,
					"error", r.Error)
			} else {
				status := "created"
				if r.Updated {
					status = "updated"
				} else {
					status = "unchanged"
				}
				slog.Info("sync result",
					"season", r.Season,
					"year", r.Year,
					"list", r.ListTitle,
					"shows", r.ShowCount,
					"status", status,
					"url", r.ListURL)
			}
		}

		if err := collectErrors(allResults); err != nil {
			slog.Warn("sync cycle had errors", "error", err)
		} else {
			slog.Info("sync cycle completed successfully")
			writeLastRun(cfg.StateFile)
		}
	}
}

// runValidate validates config and API connectivity.
func runValidate(configPath string, verbose bool) error {
	cfg, cfgPath, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := setupLogging(cfg, verbose); err != nil {
		return err
	}

	fmt.Printf("Config: %s\n", cfgPath)
	fmt.Printf("  Years: %v\n", cfg.AniList.YearsOrDefault())
	fmt.Printf("  Seasons: %s\n", strings.Join(cfg.AniList.Season(), ", "))
	fmt.Printf("  Max per season: %d\n", cfg.AniList.MaxPerSeason)
	fmt.Printf("  Include ONA: %t\n", cfg.AniList.IncludeONA)
	fmt.Printf("  List template: %q\n", cfg.MDBList.TitleTemplate)
	fmt.Printf("  Public: %t\n", cfg.MDBList.Public)
	fmt.Printf("  Log level: %s\n", cfg.Logging.Level)

	apiKey := resolveAPIKey(cfg)
	if apiKey == "" {
		fmt.Println("  MDBList API key: not set")
	} else {
		fmt.Println("  MDBList API key: set")
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("\n❌ Config validation failed:\n  %s\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✓ Config is valid")

	// Test AniList connectivity
	fmt.Print("Testing AniList API... ")
	aniClient := anilist.New()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err = aniClient.Ping(ctx)
	cancel()
	if err != nil {
		fmt.Printf("❌ %s\n", err)
	} else {
		fmt.Println("✓ reachable")
	}

	// Test MDBList connectivity
	if apiKey != "" {
		fmt.Print("Testing MDBList API... ")
		mdbClient := mdblist.New(apiKey)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = mdbClient.Ping(ctx)
		cancel()
		if err != nil {
			fmt.Printf("❌ %s\n", err)
		} else {
			fmt.Println("✓ reachable")
		}
	} else {
		fmt.Println("Skipping MDBList check: no API key set")
	}

	fmt.Println("\n✓ Validation complete")
	return nil
}

// runInitConfig generates a default config file.
func runInitConfig(cliPath string, args []string) error {
	force := false
	// Parse remaining args for -force
	fset := flag.NewFlagSet("init-config", flag.ContinueOnError)
	fset.BoolVar(&force, "force", false, "overwrite existing config file")
	if err := fset.Parse(args); err != nil {
		return newExitError(err.Error(), 2)
	}

	path := config.ResolveConfigPath(cliPath)

	// Check if file already exists
	if _, err := os.Stat(path); err == nil && !force {
		return newExitError(
			fmt.Sprintf("config file already exists at %s; use -force to overwrite", path), 1)
	}

	if err := config.WriteDefaultConfig(path); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	fmt.Printf("Default config written to %s\n", path)
	fmt.Println("Edit the file to set your MDBList API key and preferences.")
	return nil
}

// setupLogging configures the slog logger.
func setupLogging(cfg *config.Config, verbose bool) error {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	return logging.Setup(level, cfg.Logging.File)
}

// resolveAPIKey returns the MDBList API key from config (env overrides already applied).
func resolveAPIKey(cfg *config.Config) string {
	return cfg.MDBListAPIKey
}

// checkLastRun returns true if at least minInterval has elapsed since the last run
// recorded in the state file. If the file doesn't exist (first run), returns true.
func checkLastRun(path string, minInterval time.Duration) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist or can't be read — first run or stateless
		return true
	}

	var lastRun time.Time
	if err := lastRun.UnmarshalText(data); err != nil {
		return true
	}

	elapsed := time.Since(lastRun)
	if elapsed < minInterval {
		slog.Debug("last run was too recent",
			"last_run", lastRun,
			"elapsed", elapsed.Round(time.Second),
			"min_interval", minInterval)
		return false
	}
	return true
}

// writeLastRun records the current timestamp in the state file.
func writeLastRun(path string) {
	now := time.Now()
	data, err := now.MarshalText()
	if err != nil {
		slog.Debug("failed to marshal timestamp", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Debug("failed to write state file", "path", path, "error", err)
	}
}

// collectErrors checks results for errors and returns a combined error if any.
func collectErrors(results []sync.Result) error {
	var errs []string
	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s %d: %s", r.Season, r.Year, r.Error))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d sync error(s):\n  - %s", len(errs), strings.Join(errs, "\n  - "))
	}
	return nil
}

// oneshotMode returns a human-readable mode description.
func oneshotMode(dryRun bool, outputDir string) string {
	if dryRun {
		return "dry-run"
	}
	if outputDir != "" {
		return "file-output"
	}
	return "mdblist"
}
