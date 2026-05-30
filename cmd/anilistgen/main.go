package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
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
		showVer    bool
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
	flags.BoolVar(&showVer, "version", false, "print version and exit")
	flags.BoolVar(&showVer, "V", false, "print version and exit (shorthand)")

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

	if showVer {
		fmt.Println(version)
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
	dir := filepath.Dir(stateFile)
	base := filepath.Base(stateFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+"_listcache.json")
}

// manualMatchPath derives the manual-match file path from the state-file path.
// e.g. /tmp/anilistgen.lastrun → /tmp/anilistgen_manual.yml
func manualMatchPath(stateFile string) string {
	if stateFile == "" {
		return ""
	}
	dir := filepath.Dir(stateFile)
	base := filepath.Base(stateFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+"_manual.yml")
}

// runOneshot processes all configured seasons once and exits.
func runOneshot(configPath string, dryRun bool, outputDir string, verbose bool) error {
	cfg, _, err := config.Load(configPath)
	if err != nil {
		return err
	}

	closeLog, err := setupLogging(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeLog()

	apiKey := cfg.MDBListAPIKey

	if !dryRun && outputDir == "" && apiKey == "" {
		return newExitError(
			"MDBList API key required. Set mdblist_api_key in config or ALG_MDBLIST_API_KEY env var.",
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
		AheadMonths:           cfg.AniList.AheadMonths,
		TitleTemplate:         cfg.MDBList.TitleTemplate,
		DescriptionTemplate:   cfg.MDBList.DescriptionTemplate,
		Public:                cfg.MDBList.Public,
		DryRun:                dryRun,
		OutputDir:             outputDir,
		FallbackRelationTypes: cfg.AniList.FallbackRelationTypes,
		ExcludeTags:           cfg.AniList.ExcludeTags,
		ListCachePath:         listCachePath(cfg.StateFile),
		ManualMatchFile:       manualMatchPath(cfg.StateFile),
	}

	syncer := sync.New(aniClient, mdbClient, syncCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	closeLog, err := setupLogging(cfg, verbose)
	if err != nil {
		return err
	}
	defer closeLog()

	if cfg.Interval.Duration == 0 {
		return newExitError("interval must be non-zero in daemon mode; set it in "+cfgPath, 1)
	}

	apiKey := cfg.MDBListAPIKey
	if apiKey == "" {
		return newExitError(
			"MDBList API key required. Set mdblist_api_key in config or ALG_MDBLIST_API_KEY env var.",
			1)
	}

	aniClient := anilist.New()
	mdbClient := mdblist.New(apiKey)

	syncCfg := sync.SyncConfig{
		MaxPerSeason:          cfg.AniList.MaxPerSeason,
		IncludeONA:            cfg.AniList.IncludeONA,
		WinterOverflow:        cfg.AniList.WinterOverflow,
		AheadMonths:           cfg.AniList.AheadMonths,
		TitleTemplate:         cfg.MDBList.TitleTemplate,
		DescriptionTemplate:   cfg.MDBList.DescriptionTemplate,
		Public:                cfg.MDBList.Public,
		DryRun:                dryRun,
		FallbackRelationTypes: cfg.AniList.FallbackRelationTypes,
		ExcludeTags:           cfg.AniList.ExcludeTags,
		ListCachePath:         listCachePath(cfg.StateFile),
		ManualMatchFile:       manualMatchPath(cfg.StateFile),
	}

	syncer := sync.New(aniClient, mdbClient, syncCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling — also listen for SIGHUP for config/log reload.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	slog.Info("daemon started",
		"interval", cfg.Interval.Duration,
		"run_on_start", cfg.RunOnStart,
		"state_file", cfg.StateFile,
		"config", cfgPath)

	// On the first iteration, either run immediately (RunOnStart=true)
	// or sleep through the interval first (RunOnStart=false).
	mustSleep := !cfg.RunOnStart

	// reloadConfig reloads the YAML file, rebuilds the syncer, and reopens
	// the log file. Called on SIGHUP.
	reloadConfig := func() {
		slog.Info("SIGHUP received, reloading config")
		newCfg, newCfgPath, err := config.Load(configPath)
		if err != nil {
			slog.Warn("config reload failed, keeping current config", "error", err)
			return
		}

		// Reopen log file with new config
		newClose, logErr := setupLogging(newCfg, verbose)
		if logErr != nil {
			slog.Warn("log reopen after reload failed, keeping old logger", "error", logErr)
		} else {
			closeLog() // close old log file
			closeLog = newClose
		}

		// Rebuild syncer with new config
		apiKey := newCfg.MDBListAPIKey
		var mdbClient *mdblist.Client
		if apiKey != "" {
			mdbClient = mdblist.New(apiKey)
		}

		newSyncCfg := sync.SyncConfig{
			MaxPerSeason:          newCfg.AniList.MaxPerSeason,
			IncludeONA:            newCfg.AniList.IncludeONA,
			WinterOverflow:        newCfg.AniList.WinterOverflow,
			AheadMonths:           newCfg.AniList.AheadMonths,
			TitleTemplate:         newCfg.MDBList.TitleTemplate,
			DescriptionTemplate:   newCfg.MDBList.DescriptionTemplate,
			Public:                newCfg.MDBList.Public,
			DryRun:                dryRun,
			FallbackRelationTypes: newCfg.AniList.FallbackRelationTypes,
			ExcludeTags:           newCfg.AniList.ExcludeTags,
			ListCachePath:         listCachePath(newCfg.StateFile),
			ManualMatchFile:       manualMatchPath(newCfg.StateFile),
		}

		cfg = newCfg
		cfgPath = newCfgPath
		syncer = sync.New(aniClient, mdbClient, newSyncCfg)
		slog.Info("config reloaded", "config", cfgPath)
	}

	var shuttingDown bool

	for {
		if mustSleep {
			mustSleep = false
			slog.Debug("run_on_start disabled, sleeping before first cycle")
			select {
			case sig := <-sigCh:
				if sig == syscall.SIGHUP {
					reloadConfig()
					mustSleep = true // restart sleep after reload
					continue
				}
				slog.Info("received signal, completing current cycle then shutting down", "signal", sig)
				shuttingDown = true
			case <-time.After(cfg.Interval.Duration):
			}
		} else {
			// Normal sleep between cycles
			slog.Debug("sleeping", "duration", cfg.Interval.Duration)
			select {
			case sig := <-sigCh:
				if sig == syscall.SIGHUP {
					reloadConfig()
					continue
				}
				slog.Info("received signal, completing current cycle then shutting down", "signal", sig)
				shuttingDown = true
			case <-ctx.Done():
				slog.Info("context cancelled, completing current cycle then shutting down")
				shuttingDown = true
			case <-time.After(cfg.Interval.Duration):
			}
		}

		// Sanity check: skip if last run was within MinInterval
		if !checkLastRun(cfg.StateFile, config.MinInterval) {
			if shuttingDown {
				slog.Info("last run too recent, skipping cycle and shutting down")
				return nil
			}
			slog.Debug("skipping sync — last run too recent (less than MinInterval)")
			continue
		}

		// Panic recovery per cycle
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("daemon cycle panicked, recovering",
						"panic", r,
						"stack", string(debug.Stack()))
				}
			}()

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
		}()

		if shuttingDown {
			slog.Info("sync cycle completed, shutting down")
			return nil
		}
	}
}

// runValidate validates config and API connectivity.
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
	fmt.Printf("  Seasons: %s\n", strings.Join(cfg.AniList.Season(), ", "))
	fmt.Printf("  Max per season: %d\n", cfg.AniList.MaxPerSeason)
	fmt.Printf("  Include ONA: %t\n", cfg.AniList.IncludeONA)
	fmt.Printf("  List template: %q\n", cfg.MDBList.TitleTemplate)
	fmt.Printf("  Public: %t\n", cfg.MDBList.Public)
	fmt.Printf("  Log level: %s\n", cfg.Logging.Level)

	apiKey := cfg.MDBListAPIKey
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

// setupLogging configures the slog logger and returns a close function
// to flush/close the underlying log file (no-op for stderr).
func setupLogging(cfg *config.Config, verbose bool) (func(), error) {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	return logging.Setup(level, cfg.Logging.File)
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
