package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config*.yaml")
	if err != nil {
		t.Fatalf("creating temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp config: %v", err)
	}
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
bigquery:
  project_id: "my-project"
  dataset: "my_dataset"
  ratings_dataset: "my_ratings"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.BigQuery.ProjectID != "my-project" {
		t.Errorf("BigQuery.ProjectID = %q, want %q", cfg.BigQuery.ProjectID, "my-project")
	}
	if cfg.BigQuery.Dataset != "my_dataset" {
		t.Errorf("BigQuery.Dataset = %q, want %q", cfg.BigQuery.Dataset, "my_dataset")
	}
	if cfg.BigQuery.RatingsDataset != "my_ratings" {
		t.Errorf("BigQuery.RatingsDataset = %q, want %q", cfg.BigQuery.RatingsDataset, "my_ratings")
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	// When port is omitted it should default to 8080.
	path := writeConfig(t, `
bigquery:
  project_id: "proj"
  dataset: "ds"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080 (default)", cfg.Server.Port)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("Load() expected an error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, `[this is not valid yaml: {`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected an error for invalid YAML, got nil")
	}
}

func TestLoad_UnknownField(t *testing.T) {
	// The decoder should reject unknown top-level keys.
	path := writeConfig(t, `
server:
  port: 8080
unknown_field: "oops"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected an error for unknown field, got nil")
	}
}
