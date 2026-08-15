package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Poller Tests ---

func TestPoll_TraceFoundImmediately(t *testing.T) {
	traceJSON := makeTraceJSON("billing-service", "process-payment", 150)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(traceJSON)
	}))
	defer srv.Close()

	cfg := DefaultPollConfig(srv.URL)
	cfg.Timeout = 5 * time.Second

	result := Poll(context.Background(), "abc123", cfg)

	if !result.Found {
		t.Fatal("expected trace to be found")
	}
	if result.SpanCount != 1 {
		t.Errorf("SpanCount = %d, want 1", result.SpanCount)
	}
	if result.Spans[0].ServiceName != "billing-service" {
		t.Errorf("ServiceName = %q, want %q", result.Spans[0].ServiceName, "billing-service")
	}
	if result.Spans[0].DurationMs != 150 {
		t.Errorf("DurationMs = %d, want 150", result.Spans[0].DurationMs)
	}
}

func TestPoll_TraceFoundAfterRetry(t *testing.T) {
	attempts := 0
	traceJSON := makeTraceJSON("gateway", "GET /api", 50)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(traceJSON)
	}))
	defer srv.Close()

	cfg := DefaultPollConfig(srv.URL)
	cfg.Timeout = 10 * time.Second
	cfg.Interval = 100 * time.Millisecond // fast polling for test

	result := Poll(context.Background(), "abc123", cfg)

	if !result.Found {
		t.Fatal("expected trace to be found after retries")
	}
	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
}

func TestPoll_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := DefaultPollConfig(srv.URL)
	cfg.Timeout = 500 * time.Millisecond
	cfg.Interval = 100 * time.Millisecond

	result := Poll(context.Background(), "abc123", cfg)

	if result.Found {
		t.Error("expected trace NOT to be found (timeout)")
	}
	if result.Elapsed < 400*time.Millisecond {
		t.Errorf("Elapsed = %v, expected at least ~500ms", result.Elapsed)
	}
}

func TestPoll_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := DefaultPollConfig(srv.URL)
	cfg.Timeout = 10 * time.Second
	cfg.Interval = 100 * time.Millisecond

	// Cancel after a short delay.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	result := Poll(ctx, "abc123", cfg)

	if result.Found {
		t.Error("expected trace NOT to be found (context cancelled)")
	}
}

// --- Parse Tests ---

func TestParseTempoResponse_ResourceSpans(t *testing.T) {
	body := makeTraceJSON("checkout-service", "POST /checkout", 200)

	result, err := parseTempoResponse(body)
	if err != nil {
		t.Fatalf("parseTempoResponse: %v", err)
	}

	if !result.Found {
		t.Fatal("expected Found=true")
	}
	if result.SpanCount != 1 {
		t.Errorf("SpanCount = %d, want 1", result.SpanCount)
	}
	if result.Spans[0].Name != "POST /checkout" {
		t.Errorf("Name = %q, want %q", result.Spans[0].Name, "POST /checkout")
	}
}

func TestParseTempoResponse_MultipleSpans(t *testing.T) {
	resp := tempoResponse{
		ResourceSpans: []resourceSpansBatch{
			{
				Resource: resource{Attributes: []attribute{
					{Key: "service.name", Value: attributeValue{StringValue: "gateway"}},
				}},
				ScopeSpans: []scopeSpan{{
					Spans: []span{
						{Name: "GET /api", StartTimeUnixNano: "1000000000", EndTimeUnixNano: "1050000000"},
					},
				}},
			},
			{
				Resource: resource{Attributes: []attribute{
					{Key: "service.name", Value: attributeValue{StringValue: "billing"}},
				}},
				ScopeSpans: []scopeSpan{{
					Spans: []span{
						{Name: "process-payment", StartTimeUnixNano: "1010000000", EndTimeUnixNano: "1300000000",
							Status: spanStatus{Code: "STATUS_CODE_ERROR", Message: "insufficient_funds"}},
					},
				}},
			},
		},
	}
	body, _ := json.Marshal(resp)

	result, err := parseTempoResponse(body)
	if err != nil {
		t.Fatalf("parseTempoResponse: %v", err)
	}

	if result.SpanCount != 2 {
		t.Errorf("SpanCount = %d, want 2", result.SpanCount)
	}

	// Check the error span.
	var errorSpan *SpanSummary
	for i := range result.Spans {
		if result.Spans[i].StatusCode == "ERROR" {
			errorSpan = &result.Spans[i]
			break
		}
	}
	if errorSpan == nil {
		t.Fatal("expected an ERROR span")
	}
	if errorSpan.ServiceName != "billing" {
		t.Errorf("error span ServiceName = %q, want %q", errorSpan.ServiceName, "billing")
	}
	if errorSpan.StatusMessage != "insufficient_funds" {
		t.Errorf("error span StatusMessage = %q, want %q", errorSpan.StatusMessage, "insufficient_funds")
	}
}

func TestParseTempoResponse_EmptyBatches(t *testing.T) {
	body := []byte(`{"resourceSpans":[]}`)
	result, err := parseTempoResponse(body)
	if err != nil {
		t.Fatalf("parseTempoResponse: %v", err)
	}
	if result.Found {
		t.Error("expected Found=false for empty batches")
	}
}

func TestParseTempoResponse_BatchesKey(t *testing.T) {
	// Older Tempo versions use "batches" instead of "resourceSpans".
	resp := tempoResponse{
		Batches: []resourceSpansBatch{{
			Resource: resource{Attributes: []attribute{
				{Key: "service.name", Value: attributeValue{StringValue: "legacy"}},
			}},
			ScopeSpans: []scopeSpan{{
				Spans: []span{{Name: "old-span", StartTimeUnixNano: "0", EndTimeUnixNano: "100000000"}},
			}},
		}},
	}
	body, _ := json.Marshal(resp)

	result, err := parseTempoResponse(body)
	if err != nil {
		t.Fatalf("parseTempoResponse: %v", err)
	}
	if result.SpanCount != 1 {
		t.Errorf("SpanCount = %d, want 1 (from batches key)", result.SpanCount)
	}
}

// --- Summary Output Tests ---

func TestPrintSummary_Found(t *testing.T) {
	var buf bytes.Buffer
	result := PollResult{
		Found:     true,
		SpanCount: 2,
		Spans: []SpanSummary{
			{ServiceName: "gateway", Name: "GET /api", DurationMs: 50, StatusCode: "OK"},
			{ServiceName: "billing", Name: "charge", DurationMs: 300, StatusCode: "ERROR", StatusMessage: "timeout"},
		},
	}

	PrintSummary(&buf, result, "http://localhost:16686", "abc123")
	output := buf.String()

	if output == "" {
		t.Fatal("expected non-empty output")
	}
	// Check key elements are present.
	checks := []string{"✅ Trace captured", "2 spans", "300ms", "abc123", "gateway", "billing", "❌", "ERROR", "timeout"}
	for _, check := range checks {
		if !bytes.Contains([]byte(output), []byte(check)) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestPrintSummary_NotFound(t *testing.T) {
	var buf bytes.Buffer
	result := PollResult{
		Found:   false,
		Elapsed: 30 * time.Second,
	}

	PrintSummary(&buf, result, "http://localhost:16686", "abc123")
	output := buf.String()

	if output == "" {
		t.Fatal("expected non-empty output")
	}
	checks := []string{"not found", "abc123", "localhost:16686"}
	for _, check := range checks {
		if !bytes.Contains([]byte(output), []byte(check)) {
			t.Errorf("output missing %q", check)
		}
	}
}

// --- Helpers ---

func TestNormalizeStatusCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"STATUS_CODE_OK", "OK"},
		{"STATUS_CODE_ERROR", "ERROR"},
		{"Ok", "OK"},
		{"Error", "ERROR"},
		{"1", "OK"},
		{"2", "ERROR"},
		{"", "OK"},     // UNSET → OK
		{"UNSET", "OK"}, // UNSET → OK
	}

	for _, tt := range tests {
		got := normalizeStatusCode(tt.input)
		if got != tt.want {
			t.Errorf("normalizeStatusCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestComputeDurationMs(t *testing.T) {
	// 1 second = 1,000,000,000 nanoseconds → 1000ms
	got := computeDurationMs("1000000000", "2000000000")
	if got != 1000 {
		t.Errorf("computeDurationMs = %d, want 1000", got)
	}
}

func TestPluralize(t *testing.T) {
	if pluralize(1) != "" {
		t.Error("pluralize(1) should return empty string")
	}
	if pluralize(2) != "s" {
		t.Error("pluralize(2) should return 's'")
	}
	if pluralize(0) != "s" {
		t.Error("pluralize(0) should return 's'")
	}
}

// makeTraceJSON builds a minimal Tempo-style JSON response.
func makeTraceJSON(serviceName, spanName string, durationMs int) []byte {
	startNano := int64(1_000_000_000)
	endNano := startNano + int64(durationMs)*1_000_000

	resp := tempoResponse{
		ResourceSpans: []resourceSpansBatch{{
			Resource: resource{Attributes: []attribute{
				{Key: "service.name", Value: attributeValue{StringValue: serviceName}},
			}},
			ScopeSpans: []scopeSpan{{
				Spans: []span{{
					Name:              spanName,
					StartTimeUnixNano: fmt.Sprintf("%d", startNano),
					EndTimeUnixNano:   fmt.Sprintf("%d", endNano),
					Status:            spanStatus{Code: "STATUS_CODE_OK"},
				}},
			}},
		}},
	}

	body, _ := json.Marshal(resp)
	return body
}

