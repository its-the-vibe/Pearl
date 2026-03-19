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

// CommuteJourney holds the fields needed for commute analysis.
type CommuteJourney struct {
	Date      string
	StartTime string
	EndTime   string
}

// DailyRating holds a date and its rating value fetched from the ratings table.
type DailyRating struct {
	Date   time.Time
	Rating float64
}

// Client wraps a BigQuery client for querying Pearl data.
type Client struct {
	bq             *bigquery.Client
	project        string
	dataset        string
	ratingsDataset string
}

// New creates a new BigQuery Client using Application Default Credentials.
// ratingsDataset is optional; pass an empty string to disable the ratings overlay.
func New(ctx context.Context, project, dataset, ratingsDataset string) (*Client, error) {
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %w", err)
	}
	return &Client{bq: bq, project: project, dataset: dataset, ratingsDataset: ratingsDataset}, nil
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

// CommuteJourneys returns all journeys with their start and end times for
// commute analysis. Filtering by day and time window is done in the caller.
func (c *Client) CommuteJourneys(ctx context.Context) ([]CommuteJourney, error) {
	query := fmt.Sprintf(
		"SELECT date, start_time, end_time FROM `%s.%s.%s` WHERE start_time IS NOT NULL AND end_time IS NOT NULL ORDER BY date, start_time",
		c.project, c.dataset, journeysTable,
	)

	q := c.bq.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	type row struct {
		Date      string `bigquery:"date"`
		StartTime string `bigquery:"start_time"`
		EndTime   string `bigquery:"end_time"`
	}

	var journeys []CommuteJourney
	for {
		var r row
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}
		journeys = append(journeys, CommuteJourney{
			Date:      r.Date,
			StartTime: r.StartTime,
			EndTime:   r.EndTime,
		})
	}

	return journeys, nil
}

// Ratings returns daily ratings from the ratings table, ordered by date.
// It returns nil without error when no ratings dataset has been configured.
func (c *Client) Ratings(ctx context.Context) ([]DailyRating, error) {
	if c.ratingsDataset == "" {
		return nil, nil
	}

	query := fmt.Sprintf(
		"SELECT CAST(DATE(timestamp) AS STRING) AS day, rating FROM `%s.%s.ratings` ORDER BY 1",
		c.project, c.ratingsDataset,
	)

	q := c.bq.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing ratings query: %w", err)
	}

	type row struct {
		Day    string `bigquery:"day"`
		Rating int64  `bigquery:"rating"`
	}

	var ratings []DailyRating
	for {
		var r row
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading ratings row: %w", err)
		}

		t, err := time.Parse("2006-01-02", r.Day)
		if err != nil {
			return nil, fmt.Errorf("parsing ratings date %q: %w", r.Day, err)
		}
		ratings = append(ratings, DailyRating{Date: t, Rating: float64(r.Rating)})
	}

	return ratings, nil
}
