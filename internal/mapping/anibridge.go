package mapping

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

const defaultAnibridgeURL = "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst"

func DefaultAnibridgePath() string {
	dir := os.TempDir()
	return filepath.Join(dir, "anilistgen_anibridge.json.zst")
}

type anibridgeMeta struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedOn   string `json:"generated_on"`
}

type AnibridgeMapping struct {
	byMAL     map[int]int
	byAniList map[int]int
}

func NewAnibridgeMapping(byMAL, byAniList map[int]int) *AnibridgeMapping {
	return &AnibridgeMapping{byMAL: byMAL, byAniList: byAniList}
}

func (m *AnibridgeMapping) LookupByMAL(malID int) (int, bool) {
	tvdbID, ok := m.byMAL[malID]
	return tvdbID, ok
}

func (m *AnibridgeMapping) LookupByAniList(anilistID int) (int, bool) {
	tvdbID, ok := m.byAniList[anilistID]
	return tvdbID, ok
}

func LoadAnibridgeMapping(path string) (*AnibridgeMapping, error) {
	return LoadAnibridgeMappingWithAge(path, 0)
}

func LoadAnibridgeMappingWithAge(path string, maxAge time.Duration) (*AnibridgeMapping, error) {
	exists := false
	if fi, err := os.Stat(path); err == nil {
		exists = true
		if maxAge <= 0 || time.Since(fi.ModTime()) < maxAge {
			return parseAnibridgeMapping(path)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat anibridge mapping: %w", err)
	}

	if exists {
		slog.Info("anibridge mapping is stale, re-downloading", "path", path, "maxAge", maxAge)
	} else {
		slog.Info("anibridge mapping not found, downloading", "path", path)
	}

	if err := downloadAnibridgeMapping(path); err != nil {
		slog.Warn("download failed, using cached mapping", "error", err)
		if !exists {
			return nil, fmt.Errorf("anibridge mapping not found and download failed: %w", err)
		}
	}

	return parseAnibridgeMapping(path)
}

func downloadAnibridgeMapping(path string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(defaultAnibridgeURL)
	if err != nil {
		return fmt.Errorf("download anibridge mapping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download anibridge mapping: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read anibridge mapping response: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("could not create mapping directory", "path", filepath.Dir(path), "error", err)
	} else if err := os.WriteFile(path, data, 0600); err != nil {
		slog.Warn("could not cache mapping file", "path", path, "error", err)
	}
	return nil
}

func parseAnibridgeMapping(path string) (*AnibridgeMapping, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open anibridge mapping: %w", err)
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("create zstd reader: %w", err)
	}
	defer zr.Close()

	return parseAnibridgeJSON(zr, path)
}

func parseAnibridgeJSON(r io.Reader, path string) (*AnibridgeMapping, error) {
	dec := json.NewDecoder(r)

	t, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse anibridge JSON: expected opening brace: %w", err)
	}
	if t != json.Delim('{') {
		return nil, fmt.Errorf("parse anibridge JSON: expected '{', got %T(%v)", t, t)
	}

	start := time.Now()
	byMAL := map[int]int{}
	byAniList := map[int]int{}

	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parse anibridge JSON: key token: %w", err)
		}
		key, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("parse anibridge JSON: expected string key, got %T", t)
		}

		switch {
		case strings.HasPrefix(key, "mal:"):
			id, convErr := strconv.Atoi(key[4:])
			if convErr != nil || id <= 0 {
				skipValue(dec)
				continue
			}
			if tvdbID, ok := extractTVDB(dec); ok {
				byMAL[id] = tvdbID
			}

		case strings.HasPrefix(key, "anilist:"):
			id, convErr := strconv.Atoi(key[8:])
			if convErr != nil || id <= 0 {
				skipValue(dec)
				continue
			}
			if tvdbID, ok := extractTVDB(dec); ok {
				byAniList[id] = tvdbID
			}

		case key == "$meta":
			var meta anibridgeMeta
			if err := dec.Decode(&meta); err != nil {
				slog.Warn("failed to decode mapping metadata", "error", err)
			} else {
				slog.Info("anibridge dataset",
					"schema_version", meta.SchemaVersion,
					"generated_on", meta.GeneratedOn)
			}

		default:
			skipValue(dec)
		}
	}

	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("parse anibridge JSON: expected closing brace: %w", err)
	}

	slog.Info("loaded anibridge mapping",
		"mal_entries", len(byMAL), "anilist_entries", len(byAniList),
		"parse_ms", time.Since(start).Milliseconds(),
		"path", path)

	return &AnibridgeMapping{byMAL: byMAL, byAniList: byAniList}, nil
}

func skipValue(dec *json.Decoder) {
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		slog.Warn("skip value failed", "error", err)
	}
}

func extractTVDB(dec *json.Decoder) (int, bool) {
	var targets map[string]json.RawMessage
	if err := dec.Decode(&targets); err != nil {
		return 0, false
	}

	bestTVDB := 0
	bestEpCount := 0
	foundS1 := false

	for descriptor, rawValue := range targets {
		if !strings.HasPrefix(descriptor, "tvdb_show:") {
			continue
		}

		parts := strings.SplitN(descriptor, ":", 3)
		if len(parts) < 3 {
			continue
		}
		tvdbID, convErr := strconv.Atoi(parts[1])
		if convErr != nil || tvdbID <= 0 {
			continue
		}
		scope := parts[2]

		epCount := countSourceEpisodes(rawValue)

		if scope == "s1" {
			bestTVDB = tvdbID
			bestEpCount = epCount
			foundS1 = true
			continue
		}
		if !foundS1 && epCount > bestEpCount {
			bestTVDB = tvdbID
			bestEpCount = epCount
		}
	}

	if bestTVDB > 0 {
		return bestTVDB, true
	}
	return 0, false
}

func countSourceEpisodes(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}

	var ranges map[string]string
	if err := json.Unmarshal(raw, &ranges); err != nil {
		return 0
	}

	var total int
	for srcRange := range ranges {
		if srcRange == "" {
			continue
		}
		parts := strings.SplitN(srcRange, "-", 2)
		if len(parts) == 1 {
			if ep, err := strconv.Atoi(parts[0]); err == nil && ep > 0 {
				total++
			}
			continue
		}
		if parts[1] == "" {
			if start, err := strconv.Atoi(parts[0]); err == nil && start > 0 {
				total++
			}
			continue
		}
		start, startErr := strconv.Atoi(parts[0])
		end, endErr := strconv.Atoi(parts[1])
		if startErr == nil && endErr == nil && start > 0 && end >= start {
			total += end - start + 1
		}
	}
	return total
}
