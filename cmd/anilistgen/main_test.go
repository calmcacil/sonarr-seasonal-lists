package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/model"
	"github.com/calmcacil/anilistgen/internal/output"
	"github.com/calmcacil/anilistgen/internal/pipeline"
)

func TestProcessBatch(t *testing.T) {
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

	result := pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{}, true)
	if len(result) != 0 {
		t.Errorf("expected empty result for dry-run, got %d entries", len(result))
	}

	result = pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{}, false)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d entries", len(result))
	}

	result = pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{
		winter2026: nil,
	}, false)
	if shows, ok := result[winter2026]; !ok {
		t.Error("expected WINTER-2026 key in result")
	} else if len(shows) != 0 {
		t.Errorf("expected 0 shows for nil input, got %d", len(shows))
	}

	result = pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{
		winter2026: {{ID: 1, IDMal: nil}},
	}, false)
	if shows, ok := result[winter2026]; ok && len(shows) != 0 {
		t.Errorf("expected 0 resolved shows for no IDMal, got %d", len(shows))
	}

	result = pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{
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
		result := pipeline.ProcessBatch(resolver, map[model.SeasonKey][]model.Show{
			winter2026: shows,
		}, true)
		if len(result) != 0 {
			t.Error("expected empty result for dry run output")
		}
	})
}

func TestPrintDryRun(t *testing.T) {
	winter2026 := model.SeasonKey{Season: "WINTER", Year: 2026}
	data := map[model.SeasonKey][]output.Show{
		winter2026: {{TVDBID: 12345, Title: "Test Show"}},
	}
	printDryRun(data, "series")
}

func TestGroupBySeason(t *testing.T) {
	t.Parallel()

	winter := model.Show{ID: 1, Season: makePtr("WINTER")}
	spring := model.Show{ID: 2, Season: makePtr("SPRING")}
	summer := model.Show{ID: 3, Season: makePtr("SUMMER")}
	fall := model.Show{ID: 4, Season: makePtr("FALL")}
	unknown := model.Show{ID: 5, Season: nil}
	lower := model.Show{ID: 6, Season: makePtr("winter")}

	result := groupBySeason([]model.Show{winter, spring, summer, fall, unknown, lower})

	if len(result["WINTER"]) != 2 {
		t.Errorf("expected 2 WINTER shows, got %d", len(result["WINTER"]))
	}
	if len(result["SPRING"]) != 1 {
		t.Errorf("expected 1 SPRING show, got %d", len(result["SPRING"]))
	}
	if len(result["SUMMER"]) != 1 {
		t.Errorf("expected 1 SUMMER show, got %d", len(result["SUMMER"]))
	}
	if len(result["FALL"]) != 1 {
		t.Errorf("expected 1 FALL show, got %d", len(result["FALL"]))
	}
	if len(result["UNKNOWN"]) != 1 {
		t.Errorf("expected 1 UNKNOWN show, got %d", len(result["UNKNOWN"]))
	}

	found := false
	for _, s := range result["WINTER"] {
		if s.ID == 6 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected lowercase winter show in WINTER bucket")
	}
}

func TestFilterDecember(t *testing.T) {
	t.Parallel()

	dec := model.Show{ID: 1, StartDate: model.FuzzyDate{Month: makePtr(12)}}
	jan := model.Show{ID: 2, StartDate: model.FuzzyDate{Month: makePtr(1)}}
	nilMonth := model.Show{ID: 3, StartDate: model.FuzzyDate{Month: nil}}

	all := []model.Show{{ID: 10}}
	added := filterDecember(&all, []model.Show{dec, jan, nilMonth})

	if added != 1 {
		t.Errorf("expected 1 added (December), got %d", added)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 shows total, got %d", len(all))
	}
	if all[1].ID != 1 {
		t.Errorf("expected added show to have ID 1, got %d", all[1].ID)
	}
}

func TestFilterDecember_Deduplicates(t *testing.T) {
	t.Parallel()

	dec := model.Show{ID: 1, StartDate: model.FuzzyDate{Month: makePtr(12)}}
	all := []model.Show{dec}
	added := filterDecember(&all, []model.Show{dec})

	if added != 0 {
		t.Errorf("expected 0 added (already present), got %d", added)
	}
}

func makePtr[T any](v T) *T {
	return &v
}
