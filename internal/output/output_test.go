package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/calmcacil/anilistgen/internal/model"
)

func TestWriteSeasonJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	shows := []Show{
		{TVDBID: 12345, Title: "Test Show"},
		{TVDBID: 67890, Title: "Another Show"},
	}

	err := WriteSeasonJSON(dir, "series", "WINTER", 2026, shows)
	if err != nil {
		t.Fatalf("WriteSeasonJSON: %v", err)
	}

	path := filepath.Join(dir, "2026", "winter-series.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var got []Show
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 shows, got %d", len(got))
	}
	if got[0].TVDBID != 12345 {
		t.Errorf("expected TVDB 12345, got %d", got[0].TVDBID)
	}
	if got[0].Title != "Test Show" {
		t.Errorf("expected title 'Test Show', got %q", got[0].Title)
	}
}

func TestWriteSeasonJSON_Compact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	shows := []Show{{TVDBID: 1, Title: "T"}}

	err := WriteSeasonJSON(dir, "series", "spring", 2026, shows)
	if err != nil {
		t.Fatalf("WriteSeasonJSON: %v", err)
	}

	path := filepath.Join(dir, "2026", "spring-series.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if strings.Contains(string(data), "\n") {
		t.Error("expected compact JSON (no newlines)")
	}
}

func TestWriteSeasonJSON_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := WriteSeasonJSON(dir, "series", "WINTER", 2026, nil)
	if err != nil {
		t.Fatalf("WriteSeasonJSON: %v", err)
	}

	path := filepath.Join(dir, "2026", "winter-series.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(data) != "[]" {
		t.Errorf("expected empty array '[]', got %q", string(data))
	}
}

func TestWriteYearJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	shows := []Show{
		{TVDBID: 1, Title: "A"},
		{TVDBID: 2, Title: "B"},
	}

	err := WriteYearJSON(dir, "series", 2026, shows)
	if err != nil {
		t.Fatalf("WriteYearJSON: %v", err)
	}

	path := filepath.Join(dir, "2026", "series.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var got []Show
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 shows, got %d", len(got))
	}
}

func TestWriteAllJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seasonal := map[model.SeasonKey][]Show{
		{Season: "WINTER", Year: 2026}: {{TVDBID: 1, Title: "Winter Show"}},
		{Season: "SPRING", Year: 2026}: {{TVDBID: 2, Title: "Spring Show"}},
		{Season: "WINTER", Year: 2025}: {{TVDBID: 3, Title: "Old Show"}},
	}

	err := WriteAllJSON(dir, "https://example.com", "series", seasonal, nil)
	if err != nil {
		t.Fatalf("WriteAllJSON: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	dirs := map[string]bool{}
	var hasIndex bool
	for _, e := range entries {
		dirs[e.Name()] = e.IsDir()
		if e.Name() == "index.html" {
			hasIndex = true
		}
	}
	if !hasIndex {
		t.Error("missing index.html")
	}
	if !dirs["2025"] {
		t.Error("missing 2025 dir")
	}
	if !dirs["2026"] {
		t.Error("missing 2026 dir")
	}
}

func TestWriteSeasonJSON_StartsAsArray(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	shows := []Show{{TVDBID: 1, Title: "T"}}

	err := WriteSeasonJSON(dir, "movies", "fall", 2026, shows)
	if err != nil {
		t.Fatalf("WriteSeasonJSON: %v", err)
	}

	path := filepath.Join(dir, "2026", "fall-movies.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if data[0] != '[' {
		t.Errorf("expected JSON array starting with '[', got %c", data[0])
	}
}

func TestWriteAllJSON_MultipleCategories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	winter2026 := model.SeasonKey{Season: "WINTER", Year: 2026}
	series := map[model.SeasonKey][]Show{
		winter2026: {{TVDBID: 1, Title: "Series A"}},
	}
	movies := map[model.SeasonKey][]Show{
		winter2026: {{TVDBID: 2, Title: "Movie A"}},
	}

	if err := WriteAllJSON(dir, "https://example.com", "series", series, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteAllJSON(dir, "https://example.com", "movies", movies, nil); err != nil {
		t.Fatal(err)
	}

	check := func(path string) {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	check("2026/winter-series.json")
	check("2026/winter-movies.json")
	check("2026/series.json")
	check("2026/movies.json")
}
