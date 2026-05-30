package mdblist

import (
	"testing"
)

func TestListGetURL_URLSet(t *testing.T) {
	t.Parallel()

	l := List{URL: "https://mdblist.com/lists/user/custom-list"}
	if got := l.GetURL(); got != "https://mdblist.com/lists/user/custom-list" {
		t.Errorf("expected URL directly, got %q", got)
	}
}

func TestListGetURL_UserNameAndSlug(t *testing.T) {
	t.Parallel()

	l := List{UserName: "testuser", Slug: "my-list"}
	want := "https://mdblist.com/lists/testuser/my-list"
	if got := l.GetURL(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestListGetURL_NoURLNoSlug(t *testing.T) {
	t.Parallel()

	l := List{UserName: "testuser", Slug: ""}
	if got := l.GetURL(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestListGetURL_NoURLNoUserName(t *testing.T) {
	t.Parallel()

	l := List{UserName: "", Slug: "my-list"}
	if got := l.GetURL(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestListGetURL_NeitherSet(t *testing.T) {
	t.Parallel()

	var l List
	if got := l.GetURL(); got != "" {
		t.Errorf("expected empty string for zero-value List, got %q", got)
	}
}

func TestMediaIDsStruct(t *testing.T) {
	t.Parallel()

	ids := MediaIDs{
		IMDB: "tt0903747",
		TMDB: 1396,
		TVDB: 12345,
		MAL:  16498,
	}
	if ids.IMDB != "tt0903747" {
		t.Errorf("IMDB = %q, want %q", ids.IMDB, "tt0903747")
	}
	if ids.TMDB != 1396 {
		t.Errorf("TMDB = %d, want %d", ids.TMDB, 1396)
	}
	if ids.TVDB != 12345 {
		t.Errorf("TVDB = %d, want %d", ids.TVDB, 12345)
	}
	if ids.MAL != 16498 {
		t.Errorf("MAL = %d, want %d", ids.MAL, 16498)
	}
}

func TestMediaIDs_ZeroValues(t *testing.T) {
	t.Parallel()

	var ids MediaIDs
	if ids.IMDB != "" {
		t.Errorf("expected empty IMDB, got %q", ids.IMDB)
	}
	if ids.TMDB != 0 {
		t.Errorf("expected 0 TMDB, got %d", ids.TMDB)
	}
	if ids.TVDB != 0 {
		t.Errorf("expected 0 TVDB, got %d", ids.TVDB)
	}
	if ids.MAL != 0 {
		t.Errorf("expected 0 MAL, got %d", ids.MAL)
	}
}

func TestMediaInfoStruct(t *testing.T) {
	t.Parallel()

	info := MediaInfo{
		ID:    123,
		Title: "Breaking Bad",
		Year:  2008,
		Runtime: 45,
		IDs: MediaIDs{
			IMDB: "tt0903747",
			TMDB: 1396,
		},
		Type: "show",
	}
	if info.ID != 123 {
		t.Errorf("ID = %d, want %d", info.ID, 123)
	}
	if info.Title != "Breaking Bad" {
		t.Errorf("Title = %q, want %q", info.Title, "Breaking Bad")
	}
	if info.Year != 2008 {
		t.Errorf("Year = %d, want %d", info.Year, 2008)
	}
	if info.Runtime != 45 {
		t.Errorf("Runtime = %d, want %d", info.Runtime, 45)
	}
	if info.IDs.IMDB != "tt0903747" {
		t.Errorf("IDs.IMDB = %q, want %q", info.IDs.IMDB, "tt0903747")
	}
	if info.Type != "show" {
		t.Errorf("Type = %q, want %q", info.Type, "show")
	}
}

func TestMediaInfo_ZeroValues(t *testing.T) {
	t.Parallel()

	var info MediaInfo
	if info.ID != 0 {
		t.Errorf("expected 0 ID, got %d", info.ID)
	}
	if info.Title != "" {
		t.Errorf("expected empty Title, got %q", info.Title)
	}
	if info.Runtime != 0 {
		t.Errorf("expected 0 Runtime, got %d", info.Runtime)
	}
}
