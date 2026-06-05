package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/calmcacil/anilistgen/internal/mapping"
	"gopkg.in/yaml.v3"
)

type Config struct {
	AniList               AniListConfig `yaml:"anilist"`
	Blacklist             []string      `yaml:"blacklist"`
	OutputDir             string        `yaml:"output_dir"`
	BaseURL               string        `yaml:"base_url"`
	AnibridgeMappingPath  string        `yaml:"anibridge_mapping_path"`
	AnibridgeMappingMaxAge string       `yaml:"anibridge_mapping_max_age"`
	Logging               LoggingConfig `yaml:"logging"`
	IndexYears            []int         `yaml:"index_years"`
}

type AniListConfig struct {
	Years          []int    `yaml:"years"`
	Seasons        []string `yaml:"seasons"`
	MaxPerYear     int      `yaml:"max_per_year"`
	IncludeONA     bool     `yaml:"include_ona"`
	WinterOverflow bool     `yaml:"winter_overflow"`
	AheadMonths    *int     `yaml:"ahead_months"`
	ExcludeTags    []string `yaml:"exclude_tags"`
	TimeoutMinutes int      `yaml:"timeout_minutes"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

const (
	DefaultMaxPerYear = 400
	DefaultBaseURL    = "https://lists.calmcacil.dev"
)

// Season resolves the configured season strings to uppercase season codes.
// If seasons are empty or contains "all" (case-insensitive), returns all four
// seasons. Duplicates are removed.
func (a *AniListConfig) Season() []string {
	if len(a.Seasons) == 0 {
		return AllSeasons()
	}
	seen := make(map[string]bool, len(a.Seasons))
	seasons := make([]string, 0, len(a.Seasons))
	for _, s := range a.Seasons {
		if strings.EqualFold(s, "all") {
			return AllSeasons()
		}
		var season string
		switch strings.ToLower(s) {
		case "winter":
			season = "WINTER"
		case "spring":
			season = "SPRING"
		case "summer":
			season = "SUMMER"
		case "fall":
			season = "FALL"
		}
		if season != "" && !seen[season] {
			seen[season] = true
			seasons = append(seasons, season)
		}
	}
	return seasons
}

// AllSeasons returns the four standard anime seasons in uppercase.
func AllSeasons() []string {
	return []string{"WINTER", "SPRING", "SUMMER", "FALL"}
}

func (a *AniListConfig) YearsOrDefault() []int {
	if len(a.Years) > 0 {
		return a.Years
	}
	return []int{time.Now().Year()}
}

func (a *AniListConfig) AheadMonthsOrDefault() int {
	if a.AheadMonths != nil {
		return *a.AheadMonths
	}
	return 3
}

func (c *Config) FillDefaults() {
	if c.AniList.MaxPerYear <= 0 {
		c.AniList.MaxPerYear = DefaultMaxPerYear
	}
	if c.AniList.AheadMonths == nil {
		v := 3
		c.AniList.AheadMonths = &v
	}
	if c.AniList.TimeoutMinutes <= 0 {
		c.AniList.TimeoutMinutes = 10
	}
	if c.OutputDir == "" {
		c.OutputDir = "./sonarr-lists"
	}
	if c.AnibridgeMappingPath == "" {
		c.AnibridgeMappingPath = mapping.DefaultAnibridgePath()
	}
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
}

func (c *Config) Validate() error {
	var errs []string

	years := c.AniList.YearsOrDefault()
	for _, y := range years {
		if y < 2000 || y > 2100 {
			errs = append(errs, fmt.Sprintf("anilist year %d is out of range (2000-2100)", y))
		}
	}

	if c.AniList.MaxPerYear < 1 || c.AniList.MaxPerYear > 2000 {
		errs = append(errs, fmt.Sprintf("anilist.max_per_year %d must be between 1 and 2000", c.AniList.MaxPerYear))
	}

	seasons := c.AniList.Season()
	if len(seasons) == 0 {
		errs = append(errs, "anilist.seasons must specify at least one valid season")
	}

	switch strings.ToLower(c.Logging.Level) {
	case "debug", "info", "warn", "error", "":
	default:
		errs = append(errs, fmt.Sprintf("logging.level %q is invalid; must be debug, info, warn, or error", c.Logging.Level))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// DefaultConfig returns a new Config with sensible defaults: all seasons,
// current year, winter overflow enabled, 3-month ahead window, and info logging.
func DefaultConfig() *Config {
	v := 3
	return &Config{
		AniList: AniListConfig{
			Years:          nil,
			Seasons:        []string{"all"},
			MaxPerYear:     DefaultMaxPerYear,
			IncludeONA:     false,
			WinterOverflow: true,
			AheadMonths:    &v,
			TimeoutMinutes: 10,
		},
		OutputDir:             "./sonarr-lists",
		AnibridgeMappingPath: mapping.DefaultAnibridgePath(),
		BaseURL:               DefaultBaseURL,
		Logging: LoggingConfig{
			Level: "info",
			File:  "",
		},
	}
}

const envPrefix = "ALG_"

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv(envPrefix + "ANILIST_YEARS"); v != "" {
		parts := strings.Split(v, ",")
		var years []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if y, err := strconv.Atoi(p); err == nil && y > 0 {
				years = append(years, y)
			}
		}
		if len(years) > 0 {
			c.AniList.Years = years
		}
	}

	if v := os.Getenv(envPrefix + "ANILIST_SEASONS"); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		c.AniList.Seasons = parts
	}

	if v := os.Getenv(envPrefix + "ANILIST_MAX_PER_YEAR"); v != "" {
		if max, err := strconv.Atoi(v); err == nil && max > 0 {
			c.AniList.MaxPerYear = max
		}
	}

	if v := os.Getenv(envPrefix + "ANILIST_INCLUDE_ONA"); v != "" {
		c.AniList.IncludeONA = v == "1" || strings.EqualFold(v, "true")
	}

	if v := os.Getenv(envPrefix + "ANILIST_WINTER_OVERFLOW"); v != "" {
		c.AniList.WinterOverflow = v == "1" || strings.EqualFold(v, "true")
	}

	if v := os.Getenv(envPrefix + "ANILIST_EXCLUDE_TAGS"); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		c.AniList.ExcludeTags = parts
	}

	if v := os.Getenv(envPrefix + "ANILIST_AHEAD_MONTHS"); v != "" {
		if m, err := strconv.Atoi(v); err == nil && m >= 0 {
			c.AniList.AheadMonths = &m
		}
	}

	if v := os.Getenv(envPrefix + "BLACKLIST"); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		c.Blacklist = parts
	}

	if v := os.Getenv(envPrefix + "OUTPUT_DIR"); v != "" {
		c.OutputDir = v
	}

	if v := os.Getenv(envPrefix + "BASE_URL"); v != "" {
		c.BaseURL = v
	}

	if v := os.Getenv(envPrefix + "INDEX_YEARS"); v != "" {
		parts := strings.Split(v, ",")
		var years []int
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if y, err := strconv.Atoi(p); err == nil && y >= 2010 {
				years = append(years, y)
			}
		}
		if len(years) > 0 {
			c.IndexYears = years
		}
	}

	if v := os.Getenv(envPrefix + "ANIBRIDGE_MAPPING_PATH"); v != "" {
		c.AnibridgeMappingPath = v
	}

	if v := os.Getenv(envPrefix + "ANIBRIDGE_MAPPING_MAX_AGE"); v != "" {
		c.AnibridgeMappingMaxAge = v
	}

	if v := os.Getenv(envPrefix + "LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}

	if v := os.Getenv(envPrefix + "LOG_FILE"); v != "" {
		c.Logging.File = v
	}

	if v := os.Getenv(envPrefix + "ANILIST_TIMEOUT_MINUTES"); v != "" {
		if m, err := strconv.Atoi(v); err == nil && m > 0 {
			c.AniList.TimeoutMinutes = m
		}
	}
}

// Load searches for a config file using the provided path and default search
// paths. Returns the loaded config, the file path used, or an error. If no file
// is found, returns a default config with an empty path.
func Load(path string) (*Config, string, error) {
	paths := searchPaths(path)

	for _, p := range paths {
		cfg, err := loadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("config error in %s: %w", p, err)
		}
		cfg.FillDefaults()
		cfg.applyEnvOverrides()
		return cfg, p, nil
	}

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()
	return cfg, "", nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	knownKeys := map[string]bool{
		"anilist":                  true,
		"blacklist":                true,
		"output_dir":               true,
		"base_url":                 true,
		"index_years":              true,
		"anibridge_mapping_path":   true,
		"anibridge_mapping_max_age": true,
		"logging":                  true,
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for k := range raw {
		if !knownKeys[k] {
			fmt.Fprintf(os.Stderr, "warning: config file %s contains unknown key %q\n", path, k)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &cfg, nil
}

func xdgConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func searchPaths(cliPath string) []string {
	if cliPath != "" {
		return []string{cliPath}
	}
	paths := []string{filepath.Join(".", "anilistgen.yaml")}
	xdg := xdgConfigHome()
	if xdg != "" {
		paths = append(paths, filepath.Join(xdg, "anilistgen", "anilistgen.yaml"))
	}
	return paths
}

// WriteDefaultConfig writes a documented default configuration YAML file to
// the given path, creating directories as needed.
func WriteDefaultConfig(path string) error {
	content := `# anilistgen configuration

# AniList query settings
anilist:
  # Years to process
  years:
    - 2026

  # Seasons to include: winter, spring, summer, fall, or "all"
  seasons:
    - all

  # Maximum results per year (applied before internal season splitting). Default: 400
  max_per_year: 400

  # Include ONA format alongside TV. Default: false
  include_ona: false

  # Merge December-premiering shows from the previous year into WINTER.
  # When enabled, fetches the prior year's WINTER and merges only
  # shows that started in December (startDate.month == 12). Default: true
  winter_overflow: true

  # Skip shows starting more than N months ahead. Default: 3
  ahead_months: 3

  # Context timeout in minutes. Increase for large backfills. Default: 10
  timeout_minutes: 10

  # AniList content tags to exclude. Shows with any matching tag
  # (case-insensitive) are skipped entirely.
  # exclude_tags:
  #   - "Hentai"

# Filter
blacklist: []

# Output directory for JSON files
output_dir: ./sonarr-lists

# Base URL for the generated index page (used for copy-to-clipboard URLs)
base_url: https://lists.calmcacil.dev

# Anibridge mapping file path (auto-downloaded if missing). zstd-compressed JSON.
anibridge_mapping_path: /tmp/anilistgen_anibridge.json.zst

# Logging
logging:
  level: info
  file: ""
`

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("write config to %s: %w", path, err)
	}

	return nil
}

// ResolveConfigPath returns the path to an existing config file, or the
// default XDG config path if none exists. If cliPath is non-empty, returns it.
func ResolveConfigPath(cliPath string) string {
	if cliPath != "" {
		return cliPath
	}
	paths := searchPaths("")
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(xdgConfigHome(), "anilistgen", "anilistgen.yaml")
}
