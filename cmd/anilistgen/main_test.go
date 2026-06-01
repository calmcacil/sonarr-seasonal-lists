package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/model"
)

func TestResolveBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tvdb-mal.yaml")
	content := `AnimeMap:
  - malid: 16498
    tvdbid: 12345
  - malid: 99999
    tvdbid: 67890
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cm, err := mapping.LoadCommunityMapping(path)
	if err != nil {
		t.Fatalf("LoadCommunityMapping: %v", err)
	}
	resolver := mapping.NewResolver(cm)

	winter2026 := model.SeasonKey{Season: "WINTER", Year: 2026}

	result := resolveBatch(resolver, map[model.SeasonKey][]model.Show{}, true)
	if len(result) != 0 {
		t.Errorf("expected empty result for dry-run, got %d entries", len(result))
	}

	result = resolveBatch(resolver, map[model.SeasonKey][]model.Show{}, false)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d entries", len(result))
	}

	result = resolveBatch(resolver, map[model.SeasonKey][]model.Show{
		winter2026: nil,
	}, false)
	if shows, ok := result[winter2026]; !ok {
		t.Error("expected WINTER-2026 key in result")
	} else if len(shows) != 0 {
		t.Errorf("expected 0 shows for nil input, got %d", len(shows))
	}

	result = resolveBatch(resolver, map[model.SeasonKey][]model.Show{
		winter2026: {{ID: 1, IDMal: nil}},
	}, false)
	if shows, ok := result[winter2026]; ok && len(shows) != 0 {
		t.Errorf("expected 0 resolved shows for no IDMal, got %d", len(shows))
	}

	result = resolveBatch(resolver, map[model.SeasonKey][]model.Show{
		winter2026: {{ID: 1, IDMal: makePtr(16498)}},
	}, false)
	if shows, ok := result[winter2026]; !ok {
		t.Error("expected WINTER-2026 key")
	} else if len(shows) != 1 {
		t.Errorf("expected 1 resolved show, got %d", len(shows))
	} else if shows[0].TVDBID != 12345 {
		t.Errorf("expected TVDB 12345, got %d", shows[0].TVDBID)
	}

	t.Run("dry-run output format", func(t *testing.T) {
		shows := []model.Show{
			{ID: 1, IDMal: makePtr(16498), Title: model.Title{English: makePtr("Test Show")}},
			{ID: 2, IDMal: nil},
		}
		result := resolveBatch(resolver, map[model.SeasonKey][]model.Show{
			winter2026: shows,
		}, true)
		if len(result) != 0 {
			t.Error("expected empty result for dry run output")
		}
	})
}

func makePtr[T any](v T) *T {
	return &v
}
