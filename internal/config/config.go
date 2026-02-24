package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	BigQuery struct {
		ProjectID string `yaml:"project_id"`
		Dataset   string `yaml:"dataset"`
	} `yaml:"bigquery"`
}

// Load reads and parses a YAML config file at the given path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	return &cfg, nil
}
