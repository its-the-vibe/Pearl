package bigquery

import (
	"context"
	"fmt"
	"time"

	bq "cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

const journeysTable = "journeys"

// Journey represents a single row from the journeys BigQuery table.
type Journey struct {
	Date             string    `bigquery:"date"`
	StartTime        string    `bigquery:"start_time"`
	EndTime          string    `bigquery:"end_time"`
	JourneyAction    string    `bigquery:"journey_action"`
	Charge           float64   `bigquery:"charge"`
	Credit           float64   `bigquery:"credit"`
	Balance          float64   `bigquery:"balance"`
	Note             string    `bigquery:"note"`
	MessageID        string    `bigquery:"message_id"`
	PublishTime      time.Time `bigquery:"publish_time"`
	Attributes       string    `bigquery:"attributes"`
	SubscriptionName string    `bigquery:"subscription_name"`
}

// Client wraps a BigQuery client scoped to a project and dataset.
type Client struct {
	bqClient  *bq.Client
	projectID string
	dataset   string
}

// New creates a new Client connected to the given project and dataset.
func New(ctx context.Context, projectID, dataset string) (*Client, error) {
	bqClient, err := bq.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("creating BigQuery client: %w", err)
	}
	return &Client{
		bqClient:  bqClient,
		projectID: projectID,
		dataset:   dataset,
	}, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	return c.bqClient.Close()
}

// ActivityByDay returns a map of date string (YYYY-MM-DD) to journey count,
// representing the number of journeys per day.
func (c *Client) ActivityByDay(ctx context.Context) (map[string]int, error) {
	query := fmt.Sprintf(
		"SELECT date, COUNT(*) AS journey_count FROM `%s.%s.%s` GROUP BY date ORDER BY date",
		c.projectID, c.dataset, journeysTable,
	)

	q := c.bqClient.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("running BigQuery query: %w", err)
	}

	activity := make(map[string]int)
	for {
		var row struct {
			Date         string `bigquery:"date"`
			JourneyCount int    `bigquery:"journey_count"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading BigQuery row: %w", err)
		}
		activity[row.Date] = row.JourneyCount
	}

	return activity, nil
}
