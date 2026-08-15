package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecret = "test-secret-key-do-not-use-in-production"

func TestRunDebugInjectsHeaders(t *testing.T) {
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true

	err := RunDebug(context.Background(), cfg, srv.URL, opts)
	if err != nil {
		t.Fatalf("RunDebug: %v", err)
	}

	// Verify traceparent header was injected.
	traceparent := capturedHeaders.Get("Traceparent")
	if traceparent == "" {
		t.Fatal("missing traceparent header")
	}
	if !strings.HasPrefix(traceparent, "00-") {
		t.Errorf("traceparent = %q, should start with '00-'", traceparent)
	}
	if !strings.HasSuffix(traceparent, "-01") {
		t.Errorf("traceparent = %q, should end with '-01' (sampled)", traceparent)
	}

	// Verify baggage header contains the debug token.
	baggage := capturedHeaders.Get("Baggage")
	if baggage == "" {
		t.Fatal("missing baggage header")
	}
	if !strings.HasPrefix(baggage, "unsample-debug=") {
		t.Errorf("baggage = %q, should start with 'unsample-debug='", baggage)
	}
}

func TestRunDebugDisplaysResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
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
		t.Fatalf("RunDebug: %v", err)
	}

	output := buf.String()

	// Check output contains key sections.
	if !strings.Contains(output, "HTTP Response") {
		t.Error("output missing 'HTTP Response' section")
	}
	if !strings.Contains(output, "200") {
		t.Error("output missing status code 200")
	}
	if !strings.Contains(output, `{"result":"success"}`) {
		t.Error("output missing response body")
	}
	if !strings.Contains(output, "Debug Trace") {
		t.Error("output missing 'Debug Trace' section")
	}
	if !strings.Contains(output, "localhost:16686/explore") {
		t.Error("output missing trace viewer link")
	}
}

func TestRunDebugMissingSecret(t *testing.T) {
	cfg := &Config{Secret: ""}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true

	err := RunDebug(context.Background(), cfg, "https://example.com", opts)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !strings.Contains(err.Error(), "no secret configured") {
		t.Errorf("error = %q, want message about missing secret", err)
	}
}

func TestRunDebugWithCurl(t *testing.T) {
	var capturedMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	opts := DefaultDebugOpts()
	opts.Output = &bytes.Buffer{}
	opts.SkipPoll = true
	opts.CurlCmd = "curl -X POST " + srv.URL

	err := RunDebug(context.Background(), cfg, "", opts)
	if err != nil {
		t.Fatalf("RunDebug with curl: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("captured method = %q, want POST", capturedMethod)
	}
}

func TestRunDebugServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something broke"}`))
	}))
	defer srv.Close()

	cfg := &Config{
		Secret: testSecret,
		Viewer: ViewerConfig{URL: "http://localhost:16686"},
	}
	var buf bytes.Buffer
	opts := DefaultDebugOpts()
	opts.Output = &buf
	opts.SkipPoll = true

	// Should not return error — server errors are valid responses to display.
	err := RunDebug(context.Background(), cfg, srv.URL, opts)
	if err != nil {
		t.Fatalf("RunDebug should succeed even on 500: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "500") {
		t.Error("output missing status code 500")
	}
}

func TestGenerateTraceID(t *testing.T) {
	id, err := generateTraceID()
	if err != nil {
		t.Fatalf("generateTraceID: %v", err)
	}

	if len(id) != 32 {
		t.Errorf("trace ID length = %d, want 32 hex chars", len(id))
	}

	// Verify it's different each time (randomness).
	id2, _ := generateTraceID()
	if id == id2 {
		t.Error("two generated trace IDs should not be identical")
	}
}

func TestGenerateSpanID(t *testing.T) {
	id, err := generateSpanID()
	if err != nil {
		t.Fatalf("generateSpanID: %v", err)
	}

	if len(id) != 16 {
		t.Errorf("span ID length = %d, want 16 hex chars", len(id))
	}
}
