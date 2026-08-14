package unsample

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// responseCapture wraps an http.ResponseWriter to capture the response body
// written by the handler. Only used on the debug cold path.
type responseCapture struct {
	http.ResponseWriter
	body       bytes.Buffer
	maxBytes   int
	overflowed bool
}

func newResponseCapture(w http.ResponseWriter, maxBytes int) *responseCapture {
	return &responseCapture{
		ResponseWriter: w,
		maxBytes:       maxBytes,
	}
}

// Write captures bytes up to maxBytes, then stops buffering but still writes through.
func (rc *responseCapture) Write(b []byte) (int, error) {
	if !rc.overflowed {
		remaining := rc.maxBytes - rc.body.Len()
		if remaining > 0 {
			if len(b) <= remaining {
				rc.body.Write(b)
			} else {
				rc.body.Write(b[:remaining])
				rc.overflowed = true
			}
		} else {
			rc.overflowed = true
		}
	}
	return rc.ResponseWriter.Write(b)
}

// capturedBody returns the captured response body, truncated if needed.
func (rc *responseCapture) capturedBody() (string, bool) {
	if rc.body.Len() == 0 {
		return "", false
	}
	body := rc.body.Bytes()
	truncated := rc.overflowed
	return string(TruncateBytes(body, MaxBodyBytes)), truncated
}

// captureRequestBody reads the request body up to MaxBodyBytes, then restores
// r.Body so the handler can still read it. Returns the captured body and
// whether it was truncated.
func captureRequestBody(r *http.Request) (string, bool) {
	if r.Body == nil || r.Body == http.NoBody {
		return "", false
	}

	// Read up to MaxBodyBytes + 1 to detect truncation.
	buf := make([]byte, MaxBodyBytes+1)
	n, _ := io.ReadFull(r.Body, buf)
	r.Body.Close()

	truncated := n > MaxBodyBytes
	if truncated {
		n = MaxBodyBytes
	}
	captured := buf[:n]

	// Restore the body: combine captured bytes + any remaining unread bytes.
	if truncated {
		// There may be more data. Create a reader that replays captured + remaining.
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(captured), r.Body))
	} else {
		r.Body = io.NopCloser(bytes.NewReader(captured))
	}

	body := string(TruncateBytes(captured, MaxBodyBytes))
	return body, truncated
}

// isTextContent checks if the Content-Type header suggests text/JSON content
// that is safe and useful to capture. Binary content is skipped.
func isTextContent(contentType string) bool {
	if contentType == "" {
		return true // assume text if no content-type
	}
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json") ||
		strings.Contains(ct, "text") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "form-urlencoded") ||
		strings.Contains(ct, "graphql")
}
