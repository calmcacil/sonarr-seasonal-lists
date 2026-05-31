package mapping

import (
	"context"
	"testing"

	"github.com/calmcacil/anilistgen/internal/anilist"
)

func makePtr[T any](v T) *T {
	return &v
}

func TestResolver_Resolve_CommunityMapping(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{data: map[int]int{16498: 12345}}
	alm := &AnimeListsMapping{data: map[int]int{}, tmdbMap: map[int]int{}}

	r := NewResolver(cm, alm, nil, nil)
	tvdb, tmdb, ok := r.Resolve(context.Background(), 16498, 999, "Test", false)
	if !ok {
		t.Fatal("expected resolution to succeed")
	}
	if tvdb != 12345 {
		t.Errorf("expected TVDB 12345, got %d", tvdb)
	}
	if tmdb != 0 {
		t.Errorf("expected TMDB 0, got %d", tmdb)
	}
}

func TestResolver_Resolve_NoMatch(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{data: map[int]int{}}
	alm := &AnimeListsMapping{data: map[int]int{}, tmdbMap: map[int]int{}}

	r := NewResolver(cm, alm, nil, nil)
	_, _, ok := r.Resolve(context.Background(), 16498, 999, "Test", false)
	if ok {
		t.Error("expected resolution to fail")
	}
}

func TestResolver_Resolve_ZeroMALID_NotMovie(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{data: map[int]int{16498: 12345}}
	alm := &AnimeListsMapping{data: map[int]int{}, tmdbMap: map[int]int{}}

	r := NewResolver(cm, alm, nil, nil)
	_, _, ok := r.Resolve(context.Background(), 0, 999, "Test", false)
	if ok {
		t.Error("expected resolution to fail for MAL ID 0 when not a movie")
	}
}

func TestResolver_ResolveBatch(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{data: map[int]int{100: 5000}}
	alm := &AnimeListsMapping{data: map[int]int{}, tmdbMap: map[int]int{}}

	r := NewResolver(cm, alm, nil, nil)
	shows := []anilist.Show{
		{ID: 1, IDMal: makePtr(100), Title: anilist.Title{English: makePtr("Show A")}},
		{ID: 2, IDMal: makePtr(101), Title: anilist.Title{English: makePtr("Show B")}},
		{ID: 3, IDMal: makePtr(102), Title: anilist.Title{English: makePtr("Show C")}},
	}

	result := r.ResolveBatch(context.Background(), shows, false)
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	if !result[1].Resolved {
		t.Error("expected show 1 to be resolved")
	}
	if result[1].TVDBID != 5000 {
		t.Errorf("expected TVDB 5000, got %d", result[1].TVDBID)
	}
	if result[2].Resolved {
		t.Error("expected show 2 to be unresolved")
	}
	if result[3].Resolved {
		t.Error("expected show 3 to be unresolved")
	}
}

func TestResolver_Resolve_TMDBFromAnimeLists(t *testing.T) {
	t.Parallel()

	cm := &CommunityMapping{data: map[int]int{}}
	alm := &AnimeListsMapping{data: map[int]int{}, tmdbMap: map[int]int{}}

	r := NewResolver(cm, alm, nil, nil)
	_, _, ok := r.Resolve(context.Background(), 16498, 123, "Test", false)
	if ok {
		t.Error("expected resolution to fail without Jikan client")
	}
}
