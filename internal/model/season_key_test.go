package model

import (
	"testing"
)

func TestSeasonKey_String(t *testing.T) {
	tests := []struct {
		key  SeasonKey
		want string
	}{
		{SeasonKey{"WINTER", 2025}, "WINTER-2025"},
		{SeasonKey{"SPRING", 2026}, "SPRING-2026"},
		{SeasonKey{"SUMMER", 2010}, "SUMMER-2010"},
		{SeasonKey{"FALL", 2027}, "FALL-2027"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.key.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseSeasonKey(t *testing.T) {
	tests := []struct {
		input   string
		wantOK  bool
		wantKey SeasonKey
	}{
		{"WINTER-2025", true, SeasonKey{"WINTER", 2025}},
		{"SPRING-2026", true, SeasonKey{"SPRING", 2026}},
		{"SUMMER-2010", true, SeasonKey{"SUMMER", 2010}},
		{"FALL-2027", true, SeasonKey{"FALL", 2027}},
		{"bad", false, SeasonKey{}},
		{"WINTER-", false, SeasonKey{}},
		{"-2025", false, SeasonKey{}},
		{"", false, SeasonKey{}},
		{"WINTER-2025-extra", false, SeasonKey{}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseSeasonKey(tc.input)
			if tc.wantOK {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tc.wantKey {
					t.Errorf("ParseSeasonKey(%q) = %v, want %v", tc.input, got, tc.wantKey)
				}
			} else {
				if err == nil {
					t.Errorf("expected error for %q, got %v", tc.input, got)
				}
			}
		})
	}
}

func TestSeasonKey_Roundtrip(t *testing.T) {
	keys := []SeasonKey{
		{"WINTER", 2025},
		{"SPRING", 2026},
		{"SUMMER", 2010},
		{"FALL", 2027},
	}

	for _, k := range keys {
		t.Run(k.String(), func(t *testing.T) {
			parsed, err := ParseSeasonKey(k.String())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed != k {
				t.Errorf("roundtrip failed: %v != %v", parsed, k)
			}
		})
	}
}
