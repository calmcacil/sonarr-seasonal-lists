package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.MDBListAPIKey != "" {
		t.Errorf("expected empty MDBListAPIKey, got %q", cfg.MDBListAPIKey)
	}
	if cfg.Interval.Duration != DefaultInterval {
		t.Errorf("expected Interval %v, got %v", DefaultInterval, cfg.Interval.Duration)
	}
	if !cfg.RunOnStart {
		t.Error("expected RunOnStart to be true")
	}
	if cfg.StateFile != DefaultStateFile {
		t.Errorf("expected StateFile %q, got %q", DefaultStateFile, cfg.StateFile)
	}
	if cfg.AniList.Year != 0 {
		t.Errorf("expected Year 0, got %d", cfg.AniList.Year)
	}
	if cfg.AniList.Years != nil {
		t.Errorf("expected Years to be nil, got %v", cfg.AniList.Years)
	}
	if cfg.AniList.MaxPerSeason != DefaultMaxPerSeason {
		t.Errorf("expected MaxPerSeason %d, got %d", DefaultMaxPerSeason, cfg.AniList.MaxPerSeason)
	}
	if !cfg.AniList.IncludeONA {
		t.Error("expected IncludeONA to be true")
	}
	if cfg.MDBList.TitleTemplate != DefaultTitleTemplate {
		t.Errorf("expected TitleTemplate %q, got %q", DefaultTitleTemplate, cfg.MDBList.TitleTemplate)
	}
	if cfg.MDBList.DescriptionTemplate != DefaultDescriptionTemplate {
		t.Errorf("expected DescriptionTemplate %q, got %q", DefaultDescriptionTemplate, cfg.MDBList.DescriptionTemplate)
	}
	if !cfg.MDBList.Public {
		t.Error("expected Public to be true")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level 'info', got %q", cfg.Logging.Level)
	}
}

func TestFillDefaults(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.FillDefaults()

	if cfg.AniList.MaxPerSeason != DefaultMaxPerSeason {
		t.Errorf("expected MaxPerSeason %d, got %d", DefaultMaxPerSeason, cfg.AniList.MaxPerSeason)
	}
	if len(cfg.AniList.FallbackRelationTypes) != 2 || cfg.AniList.FallbackRelationTypes[0] != "PREQUEL" {
		t.Errorf("expected FallbackRelationTypes [PREQUEL PARENT], got %v", cfg.AniList.FallbackRelationTypes)
	}
	if cfg.MDBList.TitleTemplate != DefaultTitleTemplate {
		t.Errorf("expected TitleTemplate %q, got %q", DefaultTitleTemplate, cfg.MDBList.TitleTemplate)
	}
	if cfg.MDBList.DescriptionTemplate != DefaultDescriptionTemplate {
		t.Errorf("expected DescriptionTemplate %q, got %q", DefaultDescriptionTemplate, cfg.MDBList.DescriptionTemplate)
	}
	if cfg.Interval.Duration != DefaultInterval {
		t.Errorf("expected Interval %v, got %v", DefaultInterval, cfg.Interval.Duration)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level 'info', got %q", cfg.Logging.Level)
	}
}

func TestFillDefaults_PreservesSetValues(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AniList: AniListConfig{
			MaxPerSeason:          50,
			FallbackRelationTypes: []string{"PREQUEL"},
		},
		MDBList: MDBListConfig{
			TitleTemplate: "Custom {season}",
		},
		Interval: Duration{Duration: 2 * time.Hour},
		Logging:  LoggingConfig{Level: "debug"},
	}
	cfg.FillDefaults()

	if cfg.AniList.MaxPerSeason != 50 {
		t.Errorf("expected MaxPerSeason 50, got %d", cfg.AniList.MaxPerSeason)
	}
	if len(cfg.AniList.FallbackRelationTypes) != 1 || cfg.AniList.FallbackRelationTypes[0] != "PREQUEL" {
		t.Errorf("expected FallbackRelationTypes [PREQUEL], got %v", cfg.AniList.FallbackRelationTypes)
	}
	if cfg.Interval.Duration != 2*time.Hour {
		t.Errorf("expected Interval 2h, got %v", cfg.Interval.Duration)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected Logging.Level 'debug', got %q", cfg.Logging.Level)
	}
}

func TestValidate_Valid(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		MDBListAPIKey: "test-key-123",
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerSeason: 100,
			Year:         2026,
		},
		MDBList: MDBListConfig{
			TitleTemplate: "Anime {season} {year}",
		},
		Logging: LoggingConfig{Level: "info"},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingAPIKey(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		MDBListAPIKey: "",
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerSeason: 100,
			Year:         2026,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if contains := containsSubstring(err.Error(), "mdblist_api_key is required"); !contains {
		t.Errorf("expected error about missing api key, got: %v", err)
	}
}

func TestValidate_YearOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		years []int
		year  int
	}{
		{"below minimum", nil, 1999},
		{"above maximum", nil, 2101},
		{"in Years list", []int{2050, 1999}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				MDBListAPIKey: "key",
				AniList: AniListConfig{
					Seasons:      []string{"winter"},
					MaxPerSeason: 100,
					Year:         tc.year,
					Years:        tc.years,
				},
			}
			if err := cfg.Validate(); err == nil {
				t.Error("expected error for out-of-range year")
			}
		})
	}
}

func TestValidate_MaxPerSeasonRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  int
	}{
		{"too low", 0},
		{"too high", 501},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				MDBListAPIKey: "key",
				AniList: AniListConfig{
					Seasons:      []string{"winter"},
					MaxPerSeason: tc.val,
					Year:         2026,
				},
			}
			if err := cfg.Validate(); err == nil {
				t.Errorf("expected error for MaxPerSeason=%d", tc.val)
			}
		})
	}
}

func TestValidate_NoSeasons(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		MDBListAPIKey: "key",
		AniList: AniListConfig{
			Seasons:      []string{"invalid-season"},
			MaxPerSeason: 100,
			Year:         2026,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for no valid seasons")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		MDBListAPIKey: "key",
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerSeason: 100,
			Year:         2026,
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
		MDBListAPIKey: "key",
		AniList: AniListConfig{
			Seasons:      []string{"winter"},
			MaxPerSeason: 100,
			Year:         2026,
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

func TestYearsOrDefault_YearSet(t *testing.T) {
	t.Parallel()

	a := &AniListConfig{Year: 2025}
	got := a.YearsOrDefault()
	if len(got) != 1 || got[0] != 2025 {
		t.Errorf("YearsOrDefault() = %v, want [2025]", got)
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

func TestDurationUnmarshalYAML_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"24h", 24 * time.Hour},
		{"168h", 168 * time.Hour},
		{"0s", 0},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			node := &yaml.Node{Value: tc.input, Kind: yaml.ScalarNode, Tag: "!!str"}
			var d Duration
			if err := d.UnmarshalYAML(node); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Duration != tc.want {
				t.Errorf("got %v, want %v", d.Duration, tc.want)
			}
		})
	}
}

func TestDurationUnmarshalYAML_Invalid(t *testing.T) {
	t.Parallel()

	inputs := []string{"", "abc", "1x"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			node := &yaml.Node{Value: input, Kind: yaml.ScalarNode, Tag: "!!str"}
			var d Duration
			if err := d.UnmarshalYAML(node); err == nil {
				t.Errorf("expected error for %q, got nil", input)
			}
		})
	}
}

func TestDurationUnmarshalYAML_NonString(t *testing.T) {
	t.Parallel()

	node := &yaml.Node{Value: "123", Kind: yaml.ScalarNode, Tag: "!!int"}
	var d Duration
	if err := d.UnmarshalYAML(node); err == nil {
		t.Error("expected error for non-string YAML node")
	}
}

func TestApplyEnvOverrides_MDBListAPIKey(t *testing.T) {
	cfg := &Config{}
	setenv := func(key, val string) {
		os.Setenv(key, val)
		t.Cleanup(func() { os.Unsetenv(key) })
	}
	setenv("ALG_MDBLIST_API_KEY", "alg-key-123")

	cfg.applyEnvOverrides()
	if cfg.MDBListAPIKey != "alg-key-123" {
		t.Errorf("expected 'alg-key-123', got %q", cfg.MDBListAPIKey)
	}
}

func TestApplyEnvOverrides_Interval(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_INTERVAL", "30m")
	t.Cleanup(func() { os.Unsetenv("ALG_INTERVAL") })

	cfg.applyEnvOverrides()
	if cfg.Interval.Duration != 30*time.Minute {
		t.Errorf("expected 30m, got %v", cfg.Interval.Duration)
	}
}

func TestApplyEnvOverrides_AniListYear(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_YEAR", "2027")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_YEAR") })

	cfg.applyEnvOverrides()
	if cfg.AniList.Year != 2027 {
		t.Errorf("expected 2027, got %d", cfg.AniList.Year)
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

func TestApplyEnvOverrides_MaxPerSeason(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_MAX_PER_SEASON", "250")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_MAX_PER_SEASON") })

	cfg.applyEnvOverrides()
	if cfg.AniList.MaxPerSeason != 250 {
		t.Errorf("expected 250, got %d", cfg.AniList.MaxPerSeason)
	}
}

func TestApplyEnvOverrides_IncludeONA(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_INCLUDE_ONA", "false")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_INCLUDE_ONA") })

	cfg.applyEnvOverrides()
	if cfg.AniList.IncludeONA {
		t.Error("expected IncludeONA to be false")
	}
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

func TestApplyEnvOverrides_FallbackRelations(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_ANILIST_FALLBACK_RELATIONS", "PREQUEL,ADAPTATION")
	t.Cleanup(func() { os.Unsetenv("ALG_ANILIST_FALLBACK_RELATIONS") })

	cfg.applyEnvOverrides()
	if len(cfg.AniList.FallbackRelationTypes) != 2 || cfg.AniList.FallbackRelationTypes[1] != "ADAPTATION" {
		t.Errorf("expected [PREQUEL ADAPTATION], got %v", cfg.AniList.FallbackRelationTypes)
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

func TestApplyEnvOverrides_TitleTemplate(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_MDBLIST_TITLE_TEMPLATE", "Custom {season}")
	t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_TITLE_TEMPLATE") })

	cfg.applyEnvOverrides()
	if cfg.MDBList.TitleTemplate != "Custom {season}" {
		t.Errorf("expected 'Custom {season}', got %q", cfg.MDBList.TitleTemplate)
	}
}

func TestApplyEnvOverrides_DescriptionTemplate(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_MDBLIST_DESCRIPTION_TEMPLATE", "Desc {season}")
	t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_DESCRIPTION_TEMPLATE") })

	cfg.applyEnvOverrides()
	if cfg.MDBList.DescriptionTemplate != "Desc {season}" {
		t.Errorf("expected 'Desc {season}', got %q", cfg.MDBList.DescriptionTemplate)
	}
}

func TestApplyEnvOverrides_Public(t *testing.T) {
	t.Run("set to false", func(t *testing.T) {
		cfg := &Config{}
		os.Setenv("ALG_MDBLIST_PUBLIC", "false")
		t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_PUBLIC") })

		cfg.applyEnvOverrides()
		if cfg.MDBList.Public {
			t.Error("expected Public to be false")
		}
	})

	t.Run("set to 0", func(t *testing.T) {
		cfg := &Config{}
		os.Setenv("ALG_MDBLIST_PUBLIC", "0")
		t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_PUBLIC") })

		cfg.applyEnvOverrides()
		if cfg.MDBList.Public {
			t.Error("expected Public to be false when '0'")
		}
	})
}

func TestApplyEnvOverrides_Blacklist(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_MDBLIST_BLACKLIST", "One Piece, 16498, Naruto")
	t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_BLACKLIST") })

	cfg.applyEnvOverrides()
	if len(cfg.MDBList.Blacklist) != 3 {
		t.Fatalf("expected 3 blacklist entries, got %d: %v", len(cfg.MDBList.Blacklist), cfg.MDBList.Blacklist)
	}
	if cfg.MDBList.Blacklist[0] != "One Piece" {
		t.Errorf("expected 'One Piece', got %q", cfg.MDBList.Blacklist[0])
	}
	if cfg.MDBList.Blacklist[1] != "16498" {
		t.Errorf("expected '16498', got %q", cfg.MDBList.Blacklist[1])
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

func TestApplyEnvOverrides_RunOnStart(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_RUN_ON_START", "false")
	t.Cleanup(func() { os.Unsetenv("ALG_RUN_ON_START") })

	cfg.applyEnvOverrides()
	if cfg.RunOnStart {
		t.Error("expected RunOnStart to be false")
	}
}

func TestApplyEnvOverrides_StateFile(t *testing.T) {
	cfg := &Config{}
	os.Setenv("ALG_STATE_FILE", "/custom/path.state")
	t.Cleanup(func() { os.Unsetenv("ALG_STATE_FILE") })

	cfg.applyEnvOverrides()
	if cfg.StateFile != "/custom/path.state" {
		t.Errorf("expected '/custom/path.state', got %q", cfg.StateFile)
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
	content := []byte("mdblist_api_key: \"test-key\"\nanilist:\n  year: 2026\n  seasons:\n    - spring\ninterval: \"24h\"\n")
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
	if cfg.MDBListAPIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", cfg.MDBListAPIKey)
	}
	if cfg.AniList.Year != 2026 {
		t.Errorf("expected Year 2026, got %d", cfg.AniList.Year)
	}
	if cfg.AniList.MaxPerSeason != DefaultMaxPerSeason {
		t.Errorf("expected MaxPerSeason %d, got %d", DefaultMaxPerSeason, cfg.AniList.MaxPerSeason)
	}
	if cfg.Interval.Duration != 24*time.Hour {
		t.Errorf("expected Interval 24h, got %v", cfg.Interval.Duration)
	}
}

func TestLoad_ValidYAML_SeasonAll(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "anilistgen.yaml")
	content := []byte("mdblist_api_key: \"key\"\nanilist:\n  seasons:\n    - all\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	seasons := cfg.AniList.Season()
	if len(seasons) != 4 {
		t.Errorf("expected 4 seasons, got %d: %v", len(seasons), seasons)
	}
}

func TestLoad_MissingFileFallback(t *testing.T) {
	cfg, path, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for fallback, got %q", path)
	}
	if cfg.MDBListAPIKey != "" {
		t.Errorf("expected empty API key in fallback config")
	}
	if cfg.AniList.MaxPerSeason != DefaultMaxPerSeason {
		t.Errorf("expected MaxPerSeason %d in fallback config", DefaultMaxPerSeason)
	}
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	os.Setenv("ALG_MDBLIST_API_KEY", "env-key-999")
	t.Cleanup(func() { os.Unsetenv("ALG_MDBLIST_API_KEY") })

	cfg, _, err := Load("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MDBListAPIKey != "env-key-999" {
		t.Errorf("expected 'env-key-999', got %q", cfg.MDBListAPIKey)
	}
}

func TestLoad_FileWithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "anilistgen.yaml")
	content := []byte("mdblist_api_key: \"file-key\"\ninterval: \"12h\"\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ALG_INTERVAL", "6h")
	t.Cleanup(func() { os.Unsetenv("ALG_INTERVAL") })

	cfg, _, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.MDBListAPIKey != "file-key" {
		t.Errorf("expected 'file-key' (from file), got %q", cfg.MDBListAPIKey)
	}
	if cfg.Interval.Duration != 6*time.Hour {
		t.Errorf("expected 6h (env override), got %v", cfg.Interval.Duration)
	}
}

func TestResolveConfigPath_WithCLI(t *testing.T) {
	got := ResolveConfigPath("/custom/path.yaml")
	if got != "/custom/path.yaml" {
		t.Errorf("expected '/custom/path.yaml', got %q", got)
	}
}

func TestResolveConfigPath_DefaultToXDG(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/xdg/home")
	t.Cleanup(func() { os.Unsetenv("XDG_CONFIG_HOME") })

	got := ResolveConfigPath("")
	want := filepath.Join("/xdg/home", "anilistgen", "anilistgen.yaml")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "anilistgen.yaml")
	if err := os.WriteFile(existingPath, []byte("key: val"), 0644); err != nil {
		t.Fatal(err)
	}

	// Temporarily change workdir so searchPaths finds our file
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origWd) })

	got := ResolveConfigPath("")
	want := "anilistgen.yaml"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// containsSubstring is a small helper.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
