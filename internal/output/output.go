package output

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/calmcacil/anilistgen/internal/model"
)

//go:embed index.html
var indexTemplate string

var indexTmpl = template.Must(template.New("index").Parse(indexTemplate))

type Show struct {
	TVDBID int    `json:"tvdbId"`
	Title  string `json:"title,omitempty"`
}

// WriteSeasonJSON writes a compact JSON array of shows for a single
// season and year (e.g., 2026/winter-series.json).
func WriteSeasonJSON(dir, category, season string, year int, shows []Show) error {
	yearDir := filepath.Join(dir, fmt.Sprintf("%d", year))
	filename := fmt.Sprintf("%s-%s.json", strings.ToLower(season), category)
	return writeJSON(yearDir, filename, shows)
}

// WriteYearJSON writes a compact JSON array of shows aggregated across all
// seasons for a given year (e.g., 2026/series.json).
func WriteYearJSON(dir, category string, year int, shows []Show) error {
	yearDir := filepath.Join(dir, fmt.Sprintf("%d", year))
	filename := fmt.Sprintf("%s.json", category)
	return writeJSON(yearDir, filename, shows)
}

func writeJSON(dir, filename string, shows []Show) error {
	if shows == nil {
		shows = []Show{}
	}
	data, err := json.Marshal(shows)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	outPath := filepath.Join(dir, filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}
	return nil
}

// WriteAllJSON writes per-season JSON files, yearly aggregates, and (for the
// "series" category) an HTML index page with Sonarr setup instructions.
func WriteAllJSON(outputDir, baseURL, category string, seasonal map[model.SeasonKey][]Show, indexYears []int) error {
	// Aggregate per-season shows into year buckets, walking in sorted key
	// order for deterministic yearly output.
	keys := make([]model.SeasonKey, 0, len(seasonal))
	for k := range seasonal {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Year != keys[j].Year {
			return keys[i].Year < keys[j].Year
		}
		return keys[i].Season < keys[j].Season
	})

	byYear := map[int][]Show{}

	for _, key := range keys {
		shows := seasonal[key]
		if err := WriteSeasonJSON(outputDir, category, key.Season, key.Year, shows); err != nil {
			return fmt.Errorf("write %s: %w", key.String(), err)
		}
		byYear[key.Year] = append(byYear[key.Year], shows...)
	}

	for year, shows := range byYear {
		if err := WriteYearJSON(outputDir, category, year, shows); err != nil {
			return fmt.Errorf("write year %d: %w", year, err)
		}
	}

	if category == "series" {
		yearSet := make(map[int]struct{}, len(byYear)+len(indexYears))
		for y := range byYear {
			yearSet[y] = struct{}{}
		}
		for _, y := range indexYears {
			yearSet[y] = struct{}{}
		}
	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)
	if err := WriteIndex(outputDir, baseURL, years); err != nil {
			return fmt.Errorf("write index: %w", err)
		}
	}

	return nil
}

// WriteIndex generates an HTML index page with year selector, season boxes,
// and Sonarr import list setup instructions.
func WriteIndex(dir, baseURL string, years []int) error {
	if len(years) == 0 {
		years = append(years, time.Now().Year())
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	now := time.Now().Year()
	var yearOpts string
	for _, y := range years {
		sel := ""
		if y == now {
			sel = " selected"
		}
		yearOpts += fmt.Sprintf("      <option value=\"%d\"%s>%d</option>\n", y, sel, y)
	}

	tmpl, err := indexTmpl.Clone()
	if err != nil {
		return fmt.Errorf("clone index template: %w", err)
	}

	type tmplData struct {
		BaseURL     string
		YearOptions template.HTML
	}

	indexPath := filepath.Join(dir, "index.html")
	f, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("create index.html: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, tmplData{
		BaseURL:     baseURL,
		YearOptions: template.HTML(yearOpts),
	}); err != nil {
		return fmt.Errorf("render index template: %w", err)
	}

	return nil
}
