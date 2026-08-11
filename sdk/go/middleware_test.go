package unsample

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const (
	testSecret  = "test-secret-key-do-not-use-in-production"
	testTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
)

// --- Middleware Tests ---

func TestMiddleware_NoDebugToken(t *testing.T) {
	handler := setupMiddleware(t, testSecret)

	// Request without debug baggage — hot path.
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	handler := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	req := requestWithBaggage(t, "/api/test", "unsample-debug", token)

	// Wrap request with a span so we can inspect attributes.
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(req.Context(), "test-span")
	defer span.End()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	span.End()

	// Force flush to get spans.
	tp.ForceFlush(context.Background())

	// Check that debug.trace=true was set.
	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}

	found := false
	for _, s := range spans {
		for _, attr := range s.Attributes {
			if attr.Key == "debug.trace" && attr.Value.AsBool() {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected debug.trace=true attribute on span")
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	handler := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Send a token signed with a DIFFERENT secret.
	token := generateTestToken("wrong-secret", testTraceID, time.Now().Unix())
	req := requestWithBaggage(t, "/api/test", "unsample-debug", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(req.Context(), "test-span")
	defer span.End()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	span.End()
	tp.ForceFlush(context.Background())

	// debug.trace should NOT be set.
	spans := exporter.GetSpans()
	for _, s := range spans {
		for _, attr := range s.Attributes {
			if attr.Key == "debug.trace" {
				t.Error("debug.trace should NOT be set for invalid token")
			}
		}
	}

	if rr.Code != http.StatusOK {
		t.Errorf("invalid token should still return 200, got %d", rr.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	handler := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Token from 3 hours ago (expired past 2h default).
	expiredTime := time.Now().Add(-3 * time.Hour).Unix()
	token := generateTestToken(testSecret, testTraceID, expiredTime)
	req := requestWithBaggage(t, "/api/test", "unsample-debug", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(req.Context(), "test-span")
	defer span.End()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	span.End()
	tp.ForceFlush(context.Background())

	// debug.trace should NOT be set.
	spans := exporter.GetSpans()
	for _, s := range spans {
		for _, attr := range s.Attributes {
			if attr.Key == "debug.trace" {
				t.Error("debug.trace should NOT be set for expired token")
			}
		}
	}
}

func TestMiddleware_EmptySecret(t *testing.T) {
	// Empty secret = no-op middleware.
	called := false
	handler := Middleware(Config{Secret: ""})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	req := requestWithBaggage(t, "/api/test", "unsample-debug", token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called even with empty secret")
	}
}

func TestMiddleware_CustomConfig(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	handler := Middleware(Config{
		Secret:       testSecret,
		BaggageKey:   "custom-debug-key",
		AttributeKey: "custom.debug",
		TokenMaxAge:  1 * time.Hour,
	})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	req := requestWithBaggage(t, "/api/test", "custom-debug-key", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(req.Context(), "test-span")
	defer span.End()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	span.End()
	tp.ForceFlush(context.Background())

	// Check custom attribute key.
	spans := exporter.GetSpans()
	found := false
	for _, s := range spans {
		for _, attr := range s.Attributes {
			if attr.Key == "custom.debug" && attr.Value.AsBool() {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected custom.debug=true attribute on span")
	}
}

func TestMiddleware_RequestPassesThrough(t *testing.T) {
	// Verify the middleware doesn't modify the request/response.
	var capturedHeader string
	handler := setupMiddleware(t, testSecret)

	inner := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeader = r.Header.Get("X-Custom")
			w.Header().Set("X-Response", "ok")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("response body"))
		}),
	)
	_ = handler // use setupMiddleware version for other tests

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("X-Custom", "custom-value")

	rr := httptest.NewRecorder()
	inner.ServeHTTP(rr, req)

	if capturedHeader != "custom-value" {
		t.Errorf("request header not passed through: got %q", capturedHeader)
	}
	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if rr.Header().Get("X-Response") != "ok" {
		t.Error("response header not passed through")
	}
}

// --- Token Verification Tests ---

func TestVerifyToken_Valid(t *testing.T) {
	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	if !verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("valid token should verify")
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token := generateTestToken("wrong-secret", testTraceID, time.Now().Unix())
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("token with wrong secret should not verify")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	token := generateTestToken(testSecret, testTraceID, time.Now().Add(-3*time.Hour).Unix())
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("expired token should not verify")
	}
}

func TestVerifyToken_Future(t *testing.T) {
	token := generateTestToken(testSecret, testTraceID, time.Now().Add(5*time.Minute).Unix())
	if verifyToken(token, testSecret, 2*time.Hour) {
		t.Error("future token should not verify")
	}
}

func TestVerifyToken_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no separators", "justtext"},
		{"one separator", "part1:part2"},
		{"empty trace ID", ":12345:sig"},
		{"empty timestamp", "traceid::sig"},
		{"non-numeric timestamp", "traceid:notanumber:sig"},
		{"empty signature", "traceid:12345:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if verifyToken(tt.token, testSecret, 2*time.Hour) {
				t.Errorf("malformed token %q should not verify", tt.token)
			}
		})
	}
}

// --- Benchmarks ---

// BenchmarkMiddleware_HotPath benchmarks the hot path: no debug token present.
// Target: O(1), zero allocation, sub-microsecond.
func BenchmarkMiddleware_HotPath(b *testing.B) {
	handler := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rr, req)
	}
}

// BenchmarkMiddleware_ColdPath benchmarks the cold path: valid debug token.
func BenchmarkMiddleware_ColdPath(b *testing.B) {
	handler := Middleware(Config{Secret: testSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	member, _ := baggage.NewMemberRaw("unsample-debug", token)
	bag, _ := baggage.New(member)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req = req.WithContext(baggage.ContextWithBaggage(req.Context(), bag))
	rr := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rr, req)
	}
}

// --- Test Helpers ---

func setupMiddleware(t *testing.T, secret string) http.Handler {
	t.Helper()
	return Middleware(Config{Secret: secret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
}

func generateTestToken(secret, traceID string, timestamp int64) string {
	payload := traceID + ":" + strconv.FormatInt(timestamp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%d:%s", traceID, timestamp, sig)
}

func requestWithBaggage(t *testing.T, path, key, value string) *http.Request {
	t.Helper()
	member, err := baggage.NewMemberRaw(key, value)
	if err != nil {
		t.Fatalf("creating baggage member: %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatalf("creating baggage: %v", err)
	}

	req := httptest.NewRequest("GET", path, nil)
	ctx := baggage.ContextWithBaggage(req.Context(), bag)
	return req.WithContext(ctx)
}

// Ensure attribute import is used.
var _ = attribute.Bool("test", true)
