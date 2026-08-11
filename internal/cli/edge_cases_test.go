package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunDebugInvalidURL(t *testing.T) {
	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true

	err := RunDebug(context.Background(), cfg, "://invalid-url", opts)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "building request") {
		t.Errorf("error = %q, should mention building request", err)
	}
}

func TestRunDebugConnectionRefused(t *testing.T) {
	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true
	opts.Timeout = 1 * time.Second

	// Port 1 is almost certainly not listening.
	err := RunDebug(context.Background(), cfg, "http://localhost:1/endpoint", opts)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "sending request") {
		t.Errorf("error = %q, should mention sending request", err)
	}
}

func TestRunDebugTimeout(t *testing.T) {
	// Server that never responds.
	srv := newSlowServer(10 * time.Second)
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true
	opts.Timeout = 100 * time.Millisecond

	err := RunDebug(context.Background(), cfg, srv.URL, opts)
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if !strings.Contains(err.Error(), "sending request") {
		t.Errorf("error = %q, should mention sending request", err)
	}
}

func TestRunDebugContextCancellation(t *testing.T) {
	// Server that blocks until context is cancelled.
	srv := newSlowServer(10 * time.Second)
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := RunDebug(ctx, cfg, srv.URL, opts)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRunDebugLargeResponseBody(t *testing.T) {
	// 1MB response — should not crash, output should be truncated.
	largeBody := strings.Repeat("x", 1024*1024)
	srv := newJSONServer(http.StatusOK, largeBody)
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	var buf bytes.Buffer
	opts := DefaultDebugOpts()
	opts.Output = &buf
	opts.SkipPoll = true

	err := RunDebug(context.Background(), cfg, srv.URL, opts)
	if err != nil {
		t.Fatalf("RunDebug should handle large bodies: %v", err)
	}

	// Output should contain truncation indicator.
	output := buf.String()
	if !strings.Contains(output, "TRUNCATED") && len(output) > 1024*1024 {
		t.Error("large body should be truncated in output")
	}
}

func TestRunDebugEmptyResponseBody(t *testing.T) {
	srv := newJSONServer(http.StatusNoContent, "")
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	var buf bytes.Buffer
	opts := DefaultDebugOpts()
	opts.Output = &buf
	opts.SkipPoll = true

	err := RunDebug(context.Background(), cfg, srv.URL, opts)
	if err != nil {
		t.Fatalf("RunDebug should handle empty bodies: %v", err)
	}
}

func TestRunDebugStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"301 Redirect", http.StatusMovedPermanently},
		{"400 Bad Request", http.StatusBadRequest},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newJSONServer(tt.status, `{"code":`+http.StatusText(tt.status)+`}`)
			defer srv.Close()

			cfg := &Config{
				Secret: testSecret,
				Viewer: ViewerConfig{URL: "http://localhost:16686"},
			}
			opts := DefaultDebugOpts()
			opts.Output = &bytes.Buffer{}
			opts.SkipPoll = true

			// RunDebug should NEVER return an error for HTTP status codes.
			// Even 500s are valid responses that the user needs to see.
			err := RunDebug(context.Background(), cfg, srv.URL, opts)
			if err != nil {
				t.Errorf("RunDebug should not fail on %d: %v", tt.status, err)
			}
		})
	}
}

// --- Test Helpers ---

func newSlowServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))
}

func newJSONServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}
