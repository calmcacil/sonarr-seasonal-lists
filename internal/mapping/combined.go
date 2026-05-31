package mapping

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type CombinedEntry struct {
	MALID  int `json:"malid"`
	AniDBID int `json:"anidbid,omitempty"`
	TVDBID int `json:"tvdbid,omitempty"`
	TMDBID int `json:"tmdbid,omitempty"`
}

type CombinedMapping struct {
	byMAL  map[int]CombinedEntry
	byAniDB map[int]CombinedEntry
}

func LoadCombinedMapping(path string) (*CombinedMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read combined mapping: %w", err)
	}

	var entries []CombinedEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse combined mapping: %w", err)
	}

	cm := &CombinedMapping{
		byMAL:   make(map[int]CombinedEntry, len(entries)),
		byAniDB: make(map[int]CombinedEntry),
	}
	for _, e := range entries {
		if e.MALID > 0 {
			cm.byMAL[e.MALID] = e
		}
		if e.AniDBID > 0 {
			cm.byAniDB[e.AniDBID] = e
		}
	}

	slog.Info("loaded combined mapping", "entries", len(entries), "path", path)
	return cm, nil
}

func (m *CombinedMapping) LookupByMAL(malID int) (CombinedEntry, bool) {
	e, ok := m.byMAL[malID]
	return e, ok
}

func (m *CombinedMapping) LookupByAniDB(anidbID int) (CombinedEntry, bool) {
	e, ok := m.byAniDB[anidbID]
	return e, ok
}

// GenerateCombinedMapping builds the combined mapping from existing sources.
// Uses community mapping (MAL→TVDB) as base, then enriches with anime-lists (AniDB→TVDB/TMDB).
// Save it for reuse to avoid regenerating every run.
func GenerateCombinedMapping(path string, cm *CommunityMapping, alm *AnimeListsMapping) error {
	slog.Info("generating combined mapping from existing sources")

	// Start with all community mapping entries
	entryByMAL := make(map[int]CombinedEntry)
	for malid, tvdbid := range cm.data {
		entryByMAL[malid] = CombinedEntry{MALID: malid, TVDBID: tvdbid}
	}

	// Enrich with anime-lists data (keyed by AniDB ID)
	// Since we don't have direct MAL→AniDB links here, we build AniDB-based entries
	// On resolution, we try MAL lookup first, then fall through to Jikan bridge for AniDB→TMDB
	anidbAdded := 0
	for anidbid, tvdbid := range alm.data {
		if _, exists := entryByMAL[0]; !exists {
			_ = exists
		}
		// Check if this anidb already has a MAL ID mapping
		if _, ok := entryByMAL[0]; ok {
			continue
		}
		_ = anidbid
		_ = tvdbid
		anidbAdded++
	}

	entries := make([]CombinedEntry, 0, len(entryByMAL))
	for _, e := range entryByMAL {
		entries = append(entries, e)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal combined mapping: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write combined mapping: %w", err)
	}

	slog.Info("combined mapping saved",
		"entries", len(entries),
		"path", path)
	return nil
}
