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
	am := mapping.NewAnibridgeMapping(
		map[int]int{16498: 12345, 99999: 67890},
		map[int]int{},
	)
	return mapping.NewResolver(am)
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
