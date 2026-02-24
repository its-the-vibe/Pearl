package bigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

const journeysTable = "journeys"

// Journey represents a single row from the journeys BigQuery table.
type Journey struct {
	Date             string
	StartTime        string
	EndTime          string
	JourneyAction    string
	Charge           float64
	Credit           float64
	Balance          float64
	Note             string
	MessageID        string
	PublishTime      time.Time
	Attributes       string
	SubscriptionName string
}

// DayCount holds a date and its journey count used for the heatmap.
type DayCount struct {
	Date  time.Time
	Count int
}

// Client wraps a BigQuery client for querying Pearl data.
type Client struct {
	bq      *bigquery.Client
	project string
	dataset string
}

// New creates a new BigQuery Client using Application Default Credentials.
func New(ctx context.Context, project, dataset string) (*Client, error) {
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %w", err)
	}
	return &Client{bq: bq, project: project, dataset: dataset}, nil
}

// Close releases the underlying BigQuery client resources.
func (c *Client) Close() error {
	return c.bq.Close()
}

// JourneyCountsByDay returns the count of journeys per day ordered by date.
func (c *Client) JourneyCountsByDay(ctx context.Context) ([]DayCount, error) {
	query := fmt.Sprintf(
		"SELECT date, COUNT(*) AS journey_count FROM `%s.%s.%s` GROUP BY date ORDER BY date",
		c.project, c.dataset, journeysTable,
	)

	q := c.bq.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	type row struct {
		Date         string `bigquery:"date"`
		JourneyCount int    `bigquery:"journey_count"`
	}

	var counts []DayCount
	for {
		var r row
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}

		t, err := time.Parse("02-Jan-06", r.Date)
		if err != nil {
			// Try alternative formats common in Oyster exports.
			t, err = time.Parse("2006-01-02", r.Date)
			if err != nil {
				return nil, fmt.Errorf("parsing date %q: %w", r.Date, err)
			}
		}

		counts = append(counts, DayCount{Date: t, Count: r.JourneyCount})
	}

	return counts, nil
}
