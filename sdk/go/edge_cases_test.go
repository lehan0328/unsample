package unsample

import (
	"bytes"
	"testing"
	"time"
)

// --- Token Edge Case Tests ---

func TestVerifyToken_VeryLongToken(t *testing.T) {
	// 10KB of garbage — should not panic or hang.
	longToken := string(make([]byte, 10240))
	if verifyToken(longToken, testSecret, 2*time.Hour) {
		t.Error("very long token should not verify")
	}
}

func TestVerifyToken_UnicodeInToken(t *testing.T) {
	token := "🔥trace🔥:12345:sig"
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("unicode token should not verify")
	}
}

func TestVerifyToken_NullBytes(t *testing.T) {
	token := "trace\x00id:12345:sig"
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("token with null bytes should not verify")
	}
}

func TestVerifyToken_ColonsInSignature(t *testing.T) {
	// SplitN with 3 means extra colons end up in the signature field.
	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	// Append extra colon — signature should no longer match.
	if verifyToken(token+":extra", testSecret, 2*time.Hour) {
		t.Error("token with extra colons should not verify")
	}
}

func TestVerifyToken_TimestampOverflow(t *testing.T) {
	// Massive timestamp that exceeds int64 range.
	token := "traceid:99999999999999999999:sig"
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("token with overflowed timestamp should not verify")
	}
}

func TestVerifyToken_NegativeTimestamp(t *testing.T) {
	token := generateTestToken(testSecret, testTraceID, -1)
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("token with negative timestamp should not verify (expired)")
	}
}

func TestVerifyToken_ZeroMaxAge(t *testing.T) {
	// Zero max age means all tokens are expired immediately.
	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	if verifyToken(token, testSecret, 0) {
		t.Error("token should not verify with zero maxAge")
	}
}

func TestVerifyToken_EmptySecret(t *testing.T) {
	// Empty secret should still work — HMAC of empty key is valid.
	token := generateTestToken("", testTraceID, time.Now().Unix())
	if !verifyToken(token, "", 2*time.Hour) {
		t.Error("token with empty secret should verify against empty secret")
	}
}

// --- Truncation Tests ---

func TestTruncateBytes_NoTruncation(t *testing.T) {
	input := []byte("short")
	result := TruncateBytes(input, 100)
	if !bytes.Equal(result, input) {
		t.Errorf("TruncateBytes = %q, want %q", result, input)
	}
}

func TestTruncateBytes_ExactLength(t *testing.T) {
	input := []byte("exact")
	result := TruncateBytes(input, 5)
	if !bytes.Equal(result, input) {
		t.Errorf("TruncateBytes = %q, want %q", result, input)
	}
}

func TestTruncateBytes_Truncated(t *testing.T) {
	input := make([]byte, 1000)
	for i := range input {
		input[i] = 'x'
	}
	result := TruncateBytes(input, 100)

	if len(result) != 100 {
		t.Errorf("len = %d, want 100", len(result))
	}
	if !bytes.HasSuffix(result, []byte("... [TRUNCATED]")) {
		t.Errorf("result should end with truncation marker")
	}
}

func TestTruncateBytes_VerySmallMax(t *testing.T) {
	input := []byte("hello world this is long")
	result := TruncateBytes(input, 5)
	if len(result) != 5 {
		t.Errorf("len = %d, want 5", len(result))
	}
}

func TestTruncateBytes_LargePayload(t *testing.T) {
	// Simulate a 1MB payload.
	input := make([]byte, 1024*1024)
	result := TruncateBytes(input, MaxBodyBytes)

	if len(result) != MaxBodyBytes {
		t.Errorf("len = %d, want %d (MaxBodyBytes)", len(result), MaxBodyBytes)
	}
}

func TestTruncateConstants(t *testing.T) {
	// Verify constants match Sherlog's values.
	if MaxBodyBytes != 64*1024 {
		t.Errorf("MaxBodyBytes = %d, want 65536 (64KB)", MaxBodyBytes)
	}
	if MaxNestDepth != 10 {
		t.Errorf("MaxNestDepth = %d, want 10", MaxNestDepth)
	}
	if MaxStringLen != 4096 {
		t.Errorf("MaxStringLen = %d, want 4096 (4KB)", MaxStringLen)
	}
}
