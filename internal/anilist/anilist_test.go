package anilist

import (
	"testing"
)

func makePtr[T any](v T) *T {
	return &v
}

func TestSkipByDuration_NilDuration(t *testing.T) {
	t.Parallel()

	s := Show{Duration: nil, Episodes: nil}
	if s.SkipByDuration() {
		t.Error("expected false for nil duration")
	}
}

func TestSkipByDuration_DurationBelowThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration int
		episodes *int
	}{
		{"duration 0", 0, nil},
		{"duration 1", 1, nil},
		{"duration 10", 10, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Show{Duration: &tc.duration, Episodes: tc.episodes}
			if !s.SkipByDuration() {
				t.Errorf("expected true for duration=%d", tc.duration)
			}
		})
	}
}

func TestSkipByDuration_DurationAboveThreshold(t *testing.T) {
	t.Parallel()

	s := Show{Duration: makePtr(24), Episodes: nil}
	if s.SkipByDuration() {
		t.Error("expected false for duration 24")
	}
}

func TestSkipByDuration_TotalRuntimeBelowThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration int
		episodes int
	}{
		{"6min x 1ep", 6, 1},
		{"3min x 2ep", 3, 2},
		{"10min x 1ep", 10, 1},
		{"6min x 2ep", 6, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Show{Duration: &tc.duration, Episodes: &tc.episodes}
			if !s.SkipByDuration() {
				t.Errorf("expected true for %s", tc.name)
			}
		})
	}
}

func TestSkipByDuration_TotalRuntimeAboveThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration int
		episodes int
	}{
		{"11min x 1ep", 11, 1},
		{"24min x 12ep", 24, 12},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Show{Duration: &tc.duration, Episodes: &tc.episodes}
			if s.SkipByDuration() {
				t.Errorf("expected false for %s", tc.name)
			}
		})
	}
}

func TestSkipByDuration_NilEpisodes(t *testing.T) {
	t.Parallel()

	// Duration > 10 but Episodes nil — should not skip
	s := Show{Duration: makePtr(24), Episodes: nil}
	if s.SkipByDuration() {
		t.Error("expected false when duration > 10 and episodes nil")
	}
}

func TestHasTag_CaseInsensitiveMatch(t *testing.T) {
	t.Parallel()

	s := Show{Tags: []Tag{{Name: "Action"}, {Name: "Sci-Fi"}}}

	if !s.HasTag("action") {
		t.Error("expected true for 'action'")
	}
	if !s.HasTag("ACTION") {
		t.Error("expected true for 'ACTION'")
	}
	if !s.HasTag("sci-fi") {
		t.Error("expected true for 'sci-fi'")
	}
	if !s.HasTag("SCI-FI") {
		t.Error("expected true for 'SCI-FI'")
	}
}

func TestHasTag_NoMatch(t *testing.T) {
	t.Parallel()

	s := Show{Tags: []Tag{{Name: "Action"}, {Name: "Comedy"}}}
	if s.HasTag("Horror") {
		t.Error("expected false for 'Horror'")
	}
}

func TestHasTag_EmptyTags(t *testing.T) {
	t.Parallel()

	s := Show{Tags: []Tag{}}
	if s.HasTag("Action") {
		t.Error("expected false for empty tags")
	}
}

func TestHasTag_NilTags(t *testing.T) {
	t.Parallel()

	s := Show{Tags: nil}
	if s.HasTag("Action") {
		t.Error("expected false for nil tags")
	}
}

func TestDisplayTitle_EnglishAvailable(t *testing.T) {
	t.Parallel()

	english := "Attack on Titan"
	romaji := "Shingeki no Kyojin"
	s := Show{
		Title: Title{
			English: &english,
			Romaji:  &romaji,
		},
	}
	if got := s.DisplayTitle(); got != english {
		t.Errorf("expected %q, got %q", english, got)
	}
}

func TestDisplayTitle_RomajiFallback(t *testing.T) {
	t.Parallel()

	romaji := "Shingeki no Kyojin"
	s := Show{
		Title: Title{
			English: nil,
			Romaji:  &romaji,
		},
	}
	if got := s.DisplayTitle(); got != romaji {
		t.Errorf("expected %q, got %q", romaji, got)
	}
}

func TestDisplayTitle_EnglishEmpty(t *testing.T) {
	t.Parallel()

	english := ""
	romaji := "Shingeki no Kyojin"
	s := Show{
		Title: Title{
			English: &english,
			Romaji:  &romaji,
		},
	}
	if got := s.DisplayTitle(); got != romaji {
		t.Errorf("expected romaji fallback %q, got %q", romaji, got)
	}
}

func TestDisplayTitle_BothNil(t *testing.T) {
	t.Parallel()

	s := Show{
		ID: 42,
		Title: Title{
			English: nil,
			Romaji:  nil,
		},
	}
	want := "Anime #42"
	if got := s.DisplayTitle(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRelationMALIDsByType_MatchingTypes(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: makePtr(16498)}, RelationType: "PREQUEL"},
				{Node: RelationNode{IDMal: makePtr(30230)}, RelationType: "PREQUEL"},
				{Node: RelationNode{IDMal: makePtr(40028)}, RelationType: "ADAPTATION"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{"PREQUEL"})
	if len(ids) != 2 {
		t.Fatalf("expected 2 PREQUEL IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != 16498 || ids[1] != 30230 {
		t.Errorf("expected [16498 30230], got %v", ids)
	}
}

func TestRelationMALIDsByType_NonMatchingTypes(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: makePtr(16498)}, RelationType: "PREQUEL"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{"ADAPTATION", "SIDE_STORY"})
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs for non-matching types, got %v", ids)
	}
}

func TestRelationMALIDsByType_EmptyTypes(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: makePtr(16498)}, RelationType: "PREQUEL"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{})
	if ids != nil {
		t.Errorf("expected nil for empty types, got %v", ids)
	}
}

func TestRelationMALIDsByType_NilRelations(t *testing.T) {
	t.Parallel()

	s := Show{Relations: nil}
	ids := s.RelationMALIDsByType([]string{"PREQUEL"})
	if ids != nil {
		t.Errorf("expected nil for nil relations, got %v", ids)
	}
}

func TestRelationMALIDsByType_NilIDMal(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: nil}, RelationType: "PREQUEL"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{"PREQUEL"})
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs for nil IDMal, got %v", ids)
	}
}

func TestRelationMALIDsByType_ZeroIDMal(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: makePtr(0)}, RelationType: "PREQUEL"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{"PREQUEL"})
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs for zero IDMal, got %v", ids)
	}
}

func TestRelationMALIDsByType_MultipleTypes(t *testing.T) {
	t.Parallel()

	s := Show{
		Relations: &RelationBlock{
			Edges: []RelationEdge{
				{Node: RelationNode{IDMal: makePtr(16498)}, RelationType: "PREQUEL"},
				{Node: RelationNode{IDMal: makePtr(30230)}, RelationType: "ADAPTATION"},
				{Node: RelationNode{IDMal: makePtr(40028)}, RelationType: "SIDE_STORY"},
			},
		},
	}
	ids := s.RelationMALIDsByType([]string{"PREQUEL", "SIDE_STORY"})
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	// order must match edges
	if ids[0] != 16498 || ids[1] != 40028 {
		t.Errorf("expected [16498 40028], got %v", ids)
	}
}

func TestTitleStruct(t *testing.T) {
	t.Parallel()

	eng := "English Title"
	rom := "Romaji Title"
	title := Title{English: &eng, Romaji: &rom}

	if *title.English != "English Title" {
		t.Errorf("English = %q, want %q", *title.English, "English Title")
	}
	if *title.Romaji != "Romaji Title" {
		t.Errorf("Romaji = %q, want %q", *title.Romaji, "Romaji Title")
	}
}
