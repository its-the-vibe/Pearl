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
	for w := 0; w < numWeeks; w++ {
		day := startSunday.AddDate(0, 0, w*7)
		m := int(day.Month())
		if m != prevMonth {
			labels = append(labels, MonthLabel{
				Name:   day.Format("Jan"),
				Offset: w * cellWidth,
			})
			if len(labels) > 1 {
				prev := &labels[len(labels)-2]
				prev.Width = w*cellWidth - prev.Offset
			}
			prevMonth = m
		}
	}
	if len(labels) > 0 {
		last := &labels[len(labels)-1]
		last.Width = numWeeks*cellWidth - last.Offset
	}
	return labels
}
