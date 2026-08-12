package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the CLI configuration loaded from file and/or environment.
type Config struct {
	// Secret is the HMAC shared secret for token generation and verification.
	Secret string `yaml:"secret"`

	// Backend configures the trace storage backend for polling.
	Backend BackendConfig `yaml:"backend"`

	// Viewer configures the trace viewer for deep links.
	Viewer ViewerConfig `yaml:"viewer"`
}

// BackendConfig specifies the trace backend connection.
type BackendConfig struct {
	// Type is the backend type: "tempo" or "jaeger".
	Type string `yaml:"type"`

	// Endpoint is the HTTP API endpoint for trace polling.
	Endpoint string `yaml:"endpoint"`
}

// ViewerConfig specifies the trace viewer for deep links.
type ViewerConfig struct {
	// Type is the viewer type: "jaeger" or "grafana".
	Type string `yaml:"type"`

	// URL is the base URL of the trace viewer UI.
	URL string `yaml:"url"`
}

// defaultConfig returns a Config with sensible defaults for local development.
func defaultConfig() *Config {
	return &Config{
		Backend: BackendConfig{
			Type:     "tempo",
			Endpoint: "http://localhost:3200",
		},
		Viewer: ViewerConfig{
			Type: "grafana",
			URL:  "http://localhost:3000",
		},
	}
}

// LoadConfig loads configuration from the given path, the default config file,
// or environment variables, in that priority order.
//
// Priority (highest wins):
//  1. Explicit path (--config flag)
//  2. Default path (~/.unsample/config.yaml)
//  3. Environment variables (UNSAMPLE_SECRET)
//  4. Built-in defaults
func LoadConfig(path string) (*Config, error) {
	cfg := defaultConfig()

	// Try explicit path first.
	if path != "" {
		if err := loadFromFile(cfg, path); err != nil {
			return nil, fmt.Errorf("loading config from %s: %w", path, err)
		}
		applyEnvOverrides(cfg)
		return cfg, nil
	}

	// Try default path.
	home, err := os.UserHomeDir()
	if err == nil {
		defaultPath := filepath.Join(home, ".unsample", "config.yaml")
		if _, statErr := os.Stat(defaultPath); statErr == nil {
			// File exists — load it, but don't fail if it's malformed.
			_ = loadFromFile(cfg, defaultPath)
		}
	}

	// Environment overrides always win.
	applyEnvOverrides(cfg)

	return cfg, nil
}

// loadFromFile reads a YAML config file into the given Config struct.
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over file-based config.
func applyEnvOverrides(cfg *Config) {
	if secret := os.Getenv("UNSAMPLE_SECRET"); secret != "" {
		cfg.Secret = secret
	}
	if endpoint := os.Getenv("UNSAMPLE_BACKEND_ENDPOINT"); endpoint != "" {
		cfg.Backend.Endpoint = endpoint
	}
	if viewerURL := os.Getenv("UNSAMPLE_VIEWER_URL"); viewerURL != "" {
		cfg.Viewer.URL = viewerURL
	}
}
