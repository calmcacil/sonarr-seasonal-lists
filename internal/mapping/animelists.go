package mapping

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const defaultAnimeListsURL = "https://raw.githubusercontent.com/Anime-Lists/anime-lists/master/anime-list-full.xml"

type animeListXML struct {
	Anime []animeEntry `xml:"anime"`
}

type animeEntry struct {
	AnidbID       string `xml:"anidbid,attr"`
	TVDBID        string `xml:"tvdbid,attr"`
	DefaultSeason string `xml:"defaulttvdbseason,attr"`
	IMDBID        string `xml:"imdbid,attr"`
	TMDBID        string `xml:"tmdbid,attr"`
	TMDbTV        string `xml:"tmdbtv,attr"`
}

type AnimeListsMapping struct {
	data    map[int]int
	tmdbMap map[int]int
}

func LoadAnimeListsMapping(path string) (*AnimeListsMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Info("downloading anime-lists mapping", "url", defaultAnimeListsURL)
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Get(defaultAnimeListsURL)
		if err != nil {
			return nil, fmt.Errorf("download anime-lists: %w", err)
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read anime-lists response: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err == nil {
			os.WriteFile(path, data, 0600)
		}
	}

	var xmlData animeListXML
	if err := xml.Unmarshal(data, &xmlData); err != nil {
		return nil, fmt.Errorf("parse anime-lists XML: %w", err)
	}

	alm := &AnimeListsMapping{
		data:    make(map[int]int, len(xmlData.Anime)),
		tmdbMap: make(map[int]int),
	}
	for _, e := range xmlData.Anime {
		anidbID, err := strconv.Atoi(e.AnidbID)
		if err != nil || anidbID <= 0 {
			continue
		}
		if tvdbID, err := strconv.Atoi(e.TVDBID); err == nil && tvdbID > 0 {
			alm.data[anidbID] = tvdbID
		}
		if tmdbID, err := strconv.Atoi(e.TMDBID); err == nil && tmdbID > 0 {
			alm.tmdbMap[anidbID] = tmdbID
		}
	}
	slog.Info("loaded anime-lists mapping",
		"tvdb_entries", len(alm.data),
		"tmdb_entries", len(alm.tmdbMap),
		"path", path)
	return alm, nil
}

func (m *AnimeListsMapping) Lookup(anidbID int) (int, bool) {
	tvdbID, ok := m.data[anidbID]
	return tvdbID, ok
}

func (m *AnimeListsMapping) LookupTMDB(anidbID int) (int, bool) {
	tmdbID, ok := m.tmdbMap[anidbID]
	return tmdbID, ok
}
