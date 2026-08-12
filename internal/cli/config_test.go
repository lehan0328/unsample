package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig with no path: %v", err)
	}

	if cfg.Backend.Type != "tempo" {
		t.Errorf("Backend.Type = %q, want %q", cfg.Backend.Type, "tempo")
	}
	if cfg.Backend.Endpoint != "http://localhost:3200" {
		t.Errorf("Backend.Endpoint = %q, want %q", cfg.Backend.Endpoint, "http://localhost:3200")
	}
	if cfg.Viewer.Type != "grafana" {
		t.Errorf("Viewer.Type = %q, want %q", cfg.Viewer.Type, "grafana")
	}
	if cfg.Viewer.URL != "http://localhost:3000" {
		t.Errorf("Viewer.URL = %q, want %q", cfg.Viewer.URL, "http://localhost:3000")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `secret: "file-secret"
backend:
  type: jaeger
  endpoint: http://jaeger:3200
viewer:
  type: grafana
  url: http://grafana:3000
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig from file: %v", err)
	}

	if cfg.Secret != "file-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "file-secret")
	}
	if cfg.Backend.Type != "jaeger" {
		t.Errorf("Backend.Type = %q, want %q", cfg.Backend.Type, "jaeger")
	}
	if cfg.Viewer.URL != "http://grafana:3000" {
		t.Errorf("Viewer.URL = %q, want %q", cfg.Viewer.URL, "http://grafana:3000")
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `secret: "file-secret"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	t.Setenv("UNSAMPLE_SECRET", "env-secret")

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Env should override file.
	if cfg.Secret != "env-secret" {
		t.Errorf("Secret = %q, want %q (env override)", cfg.Secret, "env-secret")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing explicit config file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: [[["), 0600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
