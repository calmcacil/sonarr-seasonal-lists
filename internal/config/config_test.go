package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/calmcacil/anilistgen/internal/mapping"
)

func TestFillDefaults(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.FillDefaults()

	if cfg.AniList.MaxPerYear != DefaultMaxPerYear {
		t.Errorf("expected MaxPerYear %d, got %d", DefaultMaxPerYear, cfg.AniList.MaxPerYear)
	}
	if cfg.AniList.AheadMonthsOrDefault() != 3 {
		t.Errorf("expected AheadMonths 3, got %d", cfg.AniList.AheadMonthsOrDefault())
	}
	if cfg.OutputDir != "./sonarr-lists" {
		t.Errorf("expected OutputDir './sonarr-lists', got %q", cfg.OutputDir)
	}
	if cfg.AnibridgeMappingPath != mapping.DefaultAnibridgePath() {
		t.Errorf("expected AnibridgeMappingPath %q, got %q", mapping.DefaultAnibridgePath(), cfg.AnibridgeMappingPath)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level 'info', got %q", cfg.Logging.Level)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("expected BaseURL %q, got %q", DefaultBaseURL, cfg.BaseURL)
	}
}

func TestFillDefaults_PreservesSetValues(t *testing.T) {
	t.Parallel()

	v := 6
	cfg := &Config{
		AniList: AniListConfig{
			MaxPerYear: 50,
			AheadMonths:   &v,
		},
		OutputDir:  "/custom/output",
		Logging:    LoggingConfig{Level: "debug"},
	}
	cfg.FillDefaults()

	if cfg.AniList.MaxPerYear != 50 {
		t.Errorf("expected MaxPerYear 50, got %d", cfg.AniList.MaxPerYear)
	}
	if cfg.AniList.AheadMonthsOrDefault() != 6 {
		t.Errorf("expected AheadMonths 6, got %d", cfg.AniList.AheadMonthsOrDefault())
	}
	if cfg.OutputDir != "/custom/output" {
		t.Errorf("expected OutputDir '/custom/output', got %q", cfg.OutputDir)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected Logging.Level 'debug', got %q", cfg.Logging.Level)
	}
}

func TestValidate_Valid(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerYear: 100,
		},
		Logging: LoggingConfig{Level: "info"},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_YearOutOfRange(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerYear: 100,
			Years:        []int{1999},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for out-of-range year")
	}
}

func TestValidate_MaxPerYearRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  int
	}{
		{"too low", 0},
		{"too high", 2001},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				AniList: AniListConfig{
					Seasons:    []string{"winter"},
					MaxPerYear: tc.val,
				},
			}
			if err := cfg.Validate(); err == nil {
				t.Errorf("expected error for MaxPerYear=%d", tc.val)
			}
		})
	}
}

func TestValidate_NoSeasons(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			Seasons:    []string{"invalid-season"},
			MaxPerYear: 100,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for no valid seasons")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerYear: 100,
		},
		Logging: LoggingConfig{Level: "trace"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestValidate_EmptyLogLevelIsValid(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerYear: 100,
		},
		Logging: LoggingConfig{Level: ""},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for empty log level, got: %v", err)
	}
}

func TestSeason_All(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{Seasons: []string{"all"}}
	got := a.Season()

	want := []string{"WINTER", "SPRING", "SUMMER", "FALL"}
	if len(got) != len(want) {
		t.Fatalf("expected %d seasons, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("season[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSeason_AllCaseInsensitive(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{Seasons: []string{"ALL"}}
	got := a.Season()
	if len(got) != 4 {
		t.Errorf("expected 4 seasons for ALL, got %d", len(got))
	}
}

func TestSeason_Specific(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"winter", "WINTER"},
		{"WINTER", "WINTER"},
		{"Spring", "SPRING"},
		{"summer", "SUMMER"},
		{"FALL", "FALL"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			a := &AniListConfig{Seasons: []string{tc.input}}
			got := a.Season()
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("Season(%q) = %v, want [%q]", tc.input, got, tc.want)
			}
		})
	}
}

func TestSeason_Empty(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{Seasons: nil}
	got := a.Season()
	if len(got) != 4 {
		t.Errorf("expected 4 seasons for empty, got %d: %v", len(got), got)
	}

	a2 := &AniListConfig{Seasons: []string{}}
	got2 := a2.Season()
	if len(got2) != 4 {
		t.Errorf("expected 4 seasons for empty slice, got %d: %v", len(got2), got2)
	}
}

func TestYearsOrDefault_YearsSet(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{Years: []int{2024, 2025, 2026}}
	got := a.YearsOrDefault()
	if len(got) != 3 || got[0] != 2024 || got[1] != 2025 || got[2] != 2026 {
		t.Errorf("YearsOrDefault() = %v, want [2024 2025 2026]", got)
	}
}

func TestYearsOrDefault_DefaultsToCurrent(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{}
	got := a.YearsOrDefault()
	want := []int{time.Now().Year()}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("YearsOrDefault() = %v, want %v", got, want)
	}
}

func TestApplyEnvOverrides_AniListYears(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_YEARS", "2025,2026,2027")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_YEARS") })

	cfg.applyEnvOverrides()
	if len(cfg.AniList.Years) != 3 || cfg.AniList.Years[0] != 2025 || cfg.AniList.Years[2] != 2027 {
		t.Errorf("expected [2025 2026 2027], got %v", cfg.AniList.Years)
	}
}

func TestApplyEnvOverrides_Seasons(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_SEASONS", "spring,fall")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_SEASONS") })

	cfg.applyEnvOverrides()
	if len(cfg.AniList.Seasons) != 2 || cfg.AniList.Seasons[0] != "spring" || cfg.AniList.Seasons[1] != "fall" {
		t.Errorf("expected [spring fall], got %v", cfg.AniList.Seasons)
	}
}

func TestApplyEnvOverrides_MaxPerYear(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_MAX_PER_YEAR", "500")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_MAX_PER_YEAR") })

	cfg.applyEnvOverrides()
	if cfg.AniList.MaxPerYear != 500 {
		t.Errorf("expected 500, got %d", cfg.AniList.MaxPerYear)
	}
}

func TestApplyEnvOverrides_IncludeONA(t *testing.T) {
	t.Run("overrides to true", func(t *testing.T) {
		cfg := &Config{AniList: AniListConfig{IncludeONA: false}}
		os.Setenv("ALG_ANILIST_INCLUDE_ONA", "true")
		t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_INCLUDE_ONA") })
		cfg.applyEnvOverrides()
		if !cfg.AniList.IncludeONA {
			t.Error("expected IncludeONA to be true")
		}
	})
	t.Run("overrides to false", func(t *testing.T) {
		cfg := &Config{AniList: AniListConfig{IncludeONA: true}}
		os.Setenv("ALG_ANILIST_INCLUDE_ONA", "false")
		t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_INCLUDE_ONA") })
		cfg.applyEnvOverrides()
		if cfg.AniList.IncludeONA {
			t.Error("expected IncludeONA to be false")
		}
	})
}

func TestApplyEnvOverrides_WinterOverflow(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_WINTER_OVERFLOW", "1")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_WINTER_OVERFLOW") })

	cfg.applyEnvOverrides()
	if !cfg.AniList.WinterOverflow {
		t.Error("expected WinterOverflow to be true")
	}
}

func TestApplyEnvOverrides_ExcludeTags(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_EXCLUDE_TAGS", "hentai,guro")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_EXCLUDE_TAGS") })

	cfg.applyEnvOverrides()
	if len(cfg.AniList.ExcludeTags) != 2 || cfg.AniList.ExcludeTags[0] != "hentai" {
		t.Errorf("expected [hentai guro], got %v", cfg.AniList.ExcludeTags)
	}
}

func TestApplyEnvOverrides_LogLevel(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_LOG_LEVEL", "debug")
	t.Cleanup(func() { os.Unsetenv("ALG_LOG_LEVEL") })

	cfg.applyEnvOverrides()
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected 'debug', got %q", cfg.Logging.Level)
	}
}

func TestApplyEnvOverrides_LogFile(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_LOG_FILE", "/tmp/test.log")
	t.Cleanup(func() { os.Unsetenv("ALG_LOG_FILE") })

	cfg.applyEnvOverrides()
	if cfg.Logging.File != "/tmp/test.log" {
		t.Errorf("expected '/tmp/test.log', got %q", cfg.Logging.File)
	}
}

func TestSearchPaths_WithCLI(t *testing.T) {
	paths := searchPaths("/custom/config.yaml")
	if len(paths) != 1 || paths[0] != "/custom/config.yaml" {
		t.Errorf("expected [/custom/config.yaml], got %v", paths)
	}
}

func TestSearchPaths_WithoutCLI(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	paths := searchPaths("")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != "anilistgen.yaml" {
		t.Errorf("paths[0] = %q, want %q", paths[0], "anilistgen.yaml")
	}
	wantXDG := filepath.Join(home, ".config", "anilistgen", "anilistgen.yaml")
	if paths[1] != wantXDG {
		t.Errorf("paths[1] = %q, want %q", paths[1], wantXDG)
	}
}

func TestSearchPaths_WithXDG(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })

	paths := searchPaths("")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
	want := filepath.Join("/custom/xdg", "anilistgen", "anilistgen.yaml")
	if paths[1] != want {
		t.Errorf("paths[1] = %q, want %q", paths[1], want)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "anilistgen.yaml")
	content := []byte("anilist:\n  years:\n    - 2026\n  seasons:\n    - spring\noutput_dir: /tmp/out\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if path != cfgPath {
		t.Errorf("expected path %q, got %q", cfgPath, path)
	}
	if len(cfg.AniList.Years) != 1 || cfg.AniList.Years[0] != 2026 {
		t.Errorf("expected Years [2026], got %v", cfg.AniList.Years)
	}
	if cfg.OutputDir != "/tmp/out" {
		t.Errorf("expected OutputDir '/tmp/out', got %q", cfg.OutputDir)
	}
}

func TestLoad_MissingFileFallback(t *testing.T) {
	_, path, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for fallback, got %q", path)
	}
}

func TestResolveConfigPath_WithCLI(t *testing.T) {
	got := ResolveConfigPath("/custom/path.yaml")
	if got != "/custom/path.yaml" {
		t.Errorf("expected '/custom/path.yaml', got %q", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.AniList.MaxPerYear != DefaultMaxPerYear {
		t.Errorf("expected MaxPerYear %d, got %d", DefaultMaxPerYear, cfg.AniList.MaxPerYear)
	}
	if cfg.AniList.IncludeONA {
		t.Error("expected IncludeONA to be false")
	}
	if cfg.OutputDir != "./sonarr-lists" {
		t.Errorf("expected OutputDir %q, got %q", "./sonarr-lists", cfg.OutputDir)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level 'info', got %q", cfg.Logging.Level)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("expected BaseURL %q, got %q", DefaultBaseURL, cfg.BaseURL)
	}
}
