package token

import (
	"strings"
	"testing"
	"time"
)

const (
	testSecret  = "test-secret-key-do-not-use-in-production"
	testTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
)

func TestGenerate(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	// Token should have 3 parts separated by ":"
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %q", len(parts), token)
	}

	// First part should be the trace ID
	if parts[0] != testTraceID {
		t.Errorf("trace ID = %q, want %q", parts[0], testTraceID)
	}

	// Second part should be a valid unix timestamp
	if parts[1] == "" {
		t.Error("timestamp part is empty")
	}

	// Third part should be a non-empty base64 signature
	if parts[2] == "" {
		t.Error("signature part is empty")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	// Same inputs should produce the same token for the same timestamp
	ts := time.Now().Unix()
	token1 := GenerateWithTimestamp(testSecret, testTraceID, ts)
	token2 := GenerateWithTimestamp(testSecret, testTraceID, ts)

	if token1 != token2 {
		t.Errorf("tokens should be deterministic:\n  got:  %q\n  want: %q", token1, token2)
	}
}

func TestGenerateDifferentSecrets(t *testing.T) {
	ts := time.Now().Unix()
	token1 := GenerateWithTimestamp("secret-a", testTraceID, ts)
	token2 := GenerateWithTimestamp("secret-b", testTraceID, ts)

	if token1 == token2 {
		t.Error("different secrets should produce different tokens")
	}
}

func TestGenerateDifferentTraceIDs(t *testing.T) {
	ts := time.Now().Unix()
	token1 := GenerateWithTimestamp(testSecret, "trace-aaa", ts)
	token2 := GenerateWithTimestamp(testSecret, "trace-bbb", ts)

	if token1 == token2 {
		t.Error("different trace IDs should produce different tokens")
	}
}

func TestVerifyValidToken(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	if !Verify(token, testSecret, DefaultMaxAge) {
		t.Error("freshly generated token should be valid")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	if Verify(token, "wrong-secret", DefaultMaxAge) {
		t.Error("token with wrong secret should be invalid")
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	// Generate a token from 3 hours ago
	ts := time.Now().Add(-3 * time.Hour).Unix()
	token := GenerateWithTimestamp(testSecret, testTraceID, ts)

	if Verify(token, testSecret, DefaultMaxAge) {
		t.Error("expired token (3h old, max 2h) should be invalid")
	}
}

func TestVerifyNotExpiredToken(t *testing.T) {
	// Generate a token from 1 hour ago (within 2h max age)
	ts := time.Now().Add(-1 * time.Hour).Unix()
	token := GenerateWithTimestamp(testSecret, testTraceID, ts)

	if !Verify(token, testSecret, DefaultMaxAge) {
		t.Error("token from 1h ago should still be valid (max 2h)")
	}
}

func TestVerifyFutureToken(t *testing.T) {
	// Generate a token from 10 minutes in the future (beyond 1 min tolerance)
	ts := time.Now().Add(10 * time.Minute).Unix()
	token := GenerateWithTimestamp(testSecret, testTraceID, ts)

	if Verify(token, testSecret, DefaultMaxAge) {
		t.Error("token from far in the future should be invalid")
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	// Tamper with the last character of the signature
	tampered := token[:len(token)-1] + "X"

	if Verify(tampered, testSecret, DefaultMaxAge) {
		t.Error("tampered token should be invalid")
	}
}

func TestVerifyTamperedTraceID(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	// Replace the trace ID portion
	parts := strings.SplitN(token, ":", 3)
	tampered := "aaaa" + parts[0][4:] + ":" + parts[1] + ":" + parts[2]

	if Verify(tampered, testSecret, DefaultMaxAge) {
		t.Error("token with tampered trace ID should be invalid")
	}
}

func TestVerifyMalformedTokens(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"no separators", "justabunchoftext"},
		{"one separator", "part1:part2"},
		{"empty trace ID", ":12345:signature"},
		{"empty timestamp", "traceid::signature"},
		{"non-numeric timestamp", "traceid:notanumber:signature"},
		{"empty signature", "traceid:12345:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if Verify(tt.token, testSecret, DefaultMaxAge) {
				t.Errorf("malformed token %q should be invalid", tt.token)
			}
		})
	}
}

func TestExtractTraceID(t *testing.T) {
	token := Generate(testSecret, testTraceID)

	got := ExtractTraceID(token)
	if got != testTraceID {
		t.Errorf("ExtractTraceID = %q, want %q", got, testTraceID)
	}
}

func TestExtractTraceIDMalformed(t *testing.T) {
	got := ExtractTraceID("not-a-valid-token")
	if got != "" {
		t.Errorf("ExtractTraceID of malformed token = %q, want empty", got)
	}
}

func TestVerifyCustomMaxAge(t *testing.T) {
	// Generate a token from 30 seconds ago
	ts := time.Now().Add(-30 * time.Second).Unix()
	token := GenerateWithTimestamp(testSecret, testTraceID, ts)

	// Should be valid with 1 minute max age
	if !Verify(token, testSecret, 1*time.Minute) {
		t.Error("30s old token should be valid with 1m max age")
	}

	// Should be invalid with 10 second max age
	if Verify(token, testSecret, 10*time.Second) {
		t.Error("30s old token should be invalid with 10s max age")
	}
}

// Benchmark the hot-path scenario: verifying an empty token (no debug flag)
func BenchmarkVerifyEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Verify("", testSecret, DefaultMaxAge)
	}
}

// Benchmark verifying a valid token (cold path)
func BenchmarkVerifyValid(b *testing.B) {
	token := Generate(testSecret, testTraceID)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(token, testSecret, DefaultMaxAge)
	}
}
