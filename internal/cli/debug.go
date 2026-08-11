package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/unsample/unsample/internal/token"
	"github.com/unsample/unsample/internal/trace"
)

// DebugOpts holds options for the debug command.
type DebugOpts struct {
	// CurlCmd is an optional curl command string to parse instead of a raw URL.
	CurlCmd string

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// Output is the writer for command output (defaults to os.Stdout).
	Output io.Writer

	// HTTPClient is the HTTP client to use (defaults to a client with timeout).
	HTTPClient *http.Client

	// SkipPoll skips trace polling (useful for tests or when backend is unavailable).
	SkipPoll bool
}

// DefaultDebugOpts returns DebugOpts with sensible defaults.
func DefaultDebugOpts() DebugOpts {
	return DebugOpts{
		Timeout: 30 * time.Second,
		Output:  os.Stdout,
	}
}

// RunDebug executes the debug command: generates a signed token, injects debug
// headers into an HTTP request, sends it, and displays the response.
func RunDebug(ctx context.Context, cfg *Config, rawURL string, opts DebugOpts) error {
	if cfg.Secret == "" {
		return errors.New("no secret configured\n\n" +
			"Set UNSAMPLE_SECRET environment variable or add 'secret' to ~/.unsample/config.yaml")
	}

	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}

	// Build the HTTP request — either from curl string or raw URL.
	var req *http.Request
	var err error
	if opts.CurlCmd != "" {
		req, err = ParseCurl(opts.CurlCmd)
		if err != nil {
			return fmt.Errorf("parsing curl command: %w", err)
		}
	} else {
		req, err = http.NewRequest("GET", rawURL, nil)
		if err != nil {
			return fmt.Errorf("building request for %s: %w", rawURL, err)
		}
	}
	req = req.WithContext(ctx)

	// Generate W3C trace identifiers.
	traceID, err := generateTraceID()
	if err != nil {
		return fmt.Errorf("generating trace ID: %w", err)
	}
	spanID, err := generateSpanID()
	if err != nil {
		return fmt.Errorf("generating span ID: %w", err)
	}

	// Generate HMAC-signed debug token.
	debugToken := token.Generate(cfg.Secret, traceID)

	// Inject debug headers.
	// traceparent: W3C trace context with trace-flags=01 (sampled).
	req.Header.Set("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, spanID))
	// baggage: carries the signed debug token across service boundaries.
	req.Header.Set("baggage", fmt.Sprintf("unsample-debug=%s", debugToken))

	fmt.Fprintf(out, "🔍 Debug tracing: %s %s\n", req.Method, req.URL)
	fmt.Fprintf(out, "   Trace ID: %s\n", traceID)

	// Send the request.
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// Display HTTP response.
	printHTTPResponse(out, resp, body)

	// Display trace info.
	fmt.Fprintln(out)
	printTraceHeader(out)

	if opts.SkipPoll || cfg.Backend.Endpoint == "" {
		// No backend configured — show trace ID and deep link without polling.
		printTraceWaiting(out, traceID)
		printTraceLink(out, cfg.Viewer.URL, traceID)
	} else {
		// Poll Tempo for the trace.
		fmt.Fprintf(out, "⏳ Waiting for trace...\n")
		pollCfg := trace.DefaultPollConfig(cfg.Backend.Endpoint)
		result := trace.Poll(ctx, traceID, pollCfg)
		trace.PrintSummary(out, result, cfg.Viewer.URL, traceID)
	}

	printSeparator(out)

	return nil
}

// generateTraceID generates a W3C-compliant 128-bit trace ID as 32 hex chars.
// Uses crypto/rand for cryptographic randomness.
func generateTraceID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// generateSpanID generates a W3C-compliant 64-bit span ID as 16 hex chars.
// Uses crypto/rand for cryptographic randomness.
func generateSpanID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
