package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AniList  AniListConfig  `yaml:"anilist"`
	Blacklist []string      `yaml:"blacklist"`
	OutputDir string        `yaml:"output_dir"`
	CommunityMappingPath string `yaml:"community_mapping_path"`
	AnimeListsPath       string `yaml:"anime_lists_path"`
	TMDBAPIKey string        `yaml:"tmdb_api_key"`
	Logging  LoggingConfig `yaml:"logging"`
	Sonarr   SonarrConfig  `yaml:"sonarr"`
}

type AniListConfig struct {
	Years         []int    `yaml:"years"`
	Seasons       []string `yaml:"seasons"`
	MaxPerSeason  int      `yaml:"max_per_season"`
	IncludeONA    bool     `yaml:"include_ona"`
	WinterOverflow bool    `yaml:"winter_overflow"`
	AheadMonths   int      `yaml:"ahead_months"`
	ExcludeTags   []string `yaml:"exclude_tags"`
}

type SonarrConfig struct {
	URL           string `yaml:"url"`
	APIKey        string `yaml:"api_key"`
	QualityProfile string `yaml:"quality_profile"`
	RootFolder     string `yaml:"root_folder"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

const (
	DefaultMaxPerSeason = 100
	DefaultStateFile    = "/tmp/anilistgen.lastrun"
	DefaultMappingPath  = "/tmp/anilistgen_tvdb.yaml"
	DefaultAnimeListsPath = "/tmp/anime-list-full.xml"
)

func (a *AniListConfig) Season() []string {
	if len(a.Seasons) == 0 {
		return AllSeasons()
	}
	seasons := make([]string, 0, len(a.Seasons))
	for _, s := range a.Seasons {
		if strings.EqualFold(s, "all") {
			return AllSeasons()
		}
		switch strings.ToLower(s) {
		case "winter":
			seasons = append(seasons, "WINTER")
		case "spring":
			seasons = append(seasons, "SPRING")
		case "summer":
			seasons = append(seasons, "SUMMER")
		case "fall":
			seasons = append(seasons, "FALL")
		}
	}
	return seasons
}

func AllSeasons() []string {
	return []string{"WINTER", "SPRING", "SUMMER", "FALL"}
}

func (a *AniListConfig) YearsOrDefault() []int {
	if len(a.Years) > 0 {
		return a.Years
	}
	return []int{time.Now().Year()}
}

func (c *Config) FillDefaults() {
	if c.AniList.MaxPerSeason <= 0 {
		c.AniList.MaxPerSeason = DefaultMaxPerSeason
	}
	if c.AniList.AheadMonths == 0 {
		c.AniList.AheadMonths = 3
	}
	if c.OutputDir == "" {
		c.OutputDir = "./sonarr-lists"
	}
	if c.CommunityMappingPath == "" {
		c.CommunityMappingPath = DefaultMappingPath
	}
	if c.AnimeListsPath == "" {
		c.AnimeListsPath = DefaultAnimeListsPath
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

	if c.AniList.MaxPerSeason < 1 || c.AniList.MaxPerSeason > 500 {
		errs = append(errs, fmt.Sprintf("anilist.max_per_season %d must be between 1 and 500", c.AniList.MaxPerSeason))
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

func DefaultConfig() *Config {
	return &Config{
		AniList: AniListConfig{
			Years:          nil,
			Seasons:        []string{"all"},
			MaxPerSeason:   DefaultMaxPerSeason,
			IncludeONA:     false,
			WinterOverflow: true,
			AheadMonths:    3,
		},
		OutputDir:            "./sonarr-lists",
		CommunityMappingPath: DefaultMappingPath,
		AnimeListsPath:       DefaultAnimeListsPath,
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

	if v := os.Getenv(envPrefix + "ANILIST_MAX_PER_SEASON"); v != "" {
		if max, err := strconv.Atoi(v); err == nil && max > 0 {
			c.AniList.MaxPerSeason = max
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
			c.AniList.AheadMonths = m
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

	if v := os.Getenv(envPrefix + "COMMUNITY_MAPPING_PATH"); v != "" {
		c.CommunityMappingPath = v
	}

	if v := os.Getenv(envPrefix + "ANIME_LISTS_PATH"); v != "" {
		c.AnimeListsPath = v
	}

	if v := os.Getenv(envPrefix + "TMDB_API_KEY"); v != "" {
		c.TMDBAPIKey = v
	}

	if v := os.Getenv(envPrefix + "LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}

	if v := os.Getenv(envPrefix + "LOG_FILE"); v != "" {
		c.Logging.File = v
	}

	if v := os.Getenv(envPrefix + "SONARR_URL"); v != "" {
		c.Sonarr.URL = v
	}

	if v := os.Getenv(envPrefix + "SONARR_API_KEY"); v != "" {
		c.Sonarr.APIKey = v
	}

	if v := os.Getenv(envPrefix + "SONARR_QUALITY_PROFILE"); v != "" {
		c.Sonarr.QualityProfile = v
	}

	if v := os.Getenv(envPrefix + "SONARR_ROOT_FOLDER"); v != "" {
		c.Sonarr.RootFolder = v
	}
}

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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw map[string]any
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	knownKeys := map[string]bool{
		"anilist":                true,
		"blacklist":              true,
		"output_dir":             true,
		"community_mapping_path": true,
		"anime_lists_path":       true,
		"tmdb_api_key":           true,
		"logging":                true,
		"sonarr":                 true,
	}
	for k := range raw {
		if !knownKeys[k] {
			fmt.Fprintf(os.Stderr, "warning: config file %s contains unknown key %q\n", path, k)
		}
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshal config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &cfg, nil
}

func searchPaths(cliPath string) []string {
	if cliPath != "" {
		return []string{cliPath}
	}
	paths := []string{filepath.Join(".", "anilistgen.yaml")}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			xdg = filepath.Join(home, ".config")
		}
	}
	if xdg != "" {
		paths = append(paths, filepath.Join(xdg, "anilistgen", "anilistgen.yaml"))
	}
	return paths
}

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

  # Maximum results per season. Default: 100
  max_per_season: 100

  # Include ONA format alongside TV. Default: false
  include_ona: false

  # Merge December-premiering shows from the previous year into WINTER.
  # When enabled, fetches the prior year's WINTER and merges only
  # shows that started in December (startDate.month == 12). Default: true
  winter_overflow: true

  # Skip shows starting more than N months ahead. Default: 3
  ahead_months: 3

  # AniList content tags to exclude. Shows with any matching tag
  # (case-insensitive) are skipped entirely.
  # exclude_tags:
  #   - "Hentai"

# Filter
blacklist: []

# Output directory for JSON files
output_dir: ./sonarr-lists

# Mapping file paths (auto-downloaded if missing)
community_mapping_path: /tmp/anilistgen_tvdb.yaml
anime_lists_path: /tmp/anime-list-full.xml

# Logging
logging:
  level: info
  file: ""

# Optional Sonarr API push (instead of GitHub Pages)
sonarr:
  url: ""
  api_key: ""
  quality_profile: "HD-1080p"
  root_folder: "/tv"
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
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "anilistgen", "anilistgen.yaml")
}
