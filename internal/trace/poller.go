package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PollConfig configures the trace polling behavior.
type PollConfig struct {
	// Endpoint is the Tempo HTTP API base URL (e.g. http://localhost:3200).
	Endpoint string

	// Timeout is the maximum time to wait for the trace to appear.
	Timeout time.Duration

	// Interval is the time between poll attempts.
	Interval time.Duration

	// HTTPClient is the HTTP client to use (optional, defaults to http.DefaultClient).
	HTTPClient *http.Client
}

// DefaultPollConfig returns a PollConfig with sensible defaults.
func DefaultPollConfig(endpoint string) PollConfig {
	return PollConfig{
		Endpoint: endpoint,
		Timeout:  30 * time.Second,
		Interval: 2 * time.Second,
	}
}

// PollResult holds the outcome of polling for a trace.
type PollResult struct {
	// Found is true if the trace was found within the timeout.
	Found bool

	// SpanCount is the total number of spans in the trace.
	SpanCount int

	// Spans is the list of span summaries, ordered by start time.
	Spans []SpanSummary

	// Elapsed is how long polling took.
	Elapsed time.Duration
}

// SpanSummary is a lightweight representation of a single span.
type SpanSummary struct {
	// ServiceName is the service that produced this span.
	ServiceName string

	// Name is the span's operation name.
	Name string

	// DurationMs is the span duration in milliseconds.
	DurationMs int64

	// StatusCode is the span's status (OK, ERROR, UNSET).
	StatusCode string

	// StatusMessage is the optional error message.
	StatusMessage string
}

// Poll polls the Tempo API for a trace by ID, retrying with the configured
// interval until the trace is found or the timeout expires.
//
// This implements safety guardrail #9: poll before showing the deep link
// to avoid "Trace Not Found" 404 UX.
func Poll(ctx context.Context, traceID string, cfg PollConfig) PollResult {
	start := time.Now()
	deadline := start.Add(cfg.Timeout)

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	for {
		result, err := fetchTrace(ctx, client, cfg.Endpoint, traceID)
		if err == nil && result.Found {
			result.Elapsed = time.Since(start)
			return result
		}

		// Check if we've exceeded the timeout.
		if time.Now().After(deadline) {
			return PollResult{
				Found:   false,
				Elapsed: time.Since(start),
			}
		}

		// Wait before next attempt, respecting context cancellation.
		select {
		case <-ctx.Done():
			return PollResult{
				Found:   false,
				Elapsed: time.Since(start),
			}
		case <-time.After(cfg.Interval):
			// Continue polling.
		}
	}
}

// fetchTrace makes a single request to the Tempo API for a trace.
func fetchTrace(ctx context.Context, client *http.Client, endpoint, traceID string) (PollResult, error) {
	url := fmt.Sprintf("%s/api/traces/%s", endpoint, traceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return PollResult{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return PollResult{}, fmt.Errorf("fetching trace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return PollResult{Found: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return PollResult{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PollResult{}, fmt.Errorf("reading response: %w", err)
	}

	return parseTempoResponse(body)
}

// tempoResponse represents the Tempo API JSON response structure.
// Tempo returns traces in OTLP JSON format under a "batches" or
// "resourceSpans" key.
type tempoResponse struct {
	Batches       []resourceSpansBatch `json:"batches"`
	ResourceSpans []resourceSpansBatch `json:"resourceSpans"`
}

type resourceSpansBatch struct {
	Resource   resource    `json:"resource"`
	ScopeSpans []scopeSpan `json:"scopeSpans"`
}

type resource struct {
	Attributes []attribute `json:"attributes"`
}

type scopeSpan struct {
	Spans []span `json:"spans"`
}

type span struct {
	Name              string      `json:"name"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`
	Status            spanStatus  `json:"status"`
	Attributes        []attribute `json:"attributes"`
}

type spanStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type attribute struct {
	Key   string         `json:"key"`
	Value attributeValue `json:"value"`
}

type attributeValue struct {
	StringValue string `json:"stringValue"`
}

// parseTempoResponse parses the Tempo JSON response into a PollResult.
func parseTempoResponse(body []byte) (PollResult, error) {
	var resp tempoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return PollResult{}, fmt.Errorf("parsing trace JSON: %w", err)
	}

	// Tempo uses either "batches" or "resourceSpans" depending on version.
	batches := resp.ResourceSpans
	if len(batches) == 0 {
		batches = resp.Batches
	}
	if len(batches) == 0 {
		return PollResult{Found: false}, nil
	}

	var spans []SpanSummary
	for _, batch := range batches {
		svcName := extractServiceName(batch.Resource)
		for _, ss := range batch.ScopeSpans {
			for _, s := range ss.Spans {
				spans = append(spans, SpanSummary{
					ServiceName:   svcName,
					Name:          s.Name,
					DurationMs:    computeDurationMs(s.StartTimeUnixNano, s.EndTimeUnixNano),
					StatusCode:    normalizeStatusCode(s.Status.Code),
					StatusMessage: s.Status.Message,
				})
			}
		}
	}

	return PollResult{
		Found:     true,
		SpanCount: len(spans),
		Spans:     spans,
	}, nil
}

// extractServiceName finds service.name from resource attributes.
func extractServiceName(r resource) string {
	for _, attr := range r.Attributes {
		if attr.Key == "service.name" {
			return attr.Value.StringValue
		}
	}
	return "unknown"
}

// computeDurationMs calculates duration from nanosecond timestamps.
func computeDurationMs(startNano, endNano string) int64 {
	var start, end int64
	_, _ = fmt.Sscanf(startNano, "%d", &start)
	_, _ = fmt.Sscanf(endNano, "%d", &end)
	if end <= start {
		return 0
	}
	return (end - start) / 1_000_000 // nano → ms
}

// normalizeStatusCode normalizes OTLP status codes to display strings.
func normalizeStatusCode(code string) string {
	switch code {
	case "STATUS_CODE_OK", "Ok", "1":
		return "OK"
	case "STATUS_CODE_ERROR", "Error", "2":
		return "ERROR"
	default:
		return "OK" // UNSET is treated as OK
	}
}
