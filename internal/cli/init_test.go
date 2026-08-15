package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit_Default(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, ".unsample")

	err := RunInit(InitOpts{Dir: outDir})
	if err != nil {
		t.Fatalf("RunInit() error = %v", err)
	}

	// Verify all expected files exist.
	expectedFiles := []string{
		"config.yaml",
		"docker-compose.yaml",
		"otel-collector-config.yaml",
		"tempo-config.yaml",
		"grafana-datasources.yaml",
	}

	for _, name := range expectedFiles {
		path := filepath.Join(outDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", path)
		}
	}

	// Verify config.yaml contains a secret.
	config, err := os.ReadFile(filepath.Join(outDir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config.yaml: %v", err)
	}
	if !strings.Contains(string(config), "secret: ") {
		t.Error("config.yaml should contain a secret")
	}
	// Secret should be 64 hex chars (32 bytes).
	for _, line := range strings.Split(string(config), "\n") {
		if strings.HasPrefix(line, "secret: ") {
			secret := strings.TrimPrefix(line, "secret: ")
			if len(secret) != 64 {
				t.Errorf("secret length = %d, want 64", len(secret))
			}
		}
	}
}

func TestRunInit_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, ".unsample")
	_ = os.MkdirAll(outDir, 0o755)

	// Create a pre-existing config.yaml.
	existing := []byte("secret: my-existing-secret\n")
	_ = os.WriteFile(filepath.Join(outDir, "config.yaml"), existing, 0o644)

	err := RunInit(InitOpts{Dir: outDir})
	if err != nil {
		t.Fatalf("RunInit() error = %v", err)
	}

	// Verify config.yaml was NOT overwritten.
	config, _ := os.ReadFile(filepath.Join(outDir, "config.yaml"))
	if !strings.Contains(string(config), "my-existing-secret") {
		t.Error("existing config.yaml should not be overwritten without --force")
	}
}

func TestRunInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, ".unsample")
	_ = os.MkdirAll(outDir, 0o755)

	// Create a pre-existing config.yaml.
	existing := []byte("secret: my-existing-secret\n")
	_ = os.WriteFile(filepath.Join(outDir, "config.yaml"), existing, 0o644)

	err := RunInit(InitOpts{Dir: outDir, Force: true})
	if err != nil {
		t.Fatalf("RunInit() error = %v", err)
	}

	// Verify config.yaml WAS overwritten.
	config, _ := os.ReadFile(filepath.Join(outDir, "config.yaml"))
	if strings.Contains(string(config), "my-existing-secret") {
		t.Error("config.yaml should be overwritten with --force")
	}
}

func TestRunInit_DockerComposeValid(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, ".unsample")

	err := RunInit(InitOpts{Dir: outDir})
	if err != nil {
		t.Fatalf("RunInit() error = %v", err)
	}

	// Basic validation that docker-compose.yaml has expected services.
	compose, err := os.ReadFile(filepath.Join(outDir, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("reading docker-compose.yaml: %v", err)
	}

	content := string(compose)
	for _, expected := range []string{"otel-collector", "tempo", "grafana", "4317", "3000"} {
		if !strings.Contains(content, expected) {
			t.Errorf("docker-compose.yaml should contain %q", expected)
		}
	}
}

func TestGenerateSecret(t *testing.T) {
	s1, err := generateSecret(32)
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}
	if len(s1) != 64 {
		t.Errorf("secret length = %d, want 64 hex chars", len(s1))
	}

	// Should be unique each time.
	s2, _ := generateSecret(32)
	if s1 == s2 {
		t.Error("consecutive secrets should be unique")
	}
}
