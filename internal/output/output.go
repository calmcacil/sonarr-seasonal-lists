package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Show struct {
	TVDBID int    `json:"tvdbId,omitempty"`
	TMDBID int    `json:"tmdbId,omitempty"`
	Title  string `json:"title,omitempty"`
}

func WriteSeasonJSON(dir, category, season string, year int, shows []Show) error {
	yearDir := filepath.Join(dir, fmt.Sprintf("%d", year))
	filename := fmt.Sprintf("%s-%s.json", strings.ToLower(season), category)
	return writeJSON(yearDir, filename, shows)
}

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

func WriteAllJSON(outputDir, category string, seasonal map[string][]Show) error {
	byYear := map[int][]Show{}

	for key, shows := range seasonal {
		parts := strings.SplitN(key, "-", 2)
		if len(parts) != 2 {
			continue
		}
		season := parts[0]
		var year int
		if _, err := fmt.Sscanf(parts[1], "%d", &year); err != nil {
			continue
		}
		if err := WriteSeasonJSON(outputDir, category, season, year, shows); err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
		byYear[year] = append(byYear[year], shows...)
	}

	for year, shows := range byYear {
		if err := WriteYearJSON(outputDir, category, year, shows); err != nil {
			return fmt.Errorf("write year %d: %w", year, err)
		}
	}

	return nil
}
