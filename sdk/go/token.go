package unsample

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// verifyToken checks that a debug token is correctly signed and not expired.
//
// Token format: <trace_id>:<unix_timestamp>:<hmac_base64url>
//
// This is a self-contained implementation that duplicates the verification
// logic from internal/token. The SDK module is published independently and
// must NOT import from internal/ (see unsample-architecture skill).
//
// Verification order (cheap checks first, crypto last):
//  1. Parse token into 3 parts
//  2. Check timestamp not expired (within maxAge)
//  3. Check timestamp not from future (1-minute clock skew tolerance)
//  4. Recompute HMAC and constant-time compare
func verifyToken(tokenStr, secret string, maxAge time.Duration) bool {
	// Parse into exactly 3 parts.
	parts := strings.SplitN(tokenStr, ":", 3)
	if len(parts) != 3 {
		return false
	}

	traceID := parts[0]
	if traceID == "" {
		return false
	}

	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}

	sig := parts[2]
	if sig == "" {
		return false
	}

	// Time checks (fast, no crypto).
	now := time.Now()
	tokenTime := time.Unix(timestamp, 0)

	if now.Sub(tokenTime) > maxAge {
		return false // expired
	}
	if tokenTime.After(now.Add(1 * time.Minute)) {
		return false // future token (clock skew)
	}

	// HMAC verification (expensive, last).
	payload := traceID + ":" + strconv.FormatInt(timestamp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}
