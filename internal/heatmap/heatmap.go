package heatmap

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// intensity levels represented by block characters and ANSI background colours.
var (
	levels = []string{"  ", "░░", "▒▒", "▓▓", "██"}
	colors = []string{
		"\033[48;5;237m", // dark grey  – 0 journeys
		"\033[48;5;22m",  // dark green – 1-2
		"\033[48;5;28m",  // mid green  – 3-4
		"\033[48;5;34m",  // bright green – 5-7
		"\033[48;5;46m",  // vivid green  – 8+
	}
	reset = "\033[0m"
)

func levelFor(count, max int) int {
	if count == 0 || max == 0 {
		return 0
	}
	// Distribute counts across levels 1-4.
	switch {
	case count >= max:
		return 4
	case float64(count) >= float64(max)*0.75:
		return 3
	case float64(count) >= float64(max)*0.5:
		return 2
	default:
		return 1
	}
}

// Render writes a GitHub-style heatmap of daily journey activity to w.
// activity maps date strings ("2006-01-02") to journey counts.
func Render(w io.Writer, activity map[string]int) error {
	if len(activity) == 0 {
		fmt.Fprintln(w, "No journey data found.")
		return nil
	}

	// Collect and sort dates.
	dates := make([]string, 0, len(activity))
	for d := range activity {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// Determine the overall date range (full weeks, starting on Monday).
	first, err := time.Parse("2006-01-02", dates[0])
	if err != nil {
		return fmt.Errorf("parsing date %q: %w", dates[0], err)
	}
	last, err := time.Parse("2006-01-02", dates[len(dates)-1])
	if err != nil {
		return fmt.Errorf("parsing date %q: %w", dates[len(dates)-1], err)
	}

	// Align start to Monday of the first week.
	for first.Weekday() != time.Monday {
		first = first.AddDate(0, 0, -1)
	}
	// Align end to Sunday of the last week.
	for last.Weekday() != time.Sunday {
		last = last.AddDate(0, 0, 1)
	}

	// Find max count for scaling.
	max := 0
	for _, c := range activity {
		if c > max {
			max = c
		}
	}

	// Build week columns: each column is 7 days (Mon→Sun).
	type week struct {
		label string // month abbreviation for the first day
		days  []time.Time
	}

	var weeks []week
	cur := first
	prevMonth := time.Month(0)
	for !cur.After(last) {
		w_ := week{}
		if cur.Month() != prevMonth {
			w_.label = cur.Format("Jan")
			prevMonth = cur.Month()
		} else {
			w_.label = "   "
		}
		for i := 0; i < 7; i++ {
			w_.days = append(w_.days, cur)
			cur = cur.AddDate(0, 0, 1)
		}
		weeks = append(weeks, w_)
	}

	// Row labels (day abbreviations, Mon-Sun).
	dayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	// Print month header row.
	fmt.Fprintf(w, "     ") // indent for day labels
	for _, wk := range weeks {
		fmt.Fprintf(w, "%-3s ", wk.label)
	}
	fmt.Fprintln(w)

	// Print one row per weekday.
	for row := 0; row < 7; row++ {
		fmt.Fprintf(w, "%s ", dayLabels[row])
		for _, wk := range weeks {
			d := wk.days[row]
			key := d.Format("2006-01-02")
			count := activity[key]
			lvl := levelFor(count, max)
			if count > 0 {
				fmt.Fprintf(w, "%s%s%s ", colors[lvl], levels[lvl], reset)
			} else {
				fmt.Fprintf(w, "%s%s%s ", colors[0], levels[0], reset)
			}
		}
		fmt.Fprintln(w)
	}

	// Print legend.
	fmt.Fprintf(w, "\nLegend: ")
	labels := []string{"0", "low", "med", "high", "peak"}
	for i, c := range colors {
		fmt.Fprintf(w, "%s%s%s %s  ", c, levels[i], reset, labels[i])
	}
	fmt.Fprintln(w)

	// Print summary stats.
	total := 0
	for _, c := range activity {
		total += c
	}
	fmt.Fprintf(w, "\nTotal journeys: %d across %d days\n", total, len(activity))

	return nil
}
