package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// printHTTPResponse writes a formatted HTTP response to the given writer.
// It displays the status line, selected headers, and the response body.
func printHTTPResponse(w io.Writer, resp *http.Response, body []byte) {
	fmt.Fprintf(w, "\n─── HTTP Response ───────────────────────────────\n")
	fmt.Fprintf(w, "%s %s\n", resp.Proto, resp.Status)

	// Print selected headers (skip noisy ones).
	for _, key := range []string{"Content-Type", "Content-Length", "Server", "Date"} {
		if val := resp.Header.Get(key); val != "" {
			fmt.Fprintf(w, "%s: %s\n", key, val)
		}
	}

	if len(body) > 0 {
		fmt.Fprintln(w)
		// Truncate very large bodies for display.
		const maxDisplay = 4096
		if len(body) > maxDisplay {
			fmt.Fprintf(w, "%s\n... (%d bytes truncated)\n", string(body[:maxDisplay]), len(body)-maxDisplay)
		} else {
			fmt.Fprintln(w, string(body))
		}
	}
}

// printTraceHeader writes the debug trace section header.
func printTraceHeader(w io.Writer) {
	fmt.Fprintf(w, "─── Debug Trace ─────────────────────────────────\n")
}

// printTraceWaiting writes a fallback message when trace polling is skipped.
func printTraceWaiting(w io.Writer, traceID string) {
	fmt.Fprintf(w, "⏳ Trace ID: %s\n", traceID)
	fmt.Fprintf(w, "   (No backend configured — set backend.endpoint in config to enable polling)\n")
}

// printTraceLink writes the deep link to the trace viewer.
func printTraceLink(w io.Writer, viewerURL, traceID string) {
	viewerURL = strings.TrimRight(viewerURL, "/")
	fmt.Fprintf(w, "   → %s/trace/%s\n", viewerURL, traceID)
}

// printSeparator writes a closing separator line.
func printSeparator(w io.Writer) {
	fmt.Fprintf(w, "─────────────────────────────────────────────────\n")
}
