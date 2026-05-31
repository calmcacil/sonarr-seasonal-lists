package mapping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const tmdbBase = "https://api.themoviedb.org/3"
const tmdbRateLimit = 300 * time.Millisecond

type TMDBClient struct {
	http     *http.Client
	token    string
	lastCall time.Time
}

func NewTMDBClient(token string) *TMDBClient {
	return &TMDBClient{
		http:  &http.Client{Timeout: 10 * time.Second},
		token: token,
	}
}

func (c *TMDBClient) throttle() {
	elapsed := time.Since(c.lastCall)
	if elapsed < tmdbRateLimit {
		time.Sleep(tmdbRateLimit - elapsed)
	}
	c.lastCall = time.Now()
}

type tmdbSearchResult struct {
	Results []struct {
		ID         int     `json:"id"`
		Title      string  `json:"title"`
		Popularity float64 `json:"popularity"`
	} `json:"results"`
}

func (c *TMDBClient) newRequest(ctx context.Context, u string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// SearchMovie searches TMDB for a movie by title and optional year.
// Returns the best matching TMDB ID, or 0 if not found.
func (c *TMDBClient) SearchMovie(ctx context.Context, title string, year int) (int, error) {
	c.throttle()

	query := cleanMovieTitle(title)
	u := fmt.Sprintf("%s/search/movie?query=%s&language=en-US",
		tmdbBase, strings.ReplaceAll(query, " ", "+"))
	if year > 0 {
		u += fmt.Sprintf("&year=%d", year)
	}

	req, err := c.newRequest(ctx, u)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("TMDB API error (HTTP %d)", resp.StatusCode)
	}

	var result tmdbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Results) == 0 {
		return 0, nil
	}

	best := result.Results[0]
	for _, r := range result.Results[1:] {
		if r.Popularity > best.Popularity {
			best = r
		}
	}

	return best.ID, nil
}

// cleanMovieTitle strips suffixes that confuse TMDB search.
func cleanMovieTitle(title string) string {
	cleaned := title
	for {
		start := strings.Index(cleaned, "(")
		end := strings.Index(cleaned, ")")
		if start >= 0 && end > start {
			cleaned = strings.TrimSpace(cleaned[:start] + cleaned[end+1:])
		} else {
			break
		}
	}

	idx := strings.Index(cleaned, ":")
	if idx > 0 {
		before := strings.TrimSpace(cleaned[:idx])
		if len(strings.Fields(before)) >= 2 || containsJapanese(before) {
			cleaned = before
		}
	}

	return strings.TrimSpace(cleaned)
}

func containsJapanese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}
