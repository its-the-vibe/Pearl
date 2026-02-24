package web

import (
	"testing"
	"time"

	bq "github.com/its-the-vibe/pearl/internal/bigquery"
)

func TestBuildHeatmapData_Empty(t *testing.T) {
	data := buildHeatmapData(nil)

	if len(data.Weeks) != 53 {
		t.Errorf("expected 53 weeks, got %d", len(data.Weeks))
	}
	for w, week := range data.Weeks {
		if len(week) != 7 {
			t.Errorf("week %d: expected 7 days, got %d", w, len(week))
		}
	}
	if data.TotalJourneys != 0 {
		t.Errorf("expected 0 total journeys, got %d", data.TotalJourneys)
	}
	if data.ActiveDays != 0 {
		t.Errorf("expected 0 active days, got %d", data.ActiveDays)
	}
	if data.BusiestDay != "–" {
		t.Errorf("expected '–' busiest day, got %q", data.BusiestDay)
	}
}

func TestBuildHeatmapData_WithCounts(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	counts := []bq.DayCount{
		{Date: yesterday, Count: 3},
		{Date: today, Count: 5},
	}

	data := buildHeatmapData(counts)

	if data.TotalJourneys != 8 {
		t.Errorf("expected 8 total journeys, got %d", data.TotalJourneys)
	}
	if data.ActiveDays != 2 {
		t.Errorf("expected 2 active days, got %d", data.ActiveDays)
	}
}

func TestIntensityLevel(t *testing.T) {
	tests := []struct {
		count, max int
		want       int
	}{
		{0, 10, 0},
		{1, 10, 1},  // ratio 0.1 → level 1
		{2, 10, 1},  // ratio 0.2 → level 1
		{3, 10, 2},  // ratio 0.3 → level 2
		{5, 10, 2},  // ratio 0.5 → level 2
		{6, 10, 3},  // ratio 0.6 → level 3
		{8, 10, 4},  // ratio 0.8 → level 4
		{10, 10, 4}, // ratio 1.0 → level 4
	}
	for _, tt := range tests {
		got := intensityLevel(tt.count, tt.max)
		if got != tt.want {
			t.Errorf("intensityLevel(%d, %d) = %d, want %d", tt.count, tt.max, got, tt.want)
		}
	}
}
