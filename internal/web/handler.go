package web

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	bq "github.com/its-the-vibe/pearl/internal/bigquery"
)

//go:embed templates/*.html
var templateFS embed.FS

// Cell represents a single day cell in the heatmap grid.
type Cell struct {
	Empty bool
	Level int
	Label string
}

// MonthLabel positions a month name above the heatmap columns.
type MonthLabel struct {
	Name   string
	Offset int
	Width  int
}

// HeatmapData is passed to the heatmap template.
type HeatmapData struct {
	Weeks         [][]Cell
	MonthLabels   []MonthLabel
	TotalJourneys int
	ActiveDays    int
	BusiestDay    string
}

// TimeLabel positions a time label on the Y-axis of the commute chart.
type TimeLabel struct {
	Y     int
	Label string
}

// CommutePoint represents a single commute journey rendered in the SVG chart.
type CommutePoint struct {
	Date        string // e.g. "Tue 05 Mar" – used as x-axis label
	ISODate     string // e.g. "2024-03-05" – used to match with ratings
	Start       string // e.g. "07:45" – used in tooltip
	End         string // e.g. "09:15" – used in tooltip
	Duration    string // e.g. "1h 30m" – used in tooltip
	X           int    // center x of bar in SVG
	BarX        int    // left edge of bar rect
	BarY        int    // top y of bar rect (= start time position)
	BarHeight   int    // height of bar rect
	BarBottomY  int    // BarY + BarHeight (= end time position)
}

// RatingPoint represents a single daily rating overlaid on the commute chart.
type RatingPoint struct {
	X      int     // x-coordinate aligned with the corresponding commute bar
	Y      int     // y-coordinate on the right (ratings) Y-axis
	Rating float64 // value between 1 and 5
	Date   string  // e.g. "Tue 05 Mar" – used in tooltip
}

// RatingLabel positions a label on the right (ratings) Y-axis.
type RatingLabel struct {
	Y     int
	Value int
}

// CommuteData is passed to the commutes template.
type CommuteData struct {
	Commutes       []CommutePoint
	TimeLabels     []TimeLabel
	TotalCommutes  int
	AvgDuration    string
	LongestCommute string
	SVGWidth       int
	SVGHeight      int
	ChartLeft      int // x of y-axis / left edge of plot area
	ChartRight     int // x of right axis / right edge of plot area
	ChartTop       int // y of top of plot area
	ChartBottom    int // y of bottom of plot area (x-axis line)
	LabelY         int // y for x-axis text labels
	Ratings        []RatingPoint
	RatingLabels   []RatingLabel
	HasRatings     bool
}

// Handler holds the dependencies for HTTP handlers.
type Handler struct {
	bqClient *bq.Client
	tmpl     *template.Template
}

// NewHandler creates a Handler with the given BigQuery client.
func NewHandler(client *bq.Client) (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Handler{bqClient: client, tmpl: tmpl}, nil
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleHeatmap)
	mux.HandleFunc("/commutes", h.handleCommutes)
	mux.HandleFunc("/health", h.handleHealth)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (h *Handler) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	counts, err := h.bqClient.JourneyCountsByDay(r.Context())
	if err != nil {
		slog.Error("querying bigquery", "error", err)
		http.Error(w, "failed to load journey data", http.StatusInternalServerError)
		return
	}

	data := buildHeatmapData(counts)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "heatmap.html", data); err != nil {
		slog.Error("rendering template", "error", err)
	}
}

func (h *Handler) handleCommutes(w http.ResponseWriter, r *http.Request) {
	journeys, err := h.bqClient.CommuteJourneys(r.Context())
	if err != nil {
		slog.Error("querying bigquery for commutes", "error", err)
		http.Error(w, "failed to load commute data", http.StatusInternalServerError)
		return
	}

	ratings, err := h.bqClient.Ratings(r.Context())
	if err != nil {
		// Ratings are optional; log and continue without the overlay.
		slog.Warn("querying bigquery for ratings", "error", err)
	}

	data := buildCommuteData(journeys, ratings)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, "commutes.html", data); err != nil {
		slog.Error("rendering commutes template", "error", err)
	}
}

// buildHeatmapData converts raw day counts into a grid suitable for the heatmap template.
// It renders the last 52 weeks (364 days), anchored to the most recent Sunday.
func buildHeatmapData(counts []bq.DayCount) HeatmapData {
	// Build a lookup map of date string → count.
	lookup := make(map[string]int, len(counts))
	for _, dc := range counts {
		lookup[dc.Date.Format("2006-01-02")] = dc.Count
	}

	// Determine max count for intensity scaling.
	maxCount := 0
	for _, c := range counts {
		if c.Count > maxCount {
			maxCount = c.Count
		}
	}

	// Anchor to the Sunday on or before today, and go back 52 weeks.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	// Find the most recent Sunday (weekday 0).
	offset := int(today.Weekday())
	endSunday := today.AddDate(0, 0, -offset)
	startSunday := endSunday.AddDate(0, 0, -52*7+7)

	// Build the grid: 53 weeks × 7 days.
	// Week columns go left→right (Sunday…Saturday).
	numWeeks := 53
	weeks := make([][]Cell, numWeeks)
	for w := range weeks {
		weeks[w] = make([]Cell, 7)
		for d := range weeks[w] {
			day := startSunday.AddDate(0, 0, w*7+d)
			if day.After(today) {
				weeks[w][d] = Cell{Empty: true}
				continue
			}
			key := day.Format("2006-01-02")
			count := lookup[key]
			weeks[w][d] = Cell{
				Level: intensityLevel(count, maxCount),
				Label: fmt.Sprintf("%s: %d journeys", day.Format("02 Jan 2006"), count),
			}
		}
	}

	// Build month labels.
	monthLabels := buildMonthLabels(startSunday, numWeeks)

	// Compute stats over the last year.
	var totalJourneys, activeDays int
	var busiestDate string
	busiestCount := 0
	oneYearAgo := today.AddDate(-1, 0, 0)
	for _, dc := range counts {
		if dc.Date.Before(oneYearAgo) {
			continue
		}
		totalJourneys += dc.Count
		activeDays++
		if dc.Count > busiestCount {
			busiestCount = dc.Count
			busiestDate = dc.Date.Format("02 Jan 2006")
		}
	}
	if busiestDate == "" {
		busiestDate = "–"
	}

	return HeatmapData{
		Weeks:         weeks,
		MonthLabels:   monthLabels,
		TotalJourneys: totalJourneys,
		ActiveDays:    activeDays,
		BusiestDay:    busiestDate,
	}
}

// intensityLevel returns a 0–4 level for the given count and maximum.
func intensityLevel(count, max int) int {
	if count == 0 || max == 0 {
		return 0
	}
	ratio := float64(count) / float64(max)
	switch {
	case ratio <= 0.25:
		return 1
	case ratio <= 0.50:
		return 2
	case ratio <= 0.75:
		return 3
	default:
		return 4
	}
}

// buildMonthLabels returns labels positioned above the week columns.
func buildMonthLabels(startSunday time.Time, numWeeks int) []MonthLabel {
	const cellWidth = 13 // 11px cell + 2px gap

	var labels []MonthLabel
	prevMonth := -1
	var prevAbsOffset int
	for w := 0; w < numWeeks; w++ {
		day := startSunday.AddDate(0, 0, w*7)
		m := int(day.Month())
		if m != prevMonth {
			absOffset := w * cellWidth
			// Offset is used as CSS margin-left inside a flex container, so only
			// the first label needs an absolute offset; subsequent labels sit
			// immediately after the previous one and need no extra margin.
			offset := 0
			if len(labels) == 0 {
				offset = absOffset
			}
			labels = append(labels, MonthLabel{
				Name:   day.Format("Jan"),
				Offset: offset,
			})
			if len(labels) > 1 {
				prev := &labels[len(labels)-2]
				prev.Width = absOffset - prevAbsOffset
			}
			prevAbsOffset = absOffset
			prevMonth = m
		}
	}
	if len(labels) > 0 {
		last := &labels[len(labels)-1]
		last.Width = numWeeks*cellWidth - prevAbsOffset
	}
	return labels
}

// SVG chart layout constants for the commute chart.
const (
	svgPaddingLeft   = 60
	svgPaddingRight  = 30
	svgPaddingTop    = 20
	svgPaddingBottom = 60
	svgPlotHeight    = 250
	svgBarStep       = 50 // horizontal distance between bar centres
	svgBarHalfWidth  = 8  // half the width of each range bar

	// Commute time window – 7:00 to 10:30.
	svgMinMinutes   = 7*60 + 0
	svgMaxMinutes   = 10*60 + 30
	svgMinutesRange = svgMaxMinutes - svgMinMinutes // 210
	svgChartHeight  = svgPaddingTop + svgPlotHeight + svgPaddingBottom
)

// commuteMinWindowMinutes and commuteMaxWindowMinutes define the start-time
// window used to decide whether a journey qualifies as a commute.
const (
	commuteMinWindowMinutes = svgMinMinutes
	commuteMaxWindowMinutes = svgMaxMinutes
)

// minutesToSVGY maps a time (in minutes from midnight) to a Y coordinate in
// the SVG chart. Earlier times appear at the top (smaller Y).
func minutesToSVGY(minutes int) int {
	return svgPaddingTop + (minutes-svgMinMinutes)*svgPlotHeight/svgMinutesRange
}

// ratingToSVGY maps a rating value (1–5) to a Y coordinate in the SVG chart,
// sharing the same plot area as the commute bars. Rating 5 maps to the top of
// the plot and rating 1 maps to the bottom.
func ratingToSVGY(rating float64) int {
	return svgPaddingTop + int((5.0-rating)/4.0*float64(svgPlotHeight))
}

// buildCommuteData filters journeys to commute candidates (Tue/Wed/Thu,
// 7:00–10:30 start time) and computes all values needed by the template.
// ratings is optional; pass nil to omit the ratings overlay.
func buildCommuteData(journeys []bq.CommuteJourney, ratings []bq.DailyRating) CommuteData {
	var points []CommutePoint
	totalMinutes := 0
	maxDuration := 0
	longestCommute := ""

	for _, j := range journeys {
		// Parse the date using the two formats supported by the app.
		t, err := time.Parse("02-Jan-06", j.Date)
		if err != nil {
			t, err = time.Parse("2006-01-02", j.Date)
			if err != nil {
				continue
			}
		}

		// Keep only Tuesday, Wednesday, Thursday.
		wd := t.Weekday()
		if wd != time.Tuesday && wd != time.Wednesday && wd != time.Thursday {
			continue
		}

		// Parse start time and apply the 7:00–10:30 window filter.
		startMins, err := parseTimeToMinutes(j.StartTime)
		if err != nil {
			continue
		}
		if startMins < commuteMinWindowMinutes || startMins > commuteMaxWindowMinutes {
			continue
		}

		// Parse end time; skip journeys without a valid end time.
		endMins, err := parseTimeToMinutes(j.EndTime)
		if err != nil {
			continue
		}
		if endMins <= startMins {
			continue
		}

		duration := endMins - startMins
		durationStr := formatDuration(duration)

		if duration > maxDuration {
			maxDuration = duration
			longestCommute = durationStr
		}
		totalMinutes += duration

		// SVG bar geometry.
		idx := len(points)
		x := svgPaddingLeft + idx*svgBarStep + svgBarStep/2
		barY := minutesToSVGY(startMins)
		barHeight := minutesToSVGY(endMins) - barY

		points = append(points, CommutePoint{
			Date:       t.Format("Mon 02 Jan"),
			ISODate:    t.Format("2006-01-02"),
			Start:      j.StartTime,
			End:        j.EndTime,
			Duration:   durationStr,
			X:          x,
			BarX:       x - svgBarHalfWidth,
			BarY:       barY,
			BarHeight:  barHeight,
			BarBottomY: barY + barHeight,
		})
	}

	avgDuration := "–"
	if len(points) > 0 {
		avgDuration = formatDuration(totalMinutes / len(points))
	}
	if longestCommute == "" {
		longestCommute = "–"
	}

	// Build Y-axis time labels every 30 minutes from 7:00 to 10:30.
	var timeLabels []TimeLabel
	for mins := svgMinMinutes; mins <= svgMaxMinutes; mins += 30 {
		timeLabels = append(timeLabels, TimeLabel{
			Y:     minutesToSVGY(mins),
			Label: fmt.Sprintf("%02d:%02d", mins/60, mins%60),
		})
	}

	numBars := len(points)
	if numBars == 0 {
		numBars = 1 // ensure a minimum-width chart even with no data
	}
	svgWidth := svgPaddingLeft + numBars*svgBarStep + svgPaddingRight

	chartBottom := svgPaddingTop + svgPlotHeight
	chartRight := svgPaddingLeft + numBars*svgBarStep

	// Build ratings overlay from the supplied daily ratings.
	ratingLookup := make(map[string]float64, len(ratings))
	for _, r := range ratings {
		ratingLookup[r.Date.Format("2006-01-02")] = r.Rating
	}

	var ratingPoints []RatingPoint
	for _, p := range points {
		rating, ok := ratingLookup[p.ISODate]
		if !ok {
			continue
		}
		ratingPoints = append(ratingPoints, RatingPoint{
			X:      p.X,
			Y:      ratingToSVGY(rating),
			Rating: rating,
			Date:   p.Date,
		})
	}

	// Right Y-axis labels for ratings 5 → 1 (top to bottom).
	var ratingLabels []RatingLabel
	for v := 5; v >= 1; v-- {
		ratingLabels = append(ratingLabels, RatingLabel{
			Y:     ratingToSVGY(float64(v)),
			Value: v,
		})
	}

	return CommuteData{
		Commutes:       points,
		TimeLabels:     timeLabels,
		TotalCommutes:  len(points),
		AvgDuration:    avgDuration,
		LongestCommute: longestCommute,
		SVGWidth:       svgWidth,
		SVGHeight:      svgChartHeight,
		ChartLeft:      svgPaddingLeft,
		ChartRight:     chartRight,
		ChartTop:       svgPaddingTop,
		ChartBottom:    chartBottom,
		LabelY:         chartBottom + 15,
		Ratings:        ratingPoints,
		RatingLabels:   ratingLabels,
		HasRatings:     len(ratingPoints) > 0,
	}
}

// parseTimeToMinutes converts a "H:MM" or "HH:MM" string into minutes from
// midnight. It tolerates an optional seconds component.
func parseTimeToMinutes(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("parsing time %q: %w", s, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q out of range", s)
	}
	return h*60 + m, nil
}

// formatDuration converts a number of minutes into a human-readable string
// such as "45m" or "1h 30m".
func formatDuration(minutes int) string {
	if minutes <= 0 {
		return "0m"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
