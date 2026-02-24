package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	BigQuery BigQueryConfig `yaml:"bigquery"`
}

// BigQueryConfig holds BigQuery-specific configuration.
type BigQueryConfig struct {
	ProjectID string `yaml:"project_id"`
	Dataset   string `yaml:"dataset"`
}

// Load reads and parses a YAML configuration file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.BigQuery.ProjectID == "" {
		return nil, fmt.Errorf("bigquery.project_id is required")
	}
	if cfg.BigQuery.Dataset == "" {
		return nil, fmt.Errorf("bigquery.dataset is required")
	}

	return &cfg, nil
}
