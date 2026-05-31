package mapping

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const jikanBase = "https://api.jikan.moe/v4"
const jikanRateLimit = 1200 * time.Millisecond

type JikanClient struct {
	http     *http.Client
	lastCall time.Time
	cache    map[int]int // MAL ID → AniDB ID
	cachePath string
}

func NewJikanClient(cachePath string) *JikanClient {
	c := &JikanClient{
		http:      &http.Client{Timeout: 15 * time.Second},
		cache:     map[int]int{},
		cachePath: cachePath,
	}
	c.loadCache()
	return c
}

func (c *JikanClient) loadCache() {
	if c.cachePath == "" {
		return
	}
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &c.cache)
	slog.Info("loaded jikan cache", "entries", len(c.cache), "path", c.cachePath)
}

func (c *JikanClient) saveCache() {
	if c.cachePath == "" || len(c.cache) == 0 {
		return
	}
	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(c.cachePath), 0755)
	os.WriteFile(c.cachePath, data, 0600)
}

func (c *JikanClient) throttle() {
	elapsed := time.Since(c.lastCall)
	if elapsed < jikanRateLimit {
		time.Sleep(jikanRateLimit - elapsed)
	}
	c.lastCall = time.Now()
}

// MALToAniDB fetches the AniDB ID for a given MAL ID via Jikan API.
// Checks cache first, only calls API on miss. Saves cache on new lookups.
func (c *JikanClient) MALToAniDB(ctx context.Context, malID int) (int, error) {
	if id, ok := c.cache[malID]; ok {
		return id, nil
	}

	c.throttle()

	u := fmt.Sprintf("%s/anime/%d/external", jikanBase, malID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Jikan API error (HTTP %d)", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	for _, entry := range result.Data {
		if strings.Contains(strings.ToLower(entry.Name), "anidb") {
			parsed, err := url.Parse(entry.URL)
			if err != nil {
				continue
			}
			aid := parsed.Query().Get("aid")
			if aid == "" {
				// Try extracting from path for direct URLs like /anime/12345
				parts := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
				aid = parts[len(parts)-1]
			}
			var id int
			if _, err := fmt.Sscanf(aid, "%d", &id); err == nil && id > 0 {
				c.cache[malID] = id
				c.saveCache()
				return id, nil
			}
		}
	}

	return 0, fmt.Errorf("no AniDB link found for MAL %d", malID)
}
