package mdblist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	apiBase   = "https://api.mdblist.com"
	maxRetry  = 3
	rateLimit = 1100 * time.Millisecond
)

// List represents an MDBList list.
type List struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Items       int    `json:"items"`
	Private     bool   `json:"private"`
	UserName    string `json:"user_name"`
	URL         string `json:"url,omitempty"`
}

// GetURL returns the public URL for the list.
func (l List) GetURL() string {
	if l.URL != "" {
		return l.URL
	}
	if l.UserName != "" && l.Slug != "" {
		return fmt.Sprintf("https://mdblist.com/lists/%s/%s", l.UserName, l.Slug)
	}
	return ""
}

// MediaIDs holds external IDs for a media item from MDBList.
type MediaIDs struct {
	IMDB string `json:"imdb"`
	TMDB int    `json:"tmdb"`
	TVDB int    `json:"tvdb"`
	MAL  int    `json:"mal"`
}

// MediaInfo represents a media item from MDBList's batch lookup.
type MediaInfo struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Year    int      `json:"year"`
	Runtime int      `json:"runtime"`
	IDs     MediaIDs `json:"ids"`
	Type    string   `json:"type"`
}

// Client manages communication with the MDBList API.
type Client struct {
	http     *http.Client
	apiKey   string
	lastCall time.Time
	mu       sync.Mutex
}

// New creates a new MDBList client.
func New(apiKey string) *Client {
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
	}
}

// throttle ensures we don't exceed MDBList rate limits.
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastCall)
	if elapsed < rateLimit {
		time.Sleep(rateLimit - elapsed)
	}
	c.lastCall = time.Now()
}

// Ping checks connectivity to the MDBList API.
func (c *Client) Ping(ctx context.Context) error {
	c.throttle()
	u := fmt.Sprintf("%s/user?apikey=%s", apiBase, url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MDBList API error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// ListLists returns all lists belonging to the authenticated user.
func (c *Client) ListLists(ctx context.Context) ([]List, error) {
	c.throttle()
	u := fmt.Sprintf("%s/lists/user?apikey=%s", apiBase, url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MDBList API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var lists []List
	if err := json.Unmarshal(body, &lists); err != nil {
		return nil, fmt.Errorf("parse lists response: %w (raw: %s)", err, string(body))
	}

	return lists, nil
}

// FindListByTitle searches the user's lists for one with a matching title.
func (c *Client) FindListByTitle(ctx context.Context, title string) (*List, error) {
	lists, err := c.ListLists(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, title) {
			return &l, nil
		}
	}
	return nil, nil
}

// CreateList creates a new static list and returns it.
func (c *Client) CreateList(ctx context.Context, name, description string, public bool) (*List, error) {
	c.throttle()

	payload := map[string]any{
		"name":        name,
		"description": description,
	}
	if public {
		payload["public"] = 1
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	u := fmt.Sprintf("%s/lists/user/add?apikey=%s", apiBase, url.QueryEscape(c.apiKey))

	var list List
	if err := c.doMutation(ctx, http.MethodPost, u, body, &list); err != nil {
		return nil, fmt.Errorf("create list %q: %w", name, err)
	}

	return &list, nil
}

// AddItems adds items to a static list by provider IDs (e.g. imdb, trakt, tmdb, tvdb).
// items is a map like {"imdb": "tt0903747"} or {"tmdb": 1396}.
func (c *Client) AddItems(ctx context.Context, listID int, items []map[string]any) error {
	c.throttle()
	u := fmt.Sprintf("%s/lists/%d/items/add?apikey=%s", apiBase, listID, url.QueryEscape(c.apiKey))

	payload := map[string]any{
		"shows": items,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	return c.doMutation(ctx, http.MethodPost, u, body, nil)
}

// RemoveItems removes items from a static list.
func (c *Client) RemoveItems(ctx context.Context, listID int, items []map[string]any) error {
	c.throttle()
	u := fmt.Sprintf("%s/lists/%d/items/remove?apikey=%s", apiBase, listID, url.QueryEscape(c.apiKey))

	payload := map[string]any{
		"shows": items,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	return c.doMutation(ctx, http.MethodPost, u, body, nil)
}

// DeleteAndRecreate deletes and recreates a list with new items.
// This is the most reliable way to replace all items.
func (c *Client) DeleteAndRecreate(ctx context.Context, listID int, name, description string, public bool, items []map[string]any) (*List, error) {
	// Delete existing list
	c.throttle()
	u := fmt.Sprintf("%s/lists/%d?apikey=%s", apiBase, listID, url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create delete request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delete request: %w", err)
	}
	resp.Body.Close()

	// Create new list with items
	newList, err := c.CreateList(ctx, name, description, public)
	if err != nil {
		return nil, fmt.Errorf("recreate list: %w", err)
	}

	// Add items one batch at a time
	if len(items) > 0 {
		// MDBList allows up to 200 items per request typically
		const batchSize = 200
		for i := 0; i < len(items); i += batchSize {
			end := i + batchSize
			if end > len(items) {
				end = len(items)
			}
			if err := c.AddItems(ctx, newList.ID, items[i:end]); err != nil {
				return nil, fmt.Errorf("add items batch: %w", err)
			}
		}
	}

	return newList, nil
}

// BatchLookupByMAL looks up media by a list of MAL IDs.
// Returns a map of malID -> MediaInfo for found items (unfound IDs are omitted).
const batchLookupSize = 15

// LookupByMAL checks a single MAL ID against MDBList's database.
// Returns nil if not found.
func (c *Client) LookupByMAL(ctx context.Context, malID int) (*MediaInfo, error) {
	result, err := c.BatchLookupByMAL(ctx, []int{malID})
	if err != nil {
		return nil, err
	}
	if info, ok := result[malID]; ok {
		return &info, nil
	}
	return nil, nil
}

func (c *Client) BatchLookupByMAL(ctx context.Context, malIDs []int) (map[int]MediaInfo, error) {
	if len(malIDs) == 0 {
		return map[int]MediaInfo{}, nil
	}

	resultMap := make(map[int]MediaInfo, len(malIDs))

	for i := 0; i < len(malIDs); i += batchLookupSize {
		end := i + batchLookupSize
		if end > len(malIDs) {
			end = len(malIDs)
		}
		batch := malIDs[i:end]

		c.throttle()

		idStrs := make([]string, len(batch))
		for j, id := range batch {
			idStrs[j] = fmt.Sprintf("%d", id)
		}

		payload := map[string]any{"ids": idStrs}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}

		u := fmt.Sprintf("%s/mal/show?apikey=%s", apiBase, url.QueryEscape(c.apiKey))

		var results []MediaInfo
		if err := c.doRequest(ctx, http.MethodPost, u, body, &results); err != nil {
			return nil, fmt.Errorf("batch lookup: %w", err)
		}

		for _, m := range results {
			if m.IDs.MAL != 0 {
				resultMap[m.IDs.MAL] = m
			}
		}
	}

	return resultMap, nil
}

// SearchResultItem holds a single result from MDBList's title search.
type SearchResultItem struct {
	Title string      `json:"title"`
	Year  int         `json:"year"`
	IDs   SearchIDs   `json:"ids"`
	Type  string      `json:"type"`
}

// SearchIDs holds provider IDs returned by the search endpoint.
type SearchIDs struct {
	IMDB  string `json:"imdbid"`
	TMDB  int    `json:"tmdbid"`
	TVDB  int    `json:"tvdbid"`
	MAL   int    `json:"malid"`
}

// searchResponse wraps the MDBList search API response.
type searchResponse struct {
	Search []SearchResultItem `json:"search"`
}

// ProviderIDsFromSearch converts a search result into a provider ID map
// suitable for AddItems, preferring IMDB over TMDB over TVDB.
func ProviderIDsFromSearch(r SearchResultItem) map[string]any {
	id := map[string]any{}
	if r.IDs.IMDB != "" {
		id["imdb"] = r.IDs.IMDB
	} else if r.IDs.TMDB != 0 {
		id["tmdb"] = r.IDs.TMDB
	} else if r.IDs.TVDB != 0 {
		id["tvdb"] = r.IDs.TVDB
	}
	return id
}

// SearchByTitle searches MDBList's database by show title.
// Returns the best matching result, or nil if nothing found.
func (c *Client) SearchByTitle(ctx context.Context, title string) (*SearchResultItem, error) {
	c.throttle()

	u := fmt.Sprintf("%s/search/show?apikey=%s&query=%s",
		apiBase, url.QueryEscape(c.apiKey), url.QueryEscape(title))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MDBList search error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var sr searchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	if len(sr.Search) == 0 {
		return nil, nil
	}

	return &sr.Search[0], nil
}

// doRequest sends an HTTP request and decodes the response.
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte, dst any) error {
	var lastErr error
	for attempt := range maxRetry {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response: %w", readErr)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			// Exponential backoff: 2s, 4s, 8s
			time.Sleep(time.Duration(1<<(attempt+1)) * time.Second)
			lastErr = fmt.Errorf("rate limited (attempt %d)", attempt+1)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("MDBList server error (HTTP %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("MDBList client error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}

		if dst != nil {
			if err := json.Unmarshal(respBody, dst); err != nil {
				return fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
			}
		}

		return nil
	}

	return fmt.Errorf("giving up after %d retries: %w", maxRetry, lastErr)
}

// doMutation sends a mutation request with retry support for 429 responses.
func (c *Client) doMutation(ctx context.Context, method, url string, body []byte, dst any) error {
	var lastErr error
	for attempt := range maxRetry {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt+1)) * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
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

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response: %w", readErr)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			// Exponential backoff: 2s, 4s, 8s
			time.Sleep(time.Duration(1<<(attempt+1)) * time.Second)
			lastErr = fmt.Errorf("rate limited (attempt %d)", attempt+1)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("MDBList server error (HTTP %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("MDBList client error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}

		if dst != nil {
			if err := json.Unmarshal(respBody, dst); err != nil {
				return fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
			}
		}

		return nil
	}

	return fmt.Errorf("giving up after %d retries: %w", maxRetry, lastErr)
}
