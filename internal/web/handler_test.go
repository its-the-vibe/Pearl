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

func TestBuildMonthLabels_OffsetAndWidth(t *testing.T) {
	// Use a fixed Sunday that is guaranteed to start in one month and span
	// into the next so we get at least two distinct month labels.
	// 2024-01-07 is a Sunday in January 2024.
	start := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)
	const numWeeks = 10
	const cellWidth = 13

	labels := buildMonthLabels(start, numWeeks)
	if len(labels) < 2 {
		t.Fatalf("expected at least 2 month labels, got %d", len(labels))
	}

	// The first label should have a non-negative offset (its absolute position).
	if labels[0].Offset < 0 {
		t.Errorf("first label Offset should be >= 0, got %d", labels[0].Offset)
	}

	// All labels after the first must have Offset == 0 because they sit
	// immediately after the previous label in the flex container.
	for i := 1; i < len(labels); i++ {
		if labels[i].Offset != 0 {
			t.Errorf("label[%d] (%s) Offset = %d, want 0", i, labels[i].Name, labels[i].Offset)
		}
	}

	// The sum of all widths plus the first label's offset should equal the
	// total grid width (numWeeks * cellWidth).
	totalWidth := labels[0].Offset
	for _, l := range labels {
		totalWidth += l.Width
	}
	if totalWidth != numWeeks*cellWidth {
		t.Errorf("total width = %d, want %d", totalWidth, numWeeks*cellWidth)
	}

	// Every individual label width must be positive.
	for i, l := range labels {
		if l.Width <= 0 {
			t.Errorf("label[%d] (%s) Width = %d, want > 0", i, l.Name, l.Width)
		}
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
