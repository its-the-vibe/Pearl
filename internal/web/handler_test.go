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

// ---- Commute tests ----

func TestParseTimeToMinutes(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"07:00", 7 * 60, false},
		{"7:00", 7 * 60, false},
		{"10:30", 10*60 + 30, false},
		{"09:45", 9*60 + 45, false},
		{"00:00", 0, false},
		{"23:59", 23*60 + 59, false},
		{"invalid", 0, true},
		{"25:00", 0, true},
		{"10:60", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTimeToMinutes(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTimeToMinutes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseTimeToMinutes(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "0m"},
		{-5, "0m"},
		{30, "30m"},
		{60, "1h"},
		{90, "1h 30m"},
		{125, "2h 5m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.minutes)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}

func TestBuildCommuteData_Empty(t *testing.T) {
	data := buildCommuteData(nil, nil)

	if data.TotalCommutes != 0 {
		t.Errorf("expected 0 commutes, got %d", data.TotalCommutes)
	}
	if data.AvgDuration != "–" {
		t.Errorf("expected '–' avg duration, got %q", data.AvgDuration)
	}
	if data.LongestCommute != "–" {
		t.Errorf("expected '–' longest commute, got %q", data.LongestCommute)
	}
	if len(data.TimeLabels) == 0 {
		t.Error("expected time labels even with empty input")
	}
}

// findTuesdayWednesdayThursday returns a Tuesday, Wednesday, and Thursday
// relative to a reference week beginning on Monday 2024-01-01.
func commuteWeekDates() (tue, wed, thu time.Time) {
	// 2024-01-01 is a Monday.
	mon := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return mon.AddDate(0, 0, 1), mon.AddDate(0, 0, 2), mon.AddDate(0, 0, 3)
}

func TestBuildCommuteData_FiltersDaysOfWeek(t *testing.T) {
	tue, wed, thu := commuteWeekDates()
	// 2024-01-01 (Monday) and 2024-01-06 (Saturday) should be excluded.
	mon := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sat := mon.AddDate(0, 0, 5)

	journeys := []bq.CommuteJourney{
		{Date: mon.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
		{Date: tue.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
		{Date: wed.Format("2006-01-02"), StartTime: "08:30", EndTime: "09:30"},
		{Date: thu.Format("2006-01-02"), StartTime: "07:15", EndTime: "08:45"},
		{Date: sat.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
	}

	data := buildCommuteData(journeys, nil)
	if data.TotalCommutes != 3 {
		t.Errorf("expected 3 commutes (Tue/Wed/Thu only), got %d", data.TotalCommutes)
	}
}

func TestBuildCommuteData_FiltersTimeWindow(t *testing.T) {
	_, wed, _ := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		// Too early (before 07:00).
		{Date: wed.Format("2006-01-02"), StartTime: "06:50", EndTime: "07:45"},
		// Exactly on the lower boundary – included.
		{Date: wed.Format("2006-01-02"), StartTime: "07:00", EndTime: "08:00"},
		// Mid-window – included.
		{Date: wed.Format("2006-01-02"), StartTime: "09:00", EndTime: "10:00"},
		// Exactly on the upper boundary – included.
		{Date: wed.Format("2006-01-02"), StartTime: "10:30", EndTime: "11:00"},
		// After window (start after 10:30) – excluded.
		{Date: wed.Format("2006-01-02"), StartTime: "10:31", EndTime: "11:30"},
	}

	data := buildCommuteData(journeys, nil)
	if data.TotalCommutes != 3 {
		t.Errorf("expected 3 commutes within time window, got %d", data.TotalCommutes)
	}
}

func TestBuildCommuteData_ComputesDuration(t *testing.T) {
	_, _, thu := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		// 45-minute commute.
		{Date: thu.Format("2006-01-02"), StartTime: "08:00", EndTime: "08:45"},
		// 90-minute commute.
		{Date: thu.Format("2006-01-02"), StartTime: "07:30", EndTime: "09:00"},
	}

	data := buildCommuteData(journeys, nil)
	if data.TotalCommutes != 2 {
		t.Fatalf("expected 2 commutes, got %d", data.TotalCommutes)
	}

	// Average = (45 + 90) / 2 = 67m → "1h 7m"
	if data.AvgDuration != "1h 7m" {
		t.Errorf("AvgDuration = %q, want %q", data.AvgDuration, "1h 7m")
	}
	if data.LongestCommute != "1h 30m" {
		t.Errorf("LongestCommute = %q, want %q", data.LongestCommute, "1h 30m")
	}
}

func TestBuildCommuteData_SVGGeometry(t *testing.T) {
	tue, _, _ := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		{Date: tue.Format("2006-01-02"), StartTime: "07:00", EndTime: "10:30"},
	}

	data := buildCommuteData(journeys, nil)
	if data.TotalCommutes != 1 {
		t.Fatalf("expected 1 commute, got %d", data.TotalCommutes)
	}

	p := data.Commutes[0]

	// BarY must be at the top of the plot area (7:00 → svgPaddingTop).
	if p.BarY != svgPaddingTop {
		t.Errorf("BarY = %d, want %d (svgPaddingTop)", p.BarY, svgPaddingTop)
	}

	// BarBottomY must be at the bottom of the plot area (10:30 → svgPaddingTop + svgPlotHeight).
	wantBottom := svgPaddingTop + svgPlotHeight
	if p.BarBottomY != wantBottom {
		t.Errorf("BarBottomY = %d, want %d", p.BarBottomY, wantBottom)
	}

	// BarHeight = BarBottomY - BarY.
	if p.BarHeight != p.BarBottomY-p.BarY {
		t.Errorf("BarHeight %d != BarBottomY(%d) - BarY(%d)", p.BarHeight, p.BarBottomY, p.BarY)
	}
}

func TestBuildCommuteData_SkipsInvalidEndTime(t *testing.T) {
	_, wed, _ := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		// Valid commute.
		{Date: wed.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
		// End time before start time – should be skipped.
		{Date: wed.Format("2006-01-02"), StartTime: "08:30", EndTime: "08:00"},
		// Empty end time – should be skipped.
		{Date: wed.Format("2006-01-02"), StartTime: "09:00", EndTime: ""},
	}

	data := buildCommuteData(journeys, nil)
	if data.TotalCommutes != 1 {
		t.Errorf("expected 1 commute (skipping invalid end times), got %d", data.TotalCommutes)
	}
}

// ---- Rating overlay tests ----

func TestRatingToSVGY(t *testing.T) {
	// Rating 5 maps to the top of the plot area.
	if got := ratingToSVGY(5); got != svgPaddingTop {
		t.Errorf("ratingToSVGY(5) = %d, want %d (svgPaddingTop)", got, svgPaddingTop)
	}
	// Rating 1 maps to the bottom of the plot area.
	if got := ratingToSVGY(1); got != svgPaddingTop+svgPlotHeight {
		t.Errorf("ratingToSVGY(1) = %d, want %d", got, svgPaddingTop+svgPlotHeight)
	}
	// Rating 3 maps to the midpoint.
	want := svgPaddingTop + svgPlotHeight/2
	if got := ratingToSVGY(3); got != want {
		t.Errorf("ratingToSVGY(3) = %d, want %d (midpoint)", got, want)
	}
}

func TestBuildCommuteData_RatingsOverlay(t *testing.T) {
	tue, wed, thu := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		{Date: tue.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
		{Date: wed.Format("2006-01-02"), StartTime: "08:30", EndTime: "09:30"},
		{Date: thu.Format("2006-01-02"), StartTime: "07:15", EndTime: "08:45"},
	}

	ratings := []bq.DailyRating{
		{Date: tue, Rating: 5},
		{Date: wed, Rating: 3},
		// No rating for Thursday – that commute gets no overlay point.
	}

	data := buildCommuteData(journeys, ratings)

	if data.TotalCommutes != 3 {
		t.Fatalf("expected 3 commutes, got %d", data.TotalCommutes)
	}
	if !data.HasRatings {
		t.Error("HasRatings should be true")
	}
	if len(data.Ratings) != 2 {
		t.Fatalf("expected 2 rating points, got %d", len(data.Ratings))
	}

	// Tuesday rating (5) should map to the top of the plot.
	if data.Ratings[0].Rating != 5 {
		t.Errorf("first rating = %v, want 5", data.Ratings[0].Rating)
	}
	if data.Ratings[0].Y != svgPaddingTop {
		t.Errorf("rating=5 Y = %d, want %d (top of plot)", data.Ratings[0].Y, svgPaddingTop)
	}
	// X must match the Tuesday commute bar.
	if data.Ratings[0].X != data.Commutes[0].X {
		t.Errorf("rating X = %d, want %d (Tuesday commute X)", data.Ratings[0].X, data.Commutes[0].X)
	}

	// Wednesday rating (3) should map to the midpoint of the plot.
	wantMid := svgPaddingTop + svgPlotHeight/2
	if data.Ratings[1].Y != wantMid {
		t.Errorf("rating=3 Y = %d, want %d (midpoint)", data.Ratings[1].Y, wantMid)
	}
}

func TestBuildCommuteData_NoRatingsWhenNil(t *testing.T) {
	_, wed, _ := commuteWeekDates()

	journeys := []bq.CommuteJourney{
		{Date: wed.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
	}

	data := buildCommuteData(journeys, nil)

	if data.HasRatings {
		t.Error("HasRatings should be false when no ratings provided")
	}
	if len(data.Ratings) != 0 {
		t.Errorf("expected 0 rating points, got %d", len(data.Ratings))
	}
	// RatingLabels should still be populated for template rendering.
	if len(data.RatingLabels) != 5 {
		t.Errorf("expected 5 rating labels, got %d", len(data.RatingLabels))
	}
}

func TestBuildCommuteData_RatingExpandsXAxis(t *testing.T) {
	tue, wed, _ := commuteWeekDates()
	// Friday is not a Tue/Wed/Thu commute day – it has no journey but will have a rating.
	fri := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)

	journeys := []bq.CommuteJourney{
		{Date: tue.Format("2006-01-02"), StartTime: "08:00", EndTime: "09:00"},
		{Date: wed.Format("2006-01-02"), StartTime: "08:30", EndTime: "09:30"},
	}

	ratings := []bq.DailyRating{
		{Date: tue, Rating: 5},
		// Rating for a date with no corresponding journey – should expand x-axis.
		{Date: fri, Rating: 3},
	}

	data := buildCommuteData(journeys, ratings)

	// Two journeys should be recorded.
	if data.TotalCommutes != 2 {
		t.Fatalf("expected 2 commutes, got %d", data.TotalCommutes)
	}

	// There should be two rating points: one for Tuesday (has journey) and one for Friday (no journey).
	if len(data.Ratings) != 2 {
		t.Fatalf("expected 2 rating points, got %d", len(data.Ratings))
	}

	// Tuesday rating has a corresponding journey bar.
	if !data.Ratings[0].HasJourney {
		t.Error("Tuesday rating should have HasJourney = true")
	}

	// Friday rating has no corresponding journey bar.
	if data.Ratings[1].HasJourney {
		t.Error("Friday rating should have HasJourney = false")
	}

	// The SVG width must accommodate three dates (Tue, Wed, Fri) – wider than just 2 journey bars.
	expectedMinWidth := svgPaddingLeft + 3*svgBarStep + svgPaddingRight
	if data.SVGWidth < expectedMinWidth {
		t.Errorf("SVGWidth = %d, want at least %d to include rating-only date", data.SVGWidth, expectedMinWidth)
	}

	// The Friday rating x-coordinate must be to the right of the Wednesday journey bar.
	wedX := data.Commutes[1].X
	friX := data.Ratings[1].X
	if friX <= wedX {
		t.Errorf("Friday rating X (%d) should be to the right of Wednesday journey X (%d)", friX, wedX)
	}

	// The Tuesday rating must share its X with the Tuesday journey bar.
	tueJourneyX := data.Commutes[0].X
	tueRatingX := data.Ratings[0].X
	if tueRatingX != tueJourneyX {
		t.Errorf("Tuesday rating X (%d) should match Tuesday journey X (%d)", tueRatingX, tueJourneyX)
	}
}

func TestBuildCommuteData_RatingLabels(t *testing.T) {
	data := buildCommuteData(nil, nil)

	if len(data.RatingLabels) != 5 {
		t.Fatalf("expected 5 rating labels, got %d", len(data.RatingLabels))
	}
	// Labels should be ordered 5 → 1 (top to bottom).
	for i, want := range []int{5, 4, 3, 2, 1} {
		if data.RatingLabels[i].Value != want {
			t.Errorf("RatingLabels[%d].Value = %d, want %d", i, data.RatingLabels[i].Value, want)
		}
	}
	// Labels are ordered 5 → 1 (top to bottom), so Y coordinates must be
	// strictly increasing: rating 5 sits at the top (smallest Y) and
	// rating 1 sits at the bottom (largest Y).
	for i := 1; i < len(data.RatingLabels); i++ {
		if data.RatingLabels[i].Y <= data.RatingLabels[i-1].Y {
			t.Errorf("RatingLabels[%d].Y (%d) should be greater than RatingLabels[%d].Y (%d)",
				i, data.RatingLabels[i].Y, i-1, data.RatingLabels[i-1].Y)
		}
	}
}
