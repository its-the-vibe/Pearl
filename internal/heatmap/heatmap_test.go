package heatmap_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/its-the-vibe/pearl/internal/heatmap"
)

func TestRender_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := heatmap.Render(&buf, map[string]int{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No journey data found") {
		t.Errorf("expected empty-data message, got: %q", buf.String())
	}
}

func TestRender_WithData(t *testing.T) {
	activity := map[string]int{
		"2024-01-15": 3,
		"2024-01-16": 5,
		"2024-01-22": 1,
	}

	var buf bytes.Buffer
	if err := heatmap.Render(&buf, activity); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	// Should contain day labels.
	for _, day := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		if !strings.Contains(out, day) {
			t.Errorf("expected day label %q in output", day)
		}
	}

	// Should contain the legend.
	if !strings.Contains(out, "Legend") {
		t.Errorf("expected Legend in output")
	}

	// Should contain total journeys summary.
	if !strings.Contains(out, "Total journeys: 9") {
		t.Errorf("expected total journeys summary in output, got:\n%s", out)
	}
}

func TestRender_InvalidDate(t *testing.T) {
	activity := map[string]int{
		"not-a-date": 1,
	}
	var buf bytes.Buffer
	err := heatmap.Render(&buf, activity)
	if err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}
