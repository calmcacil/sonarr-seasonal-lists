package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	apiBase         = "https://graphql.anilist.co"
	maxRetry        = 3
	rateLimitDelay  = 500 * time.Millisecond
)

// RelationEdge represents a related media entry.
type RelationEdge struct {
	Node         RelationNode `json:"node"`
	RelationType string       `json:"relationType"`
}

// RelationNode holds minimal data for a related media entry.
type RelationNode struct {
	ID    int   `json:"id"`
	IDMal *int  `json:"idMal"`
	Title Title `json:"title"`
}

// Tag represents an AniList content tag with name and relevance rank.
type Tag struct {
	Name string `json:"name"`
}

// Show represents an anime show from the AniList API.
type Show struct {
	ID        int            `json:"id"`
	IDMal     *int           `json:"idMal"`
	Title     Title          `json:"title"`
	Format    string         `json:"format"`
	Episodes  *int           `json:"episodes"`
	Duration  *int           `json:"duration"`
	Genres    []string       `json:"genres"`
	Tags      []Tag          `json:"tags"`
	Status    string         `json:"status"`
	Relations *RelationBlock `json:"relations,omitempty"`
}

// RelationBlock holds the edges wrapper.
type RelationBlock struct {
	Edges []RelationEdge `json:"edges"`
}

// RelationMALIDsByType returns all non-nil MAL IDs from relations matching
// the given relation types (e.g. "PREQUEL", "ADAPTATION", "SIDE_STORY").
// If types is empty, no relations are returned (safe default).
func (s Show) RelationMALIDsByType(types []string) []int {
	if s.Relations == nil || len(types) == 0 {
		return nil
	}

	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	var ids []int
	for _, e := range s.Relations.Edges {
		if typeSet[e.RelationType] && e.Node.IDMal != nil && *e.Node.IDMal > 0 {
			ids = append(ids, *e.Node.IDMal)
		}
	}
	return ids
}

// SkipByDuration returns true if the show should be skipped because its
// per-episode duration is ≤ 10 minutes, or its total runtime
// (duration × episodes) is ≤ 10 minutes (single-episode shorts).
func (s Show) SkipByDuration() bool {
	if s.Duration != nil && *s.Duration <= 10 {
		return true
	}
	// Total runtime check: a single very short episode (e.g. 6 min × 1).
	if s.Duration != nil && s.Episodes != nil {
		total := *s.Duration * *s.Episodes
		if total <= 10 {
			return true
		}
	}
	return false
}

// HasTag returns true if the show has a tag matching the given name
// (case-insensitive).
func (s Show) HasTag(name string) bool {
	lower := strings.ToLower(name)
	for _, t := range s.Tags {
		if strings.ToLower(t.Name) == lower {
			return true
		}
	}
	return false
}

// Title holds the english and romaji titles.
type Title struct {
	English *string `json:"english"`
	Romaji  *string `json:"romaji"`
}

// DisplayTitle returns the English title if available, falling back to romaji.
func (s Show) DisplayTitle() string {
	if s.Title.English != nil && *s.Title.English != "" {
		return *s.Title.English
	}
	if s.Title.Romaji != nil {
		return *s.Title.Romaji
	}
	return fmt.Sprintf("Anime #%d", s.ID)
}

// GraphQL query for fetching seasonal anime.
const queryTemplate = `query($s: MediaSeason, $y: Int, $page: Int, $perPage: Int, $formats: [MediaFormat]) {
	Page(page: $page, perPage: $perPage) {
		media(
			season: $s, seasonYear: $y,
			type: ANIME,
			sort: POPULARITY_DESC,
			format_in: $formats
		) {
			id
			idMal
			title { romaji english }
			format
			episodes
			duration
			genres
			tags { name }
			status
			relations {
				edges {
					node {
						id
						idMal
						title { romaji english }
					}
					relationType
				}
			}
		}
	}
}`

type graphqlError struct {
	Message string `json:"message"`
}

// graphqlResponse is the top-level response from AniList.
type graphqlResponse struct {
	Data struct {
		Page struct {
			Media []Show `json:"media"`
		} `json:"Page"`
	} `json:"data"`
	Errors []graphqlError `json:"errors,omitempty"`
}

// Client fetches data from the AniList GraphQL API.
type Client struct {
	http     *http.Client
	lastCall time.Time
}

// New creates a new AniList client.
func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// throttle ensures we don't exceed AniList rate limits.
func (c *Client) throttle() {
	elapsed := time.Since(c.lastCall)
	if elapsed < rateLimitDelay {
		time.Sleep(rateLimitDelay - elapsed)
	}
	c.lastCall = time.Now()
}

// FetchSeason returns all TV/ONA anime for the given season and year.
// If includeONA is true, both TV and ONA formats are fetched; otherwise only TV.
// Results are capped at maxResults.
func (c *Client) FetchSeason(ctx context.Context, season string, year int, maxResults int, includeONA bool) ([]Show, error) {
	c.throttle()

	formats := []string{"TV"}
	if includeONA {
		formats = append(formats, "ONA")
	}

	payload := map[string]any{
		"query": queryTemplate,
		"variables": map[string]any{
			"s":       season,
			"y":       year,
			"page":    1,
			"perPage": maxResults,
			"formats": formats,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var resp graphqlResponse
	if err := c.doRequest(ctx, body, &resp); err != nil {
		return nil, fmt.Errorf("fetch %s %d: %w", season, year, err)
	}

	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("AniList GraphQL errors: %s", strings.Join(msgs, "; "))
	}

	shows := resp.Data.Page.Media
	if shows == nil {
		shows = []Show{}
	}

	return shows, nil
}

// Ping checks connectivity to the AniList API by fetching a single result.
func (c *Client) Ping(ctx context.Context) error {
	c.throttle()

	query := `{ Page(perPage: 1) { media(type: ANIME) { id } } }`
	payload := map[string]any{
		"query":     query,
		"variables": map[string]any{},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal ping payload: %w", err)
	}

	var result struct {
		Data struct {
			Page struct {
				Media []struct {
					ID int `json:"id"`
				} `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	if err := c.doRequest(ctx, body, &result); err != nil {
		return fmt.Errorf("AniList ping failed: %w", err)
	}

	return nil
}

// doRequest sends a POST request with retries and exponential backoff.
func (c *Client) doRequest(ctx context.Context, payload []byte, dst any) error {
	var lastErr error
	for attempt := range maxRetry {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase,
			bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (attempt %d)", attempt+1)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		err = json.NewDecoder(resp.Body).Decode(dst)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("decode response: %w", err)
			continue
		}

		return nil
	}

	return fmt.Errorf("giving up after %d retries: %w", maxRetry, lastErr)
}
