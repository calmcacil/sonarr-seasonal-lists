package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Show struct {
	TVDBID int    `json:"tvdbId"`
	Title  string `json:"title,omitempty"`
}

func WriteSeasonJSON(dir, season string, year int, shows []Show) error {
	filename := fmt.Sprintf("%s-%d.json", strings.ToLower(season), year)
	return writeJSON(dir, filename, shows)
}

func WriteYearJSON(dir string, year int, shows []Show) error {
	filename := fmt.Sprintf("%d.json", year)
	return writeJSON(dir, filename, shows)
}

func writeJSON(dir, filename string, shows []Show) error {
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

func WriteAllJSON(outputDir string, seasonal map[string][]Show) error {
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
		if err := WriteSeasonJSON(outputDir, season, year, shows); err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
		byYear[year] = append(byYear[year], shows...)
	}

	for year, shows := range byYear {
		if err := WriteYearJSON(outputDir, year, shows); err != nil {
			return fmt.Errorf("write year %d: %w", year, err)
		}
	}

	return nil
}
