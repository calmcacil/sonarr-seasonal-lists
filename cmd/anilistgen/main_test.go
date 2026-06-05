package main

import (
	"testing"

	"github.com/calmcacil/anilistgen/internal/model"
	"github.com/calmcacil/anilistgen/internal/output"
)

func TestPrintDryRun(t *testing.T) {
	winter2026 := model.SeasonKey{Season: "WINTER", Year: 2026}
	data := map[model.SeasonKey][]output.Show{
		winter2026: {{TVDBID: 12345, Title: "Test Show"}},
	}
	printDryRun(data, "series")
}

func makePtr[T any](v T) *T {
	return &v
}
