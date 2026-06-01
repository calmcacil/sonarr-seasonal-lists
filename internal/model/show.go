package model

import (
	"fmt"
	"strings"
	"time"
)

type Tag struct {
	Name string `json:"name"`
}

type RelationEdge struct {
	Node         RelationNode `json:"node"`
	RelationType string       `json:"relationType"`
}

type RelationNode struct {
	ID    int   `json:"id"`
	IDMal *int  `json:"idMal"`
	Title Title `json:"title"`
}

type RelationBlock struct {
	Edges []RelationEdge `json:"edges"`
}

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
	Season    *string        `json:"season"`
	StartDate FuzzyDate      `json:"startDate"`
	Relations *RelationBlock `json:"relations,omitempty"`
}

func (s Show) SeasonCode() string {
	if s.Season == nil {
		return "UNKNOWN"
	}
	return strings.ToUpper(*s.Season)
}

func (s Show) IsDecemberStart() bool {
	return s.StartDate.Month != nil && *s.StartDate.Month == 12
}

func (s Show) IsSeries() bool {
	return s.Format == "TV" || s.Format == "ONA"
}

func (s Show) IsNew() bool {
	if s.Relations == nil {
		return true
	}
	for _, e := range s.Relations.Edges {
		if e.RelationType == "PREQUEL" || e.RelationType == "PARENT" {
			return false
		}
	}
	return true
}

func (s Show) SkipByDuration() bool {
	return s.Duration != nil && *s.Duration <= 10
}

func (s Show) HasTag(name string) bool {
	lower := strings.ToLower(name)
	for _, t := range s.Tags {
		if strings.ToLower(t.Name) == lower {
			return true
		}
	}
	return false
}

type FuzzyDate struct {
	Year  *int `json:"year"`
	Month *int `json:"month"`
	Day   *int `json:"day"`
}

type Title struct {
	English *string `json:"english"`
	Romaji  *string `json:"romaji"`
}

func (s Show) IsWithinMonths(months int) bool {
	if s.StartDate.Year == nil || s.StartDate.Month == nil {
		return true
	}
	start := time.Date(*s.StartDate.Year, time.Month(*s.StartDate.Month), 1, 0, 0, 0, 0, time.UTC)
	return !start.After(time.Now().AddDate(0, months, 0))
}

func (s Show) IsWinterStart() bool {
	if s.StartDate.Month == nil {
		return true
	}
	m := *s.StartDate.Month
	return m == 12 || m == 1 || m == 2 || m == 3
}

func (s Show) DisplayTitle() string {
	if s.Title.English != nil && *s.Title.English != "" {
		return *s.Title.English
	}
	if s.Title.Romaji != nil {
		return *s.Title.Romaji
	}
	return fmt.Sprintf("Anime #%d", s.ID)
}
