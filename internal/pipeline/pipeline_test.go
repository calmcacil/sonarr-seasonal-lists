package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/calmcacil/anilistgen/internal/anilist"
	"github.com/calmcacil/anilistgen/internal/filter"
	"github.com/calmcacil/anilistgen/internal/mapping"
	"github.com/calmcacil/anilistgen/internal/model"
)

func makePtr[T any](v T) *T {
	return &v
}

func mockSeasonResponse(shows []model.Show) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Data struct {
				Page struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
						CurrentPage int  `json:"currentPage"`
					} `json:"pageInfo"`
					Media []model.Show `json:"media"`
				} `json:"Page"`
			} `json:"data"`
		}{}
		if shows == nil {
			shows = []model.Show{}
		}
		resp.Data.Page.Media = shows
		resp.Data.Page.PageInfo.HasNextPage = false
		resp.Data.Page.PageInfo.CurrentPage = 1

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func testDeps(baseURL string, resolver *mapping.Resolver) Deps {
	return Deps{
		AniClient:      anilist.NewWithBase(baseURL),
		Resolver:       resolver,
		FilterConfig:   filter.Config{},
		WinterOverflow: false,
		MaxPerYear:     100,
		AheadMonths:    12,
		Formats:        []string{"TV"},
	}
}

func testMapping(t *testing.T) *mapping.Resolver {
	t.Helper()
	cm := mapping.NewCommunityMapping(map[int]int{16498: 12345, 99999: 67890})
	return mapping.NewResolver(cm)
}

func TestProcess_OrdinarySeason(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	shows := []model.Show{
		{ID: 1, IDMal: makePtr(16498), Format: "TV", Title: model.Title{English: makePtr("Show One")}},
		{ID: 2, IDMal: makePtr(99999), Format: "TV", Title: model.Title{English: makePtr("Show Two")}},
		{ID: 3, IDMal: nil, Format: "TV", Title: model.Title{English: makePtr("Unmatched")}},
		{ID: 4, IDMal: makePtr(100), Format: "MOVIE", Title: model.Title{English: makePtr("Not a series")}},
	}

	srv := httptest.NewServer(mockSeasonResponse(shows))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	result := Process(context.Background(), deps, "SPRING", 2025)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Key.Season != "SPRING" || result.Key.Year != 2025 {
		t.Errorf("wrong key: %v", result.Key)
	}

	if len(result.All) != 2 {
		t.Errorf("expected 2 resolved All shows, got %d", len(result.All))
	}
	if len(result.NewOnly) != 2 {
		t.Errorf("expected 2 resolved NewOnly shows, got %d", len(result.NewOnly))
	}
}

func TestProcess_FetchError(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	result := Process(context.Background(), deps, "WINTER", 2025)
	if result.Err == nil {
		t.Fatal("expected error from failed fetch")
	}
	if len(result.All) != 0 || len(result.NewOnly) != 0 {
		t.Error("expected empty output on error")
	}
}

func TestProcess_ShortDurationFiltered(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	shows := []model.Show{
		{ID: 1, IDMal: makePtr(16498), Format: "TV", Duration: makePtr(6), Title: model.Title{English: makePtr("Short")}},
		{ID: 2, IDMal: makePtr(99999), Format: "TV", Duration: makePtr(24), Title: model.Title{English: makePtr("Normal")}},
	}

	srv := httptest.NewServer(mockSeasonResponse(shows))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	result := Process(context.Background(), deps, "SPRING", 2025)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if len(result.All) != 1 {
		t.Errorf("expected 1 show after duration filter, got %d", len(result.All))
	}
	if result.All[0].TVDBID != 67890 {
		t.Errorf("expected TVDB 67890 (Normal), got %d", result.All[0].TVDBID)
	}
}

func TestProcess_BlacklistFiltered(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	shows := []model.Show{
		{ID: 1, IDMal: makePtr(16498), Format: "TV", Title: model.Title{English: makePtr("Good Show")}},
		{ID: 2, IDMal: makePtr(99999), Format: "TV", Title: model.Title{English: makePtr("Bad Show")}},
	}

	srv := httptest.NewServer(mockSeasonResponse(shows))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)
	deps.FilterConfig = filter.Config{Blacklist: []string{"99999"}}

	result := Process(context.Background(), deps, "SPRING", 2025)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if len(result.All) != 1 {
		t.Errorf("expected 1 show after blacklist filter, got %d", len(result.All))
	}
	if result.All[0].TVDBID != 12345 {
		t.Errorf("expected TVDB 12345, got %d", result.All[0].TVDBID)
	}
}

func TestProcess_IsNewDetection(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	shows := []model.Show{
		{
			ID: 1, IDMal: makePtr(16498), Format: "TV",
			Title:     model.Title{English: makePtr("Original Show")},
			Relations: nil,
		},
		{
			ID: 2, IDMal: makePtr(99999), Format: "TV",
			Title: model.Title{English: makePtr("Sequel Show")},
			Relations: &model.RelationBlock{
				Edges: []model.RelationEdge{{RelationType: "PREQUEL"}},
			},
		},
	}

	srv := httptest.NewServer(mockSeasonResponse(shows))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	result := Process(context.Background(), deps, "SPRING", 2025)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if len(result.All) != 2 {
		t.Errorf("expected 2 in All (both are TV), got %d", len(result.All))
	}
	if len(result.NewOnly) != 1 {
		t.Errorf("expected 1 in NewOnly (only original), got %d", len(result.NewOnly))
	}
	if result.NewOnly[0].TVDBID != 12345 {
		t.Errorf("expected TVDB 12345 as the new-only show, got %d", result.NewOnly[0].TVDBID)
	}
}

func TestRun_MultiSeason(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	shows := []model.Show{
		{ID: 1, IDMal: makePtr(16498), Format: "TV", Season: makePtr("SPRING"), Title: model.Title{English: makePtr("Spring Show")}},
		{ID: 2, IDMal: makePtr(99999), Format: "TV", Season: makePtr("SUMMER"), Title: model.Title{English: makePtr("Summer Show")}},
	}

	srv := httptest.NewServer(mockSeasonResponse(shows))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	series, _, stats, errs := Run(context.Background(), deps, []int{2025}, []string{"SPRING", "SUMMER"})

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(stats) != 2 {
		t.Errorf("expected 2 stats, got %d", len(stats))
	}

	springKey := model.SeasonKey{Season: "SPRING", Year: 2025}
	summerKey := model.SeasonKey{Season: "SUMMER", Year: 2025}

	if s, ok := series[springKey]; !ok || len(s) != 1 {
		t.Errorf("expected 1 spring series, got %v", s)
	}
	if s, ok := series[summerKey]; !ok || len(s) != 1 {
		t.Errorf("expected 1 summer series, got %v", s)
	}
}

func TestRun_PartialFailure(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				Year int `json:"y"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if req.Variables.Year == 2024 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		shows := []model.Show{
			{ID: 1, IDMal: makePtr(16498), Format: "TV", Season: makePtr("SPRING"), Title: model.Title{English: makePtr("Good")}},
		}
		mockSeasonResponse(shows)(w, r)
	}))
	defer srv.Close()
	deps := testDeps(srv.URL, resolver)

	series, _, _, errs := Run(context.Background(), deps, []int{2024, 2025}, []string{"SPRING"})

	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
	}
	key2025 := model.SeasonKey{Season: "SPRING", Year: 2025}
	if s, ok := series[key2025]; !ok || len(s) != 1 {
		t.Errorf("expected 1 show for SPRING 2025 after failed 2024, got %v", s)
	}
}

func TestProcessBatch_NilInput(t *testing.T) {
	t.Parallel()

	resolver := testMapping(t)
	result := ProcessBatch(resolver, map[model.SeasonKey][]model.Show{
		{Season: "WINTER", Year: 2026}: nil,
	}, false)
	key := model.SeasonKey{Season: "WINTER", Year: 2026}
	if shows, ok := result[key]; !ok || len(shows) != 0 {
		t.Errorf("expected empty slice for nil input, got %v", shows)
	}
}
