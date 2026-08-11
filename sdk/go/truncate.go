package unsample

// Truncation constants for payload capture (v2 feature).
//
// These limits are derived from Google Sherlog's production incidents:
//   - Incident #2: Stack overflow from recursive protobuf → MaxBodyBytes + MaxNestDepth
//   - OTel attribute size limits → MaxStringLen
//
// In v2, the middleware will capture request/response bodies as OTel LogRecords
// (NOT span attributes) with these truncation limits applied at the SDK
// BEFORE export. This prevents Collector crashes from unbounded payloads.
const (
	// MaxBodyBytes is the maximum size of a captured request or response body.
	// Bodies larger than this are truncated with a "[TRUNCATED]" suffix.
	MaxBodyBytes = 64 * 1024 // 64KB

	// MaxNestDepth is the maximum JSON/protobuf nesting depth for captured bodies.
	// Prevents stack overflow from recursive structures (Sherlog Incident #2).
	MaxNestDepth = 10

	// MaxStringLen is the maximum length of a single string attribute.
	// OTel backends enforce attribute size limits; truncate proactively.
	MaxStringLen = 4096 // 4KB
)

// TruncateBytes truncates b to maxLen bytes, appending a marker if truncated.
// Used by the middleware to enforce payload size limits before export.
func TruncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	marker := []byte("... [TRUNCATED]")
	if maxLen < len(marker) {
		return b[:maxLen]
	}
	truncated := make([]byte, maxLen)
	copy(truncated, b[:maxLen-len(marker)])
	copy(truncated[maxLen-len(marker):], marker)
	return truncated
}
