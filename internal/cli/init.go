package cli

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// InitOpts configures the init command.
type InitOpts struct {
	// Dir is the output directory. Default: ".unsample" in the current directory.
	Dir string

	// Force overwrites existing files.
	Force bool
}

// templateData holds values injected into templates.
type templateData struct {
	Secret string
}

// RunInit generates Unsample configuration files in the target directory.
//
// It creates:
//   - config.yaml (CLI config with generated secret)
//   - docker-compose.yaml (Collector + Tempo + Grafana)
//   - otel-collector-config.yaml (routing connector config)
//   - tempo-config.yaml (trace storage config)
//   - grafana-datasources.yaml (auto-provisioned Tempo datasource)
func RunInit(opts InitOpts) error {
	if opts.Dir == "" {
		opts.Dir = ".unsample"
	}

	// Generate a random secret.
	secret, err := generateSecret(32)
	if err != nil {
		return fmt.Errorf("generating secret: %w", err)
	}

	data := templateData{Secret: secret}

	// Create output directory.
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", opts.Dir, err)
	}

	// Process each template file.
	files := []struct {
		tmplPath string // path inside embed.FS
		outName  string // output filename
		isTmpl   bool   // true if Go template, false if static copy
	}{
		{"templates/config.yaml.tmpl", "config.yaml", true},
		{"templates/docker-compose.yaml.tmpl", "docker-compose.yaml", true},
		{"templates/otel-collector-config.yaml", "otel-collector-config.yaml", false},
		{"templates/tempo-config.yaml", "tempo-config.yaml", false},
		{"templates/grafana-datasources.yaml", "grafana-datasources.yaml", false},
	}

	created := []string{}

	for _, f := range files {
		outPath := filepath.Join(opts.Dir, f.outName)

		// Skip existing files unless --force.
		if !opts.Force {
			if _, err := os.Stat(outPath); err == nil {
				fmt.Printf("  • %s already exists (use --force to overwrite)\n", outPath)
				continue
			}
		}

		content, err := templateFS.ReadFile(f.tmplPath)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", f.tmplPath, err)
		}

		var output []byte
		if f.isTmpl {
			tmpl, err := template.New(f.outName).Parse(string(content))
			if err != nil {
				return fmt.Errorf("parsing template %s: %w", f.tmplPath, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("executing template %s: %w", f.tmplPath, err)
			}
			output = buf.Bytes()
		} else {
			output = content
		}

		if err := os.WriteFile(outPath, output, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
		created = append(created, outPath)
	}

	// Try to add .unsample/ to .gitignore.
	addToGitignore(opts.Dir)

	// Print success message.
	printInitSuccess(opts.Dir, secret, created)
	return nil
}

// generateSecret generates a cryptographically random hex-encoded secret.
func generateSecret(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// addToGitignore appends the directory to .gitignore if it exists and
// doesn't already contain the entry.
func addToGitignore(dir string) {
	gitignorePath := ".gitignore"
	entry := dir + "/"

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		// No .gitignore — skip silently.
		return
	}

	// Check if already present.
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == entry || strings.TrimSpace(line) == dir {
			return // already present
		}
	}

	// Append entry.
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	// Add newline if file doesn't end with one.
	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString(entry + "\n")
}

// printInitSuccess prints the post-init instructions.
func printInitSuccess(dir, secret string, created []string) {
	fmt.Println()
	fmt.Println("  ✓ Generated shared secret (32 bytes)")
	for _, f := range created {
		fmt.Printf("  ✓ Created %s\n", f)
	}

	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println()
	fmt.Println("    1. Start the trace backend:")
	fmt.Printf("       docker compose -f %s/docker-compose.yaml up -d\n", dir)
	fmt.Println()
	fmt.Println("    2. Add the SDK middleware to each service:")
	fmt.Println("       handler := otelhttp.NewHandler(")
	fmt.Println("           unsample.Middleware(unsample.Config{")
	fmt.Println("               Secret: os.Getenv(\"UNSAMPLE_SECRET\"),")
	fmt.Println("           })(mux),")
	fmt.Println("           \"my-service\",")
	fmt.Println("       )")
	fmt.Println()
	fmt.Println("    3. Export the secret and send a debug request:")
	fmt.Printf("       export UNSAMPLE_SECRET=%s\n", secret)
	fmt.Println("       unsample debug http://localhost:8080/your-endpoint")
	fmt.Println()
	fmt.Println("    4. Open Grafana to view the trace:")
	fmt.Println("       http://localhost:3000/explore")
	fmt.Println()
}

// Ensure embed.FS is used (prevents unused import error in tests).
var _ fs.FS = templateFS
