package trace

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// PrintSummary writes a formatted trace summary to the given writer.
// It shows the span count, total duration, and a lightweight tree of spans.
//
// Example output:
//
//	✅ Trace captured (3 spans, 847ms)
//	   → http://localhost:16686/trace/4bf92f3577b34da6
//
//	   gateway            12ms  ✅ OK
//	   billing-service   340ms  ❌ ERROR  subscription_not_found
//	   notification        8ms  ✅ OK
func PrintSummary(w io.Writer, result PollResult, viewerURL, traceID string) {
	if !result.Found {
		fmt.Fprintf(w, "⏳ Trace not found after %s\n", result.Elapsed.Round(time.Second))
		fmt.Fprintf(w, "   The trace may still be ingesting. Check manually:\n")
		fmt.Fprintf(w, "   → %s/trace/%s\n", strings.TrimRight(viewerURL, "/"), traceID)
		return
	}

	// Total duration is the max span duration (approximation for the root span).
	var maxDuration int64
	for _, s := range result.Spans {
		if s.DurationMs > maxDuration {
			maxDuration = s.DurationMs
		}
	}

	fmt.Fprintf(w, "✅ Trace captured (%d span%s, %dms)\n",
		result.SpanCount, pluralize(result.SpanCount), maxDuration)
	fmt.Fprintf(w, "   → %s/trace/%s\n",
		strings.TrimRight(viewerURL, "/"), traceID)

	if len(result.Spans) > 0 {
		fmt.Fprintln(w)
		printSpanTree(w, result.Spans)
	}
}

// printSpanTree writes a flat list of spans with aligned columns.
func printSpanTree(w io.Writer, spans []SpanSummary) {
	// Calculate column widths for alignment.
	maxNameLen := 0
	for _, s := range spans {
		label := s.ServiceName
		if label == "unknown" {
			label = s.Name
		}
		if len(label) > maxNameLen {
			maxNameLen = len(label)
		}
	}

	for _, s := range spans {
		label := s.ServiceName
		if label == "unknown" {
			label = s.Name
		}

		statusIcon := "✅"
		statusSuffix := ""
		if s.StatusCode == "ERROR" {
			statusIcon = "❌"
			if s.StatusMessage != "" {
				statusSuffix = "  " + s.StatusMessage
			}
		}

		fmt.Fprintf(w, "   %-*s  %4dms  %s %s%s\n",
			maxNameLen, label, s.DurationMs, statusIcon, s.StatusCode, statusSuffix)
	}
}

// pluralize returns "s" for counts other than 1.
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
