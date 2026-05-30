package sync

import (
	"testing"
)

func TestResolvedListTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		season   string
		year     int
		want     string
	}{
		{"default template", "Anime {season} {year}", "WINTER", 2026, "Anime Winter 2026"},
		{"custom template", "My {season} List {year}", "SPRING", 2025, "My Spring List 2025"},
		{"no placeholders", "Static Title", "SUMMER", 2024, "Static Title"},
		{"season only", "{season} Season", "FALL", 2024, "Fall Season"},
		{"year only", "Year {year}", "WINTER", 2026, "Year 2026"},
		{"multiple placeholders", "{season} {year} - {season}", "WINTER", 2026, "Winter 2026 - Winter"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolvedListTitle(tc.template, tc.season, tc.year)
			if got != tc.want {
				t.Errorf("ResolvedListTitle(%q, %q, %d) = %q, want %q",
					tc.template, tc.season, tc.year, got, tc.want)
			}
		})
	}
}

func TestResolvedListDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		season   string
		year     int
		want     string
	}{
		{"default template", "All anime in {season} {year}.", "WINTER", 2026, "All anime in Winter 2026."},
		{"custom template", "Desc: {season} {year}", "SUMMER", 2025, "Desc: Summer 2025"},
		{"no placeholders", "Static description", "FALL", 2024, "Static description"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolvedListDescription(tc.template, tc.season, tc.year)
			if got != tc.want {
				t.Errorf("ResolvedListDescription(%q, %q, %d) = %q, want %q",
					tc.template, tc.season, tc.year, got, tc.want)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"WINTER", "Winter"},
		{"winter", "Winter"},
		{"WINTER", "Winter"},
		{"spring", "Spring"},
		{"SUMMER", "Summer"},
		{"fall", "Fall"},
		{"", ""},
		{"a", "A"},
		{"ABC DEF", "Abc def"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := capitalize(tc.input)
			if got != tc.want {
				t.Errorf("capitalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsBlacklisted_MALIDMatch(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: []string{"16498", "30230"}}

	if !cfg.isBlacklisted("Attack on Titan", 16498) {
		t.Error("expected true for MAL ID 16498")
	}
	if !cfg.isBlacklisted("Random Show", 30230) {
		t.Error("expected true for MAL ID 30230")
	}
	if cfg.isBlacklisted("Attack on Titan", 99999) {
		t.Error("expected false for non-blacklisted MAL ID")
	}
}

func TestIsBlacklisted_TitleSubstringMatch(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: []string{"One Piece", "Bleach"}}

	if !cfg.isBlacklisted("One Piece", 0) {
		t.Error("expected true for exact title match")
	}
	if !cfg.isBlacklisted("One Piece: Episode of Nami", 0) {
		t.Error("expected true for substring title match")
	}
	if cfg.isBlacklisted("Naruto", 0) {
		t.Error("expected false for non-blacklisted title")
	}
}

func TestIsBlacklisted_TitleCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: []string{"one piece"}}

	if !cfg.isBlacklisted("ONE PIECE", 0) {
		t.Error("expected true for case-insensitive title match")
	}
	if !cfg.isBlacklisted("One Piece: Episode of Nami", 0) {
		t.Error("expected true for substring case-insensitive match")
	}
}

func TestIsBlacklisted_NoMatch(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: []string{"One Piece"}}
	if cfg.isBlacklisted("Naruto", 16498) {
		t.Error("expected false for non-blacklisted show")
	}
}

func TestIsBlacklisted_EmptyBlacklist(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: nil}
	if cfg.isBlacklisted("Naruto", 16498) {
		t.Error("expected false for nil blacklist")
	}

	cfg2 := SyncConfig{Blacklist: []string{}}
	if cfg2.isBlacklisted("Naruto", 16498) {
		t.Error("expected false for empty blacklist")
	}
}

func TestIsBlacklisted_EmptyEntry(t *testing.T) {
	t.Parallel()

	cfg := SyncConfig{Blacklist: []string{"", "One Piece"}}
	if !cfg.isBlacklisted("One Piece", 0) {
		t.Error("expected true despite empty entries")
	}
}

func TestProviderIDStrings_SingleKey(t *testing.T) {
	t.Parallel()

	items := []mdbItem{
		{id: map[string]any{"imdb": "tt0903747"}},
	}
	got := providerIDStrings(items)
	if len(got) != 1 || got[0] != "imdb:tt0903747" {
		t.Errorf("expected [\"imdb:tt0903747\"], got %v", got)
	}
}

func TestProviderIDStrings_MultipleKeys(t *testing.T) {
	t.Parallel()

	items := []mdbItem{
		{id: map[string]any{"imdb": "tt0903747"}},
		{id: map[string]any{"tmdb": 1396}},
		{id: map[string]any{"tvdb": 12345}},
	}
	got := providerIDStrings(items)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(got), got)
	}
	if got[0] != "imdb:tt0903747" {
		t.Errorf("got[0] = %q, want %q", got[0], "imdb:tt0903747")
	}
	if got[1] != "tmdb:1396" {
		t.Errorf("got[1] = %q, want %q", got[1], "tmdb:1396")
	}
	if got[2] != "tvdb:12345" {
		t.Errorf("got[2] = %q, want %q", got[2], "tvdb:12345")
	}
}

func TestProviderIDStrings_EmptyMap(t *testing.T) {
	t.Parallel()

	items := []mdbItem{
		{id: map[string]any{}},
	}
	got := providerIDStrings(items)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("expected [\"\"], got %v", got)
	}
}

func TestProviderIDStrings_Float64Values(t *testing.T) {
	t.Parallel()

	items := []mdbItem{
		{id: map[string]any{"tmdb": float64(1396)}},
	}
	got := providerIDStrings(items)
	if len(got) != 1 || got[0] != "tmdb:1396" {
		t.Errorf("expected [\"tmdb:1396\"], got %v", got)
	}
}

func TestParseProviderID_IMDB(t *testing.T) {
	t.Parallel()

	result := parseProviderID("imdb:tt0903747")
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	if result["imdb"] != "tt0903747" {
		t.Errorf("expected 'tt0903747', got %v", result["imdb"])
	}
}

func TestParseProviderID_TMDB(t *testing.T) {
	t.Parallel()

	result := parseProviderID("tmdb:1396")
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	val, ok := result["tmdb"]
	if !ok {
		t.Fatal("expected key 'tmdb'")
	}
	intVal, ok := val.(int)
	if !ok {
		t.Fatalf("expected int, got %T", val)
	}
	if intVal != 1396 {
		t.Errorf("expected 1396, got %v", intVal)
	}
}

func TestParseProviderID_TVDB(t *testing.T) {
	t.Parallel()

	result := parseProviderID("tvdb:12345")
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	val, ok := result["tvdb"]
	if !ok {
		t.Fatal("expected key 'tvdb'")
	}
	intVal, ok := val.(int)
	if !ok {
		t.Fatalf("expected int, got %T", val)
	}
	if intVal != 12345 {
		t.Errorf("expected 12345, got %v", intVal)
	}
}

func TestParseProviderID_EmptyString(t *testing.T) {
	t.Parallel()

	result := parseProviderID("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseProviderID_NoDelimiter(t *testing.T) {
	t.Parallel()

	result := parseProviderID("tt0903747")
	if len(result) != 0 {
		t.Errorf("expected empty map for no delimiter, got %v", result)
	}
}

func TestParseProviderID_UnknownKey(t *testing.T) {
	t.Parallel()

	result := parseProviderID("mal:16498")
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	if result["mal"] != "16498" {
		t.Errorf("expected '16498', got %v", result["mal"])
	}
}

func TestResultStruct_ZeroValues(t *testing.T) {
	t.Parallel()

	var r Result
	if r.Season != "" {
		t.Errorf("expected empty Season, got %q", r.Season)
	}
	if r.Year != 0 {
		t.Errorf("expected 0 Year, got %d", r.Year)
	}
	if r.ListTitle != "" {
		t.Errorf("expected empty ListTitle, got %q", r.ListTitle)
	}
	if r.ListURL != "" {
		t.Errorf("expected empty ListURL, got %q", r.ListURL)
	}
	if r.ShowCount != 0 {
		t.Errorf("expected 0 ShowCount, got %d", r.ShowCount)
	}
	if r.TotalInDB != 0 {
		t.Errorf("expected 0 TotalInDB, got %d", r.TotalInDB)
	}
	if r.FoundViaFallback != 0 {
		t.Errorf("expected 0 FoundViaFallback, got %d", r.FoundViaFallback)
	}
	if r.NotFoundInDB != 0 {
		t.Errorf("expected 0 NotFoundInDB, got %d", r.NotFoundInDB)
	}
	if r.SkippedDuration != 0 {
		t.Errorf("expected 0 SkippedDuration, got %d", r.SkippedDuration)
	}
	if r.SkippedExcluded != 0 {
		t.Errorf("expected 0 SkippedExcluded, got %d", r.SkippedExcluded)
	}
	if r.Created {
		t.Error("expected Created to be false")
	}
	if r.Updated {
		t.Error("expected Updated to be false")
	}
	if r.Error != nil {
		t.Errorf("expected nil Error, got %v", r.Error)
	}
}
