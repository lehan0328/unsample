package unsample

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// --- Unary Interceptor Tests ---

func TestUnaryInterceptor_NoDebugToken(t *testing.T) {
	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}
	if !called {
		t.Error("handler should be called")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want 'ok'", resp)
	}
}

func TestUnaryInterceptor_ValidToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	ctx := contextWithBaggage(t, "unsample-debug", token)

	// Start a span so we can inspect attributes.
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-rpc")

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	span.End()
	tp.ForceFlush(context.Background())

	// Check debug.trace=true.
	spans := exporter.GetSpans()
	if !hasDebugAttribute(spans) {
		t.Error("expected debug.trace=true attribute on span")
	}
}

func TestUnaryInterceptor_InvalidToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})

	token := generateTestToken("wrong-secret", testTraceID, time.Now().Unix())
	ctx := contextWithBaggage(t, "unsample-debug", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-rpc")

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	span.End()
	tp.ForceFlush(context.Background())

	if hasDebugAttribute(exporter.GetSpans()) {
		t.Error("debug.trace should NOT be set for invalid token")
	}
}

func TestUnaryInterceptor_ExpiredToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})

	expiredTime := time.Now().Add(-3 * time.Hour).Unix()
	token := generateTestToken(testSecret, testTraceID, expiredTime)
	ctx := contextWithBaggage(t, "unsample-debug", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-rpc")

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	span.End()
	tp.ForceFlush(context.Background())

	if hasDebugAttribute(exporter.GetSpans()) {
		t.Error("debug.trace should NOT be set for expired token")
	}
}

func TestUnaryInterceptor_EmptySecret(t *testing.T) {
	interceptor := UnaryServerInterceptor(Config{Secret: ""})

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}

	// Even with a valid token, empty secret = no-op.
	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	ctx := contextWithBaggage(t, "unsample-debug", token)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestUnaryInterceptor_PreservesHandlerResponse(t *testing.T) {
	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})

	type testResp struct {
		Value string
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return &testResp{Value: "response-data"}, nil
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	tr, ok := resp.(*testResp)
	if !ok {
		t.Fatalf("resp type = %T, want *testResp", resp)
	}
	if tr.Value != "response-data" {
		t.Errorf("resp.Value = %q, want 'response-data'", tr.Value)
	}
}

// --- Stream Interceptor Tests ---

func TestStreamInterceptor_NoDebugToken(t *testing.T) {
	interceptor := StreamServerInterceptor(Config{Secret: testSecret})

	called := false
	handler := func(srv any, stream grpc.ServerStream) error {
		called = true
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestStreamInterceptor_ValidToken(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	interceptor := StreamServerInterceptor(Config{Secret: testSecret})

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	ctx := contextWithBaggage(t, "unsample-debug", token)

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(ctx, "test-stream")

	handler := func(srv any, stream grpc.ServerStream) error {
		return nil
	}

	ss := &fakeServerStream{ctx: ctx}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	span.End()
	tp.ForceFlush(context.Background())

	if !hasDebugAttribute(exporter.GetSpans()) {
		t.Error("expected debug.trace=true attribute on stream span")
	}
}

// --- Benchmarks ---

// BenchmarkUnaryInterceptor_HotPath benchmarks the gRPC hot path: no debug token.
// Target: O(1), zero allocation, sub-microsecond.
func BenchmarkUnaryInterceptor_HotPath(b *testing.B) {
	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		interceptor(context.Background(), nil, info, handler)
	}
}

// BenchmarkUnaryInterceptor_ColdPath benchmarks the gRPC cold path: valid token.
func BenchmarkUnaryInterceptor_ColdPath(b *testing.B) {
	interceptor := UnaryServerInterceptor(Config{Secret: testSecret})
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{}

	token := generateTestToken(testSecret, testTraceID, time.Now().Unix())
	member, _ := baggage.NewMemberRaw("unsample-debug", token)
	bag, _ := baggage.New(member)
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		interceptor(ctx, nil, info, handler)
	}
}

// --- Test Helpers ---

func contextWithBaggage(t *testing.T, key, value string) context.Context {
	t.Helper()
	member, err := baggage.NewMemberRaw(key, value)
	if err != nil {
		t.Fatalf("creating baggage member: %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatalf("creating baggage: %v", err)
	}
	return baggage.ContextWithBaggage(context.Background(), bag)
}

func hasDebugAttribute(spans []tracetest.SpanStub) bool {
	for _, s := range spans {
		for _, attr := range s.Attributes {
			if attr.Key == "debug.trace" && attr.Value.AsBool() {
				return true
			}
		}
	}
	return false
}

// fakeServerStream implements grpc.ServerStream for testing.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}

func (f *fakeServerStream) SetHeader(metadata.MD) error { return nil }
func (f *fakeServerStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeServerStream) SetTrailer(metadata.MD)       {}
func (f *fakeServerStream) SendMsg(any) error             { return nil }
func (f *fakeServerStream) RecvMsg(any) error             { return nil }

// Ensure attribute import is used.
var _ = attribute.Bool("test", true)
