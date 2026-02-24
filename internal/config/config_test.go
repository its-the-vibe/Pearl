package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/its-the-vibe/pearl/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	p := writeConfig(t, "bigquery:\n  project_id: my-project\n  dataset: my_dataset\n")
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BigQuery.ProjectID != "my-project" {
		t.Errorf("project_id = %q, want %q", cfg.BigQuery.ProjectID, "my-project")
	}
	if cfg.BigQuery.Dataset != "my_dataset" {
		t.Errorf("dataset = %q, want %q", cfg.BigQuery.Dataset, "my_dataset")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_MissingProjectID(t *testing.T) {
	p := writeConfig(t, "bigquery:\n  dataset: my_dataset\n")
	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for missing project_id, got nil")
	}
}

func TestLoad_MissingDataset(t *testing.T) {
	p := writeConfig(t, "bigquery:\n  project_id: my-project\n")
	_, err := config.Load(p)
	if err == nil {
		t.Error("expected error for missing dataset, got nil")
	}
}
