package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	bqclient "github.com/its-the-vibe/pearl/internal/bigquery"
	"github.com/its-the-vibe/pearl/internal/config"
	"github.com/its-the-vibe/pearl/internal/heatmap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()

	client, err := bqclient.New(ctx, cfg.BigQuery.ProjectID, cfg.BigQuery.Dataset)
	if err != nil {
		log.Fatalf("creating BigQuery client: %v", err)
	}
	defer client.Close()

	fmt.Println("Pearl – London Oyster Analytics Dashboard")
	fmt.Println("=========================================")
	fmt.Println()

	activity, err := client.ActivityByDay(ctx)
	if err != nil {
		log.Fatalf("fetching activity data: %v", err)
	}

	if err := heatmap.Render(os.Stdout, activity); err != nil {
		log.Fatalf("rendering heatmap: %v", err)
	}
}
