package mapping

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/calmcacil/anilistgen/internal/model"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{
		data: map[int]int{
			16498: 12345,
			99999: 67890,
		},
	}

	t.Run("known MAL ID", func(t *testing.T) {
		tvdbID, ok := cm.Lookup(16498)
		if !ok {
			t.Error("expected ok for known MAL ID")
		}
		if tvdbID != 12345 {
			t.Errorf("expected TVDB 12345, got %d", tvdbID)
		}
	})

	t.Run("unknown MAL ID", func(t *testing.T) {
		_, ok := cm.Lookup(1)
		if ok {
			t.Error("expected !ok for unknown MAL ID")
		}
	})

	t.Run("zero MAL ID", func(t *testing.T) {
		_, ok := cm.Lookup(0)
		if ok {
			t.Error("expected !ok for zero MAL ID")
		}
	})
}

func TestLoadCommunityMapping_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tvdb-mal.yaml")
	content := `AnimeMap:
  - malid: 16498
    tvdbid: 12345
    tvdbseason: 1
    title: "Test Show"
  - malid: 99999
    tvdbid: 67890
    tvdbseason: 1
    title: "Another Show"
  - malid: 0
    tvdbid: 0
    tvdbseason: 0
    title: "Invalid Entry"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cm, err := LoadCommunityMapping(path)
	if err != nil {
		t.Fatalf("LoadCommunityMapping: %v", err)
	}

	if cm == nil {
		t.Fatal("expected non-nil CommunityMapping")
	}

	// Should have 2 entries (malid=0,tvdbid=0 filtered out)
	tvdbID, ok := cm.Lookup(16498)
	if !ok {
		t.Error("expected MAL 16498 to resolve")
	}
	if tvdbID != 12345 {
		t.Errorf("expected TVDB 12345, got %d", tvdbID)
	}

	tvdbID, ok = cm.Lookup(99999)
	if !ok {
		t.Error("expected MAL 99999 to resolve")
	}
	if tvdbID != 67890 {
		t.Errorf("expected TVDB 67890, got %d", tvdbID)
	}

	if _, ok := cm.Lookup(0); ok {
		t.Error("expected MAL 0 not to resolve")
	}
}

func TestLoadCommunityMapping_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")

	// This will attempt a network download, so we expect it to fail
	// with a non-nil error (file not found is not skipped since the
	// file doesn't exist and the download should fail in test env).
	_, err := LoadCommunityMapping(path)
	if err == nil {
		t.Skip("network request succeeded unexpectedly — test env may have internet access")
	}
}

func TestNewResolverAndProject(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{
		data: map[int]int{
			16498: 12345,
		},
	}
	r := NewResolver(cm)
	if r == nil {
		t.Fatal("expected non-nil Resolver")
	}

	t.Run("project known", func(t *testing.T) {
		shows := []model.Show{
			{ID: 1, IDMal: makePtr(16498), Title: model.Title{English: makePtr("Test Show")}},
		}
		result := r.Project(shows)
		if len(result) != 1 {
			t.Fatalf("expected 1 show, got %d", len(result))
		}
		if result[0].TVDBID != 12345 {
			t.Errorf("expected 12345, got %d", result[0].TVDBID)
		}
		if result[0].Title != "Test Show" {
			t.Errorf("expected 'Test Show', got %q", result[0].Title)
		}
	})

	t.Run("project zero MAL", func(t *testing.T) {
		shows := []model.Show{
			{ID: 1, IDMal: nil, Title: model.Title{English: makePtr("No MAL")}},
		}
		result := r.Project(shows)
		if len(result) != 0 {
			t.Errorf("expected 0 shows for nil MAL, got %d", len(result))
		}
	})

	t.Run("project unknown", func(t *testing.T) {
		shows := []model.Show{
			{ID: 1, IDMal: makePtr(1), Title: model.Title{English: makePtr("Unknown")}},
		}
		result := r.Project(shows)
		if len(result) != 0 {
			t.Errorf("expected 0 shows for unknown MAL, got %d", len(result))
		}
	})
}

func makePtr[T any](v T) *T {
	return &v
}
