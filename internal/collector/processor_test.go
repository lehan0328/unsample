package unsampleprocessor

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// --- Processor Tests ---

func TestProcessTraces_DebugSpanRouted(t *testing.T) {
	proc := newTestProcessor(t, 10)
	td := newTraces(withDebugSpan("checkout", true))

	result := proc.ProcessTraces(context.Background(), td)

	if result.DebugSpanCount != 1 {
		t.Errorf("DebugSpanCount = %d, want 1", result.DebugSpanCount)
	}
	if result.ProductionSpanCount != 0 {
		t.Errorf("ProductionSpanCount = %d, want 0", result.ProductionSpanCount)
	}
	if result.Debug.SpanCount() != 1 {
		t.Errorf("Debug.SpanCount = %d, want 1", result.Debug.SpanCount())
	}
}

func TestProcessTraces_ProductionSpanRouted(t *testing.T) {
	proc := newTestProcessor(t, 10)
	td := newTraces(withPlainSpan("checkout"))

	result := proc.ProcessTraces(context.Background(), td)

	if result.ProductionSpanCount != 1 {
		t.Errorf("ProductionSpanCount = %d, want 1", result.ProductionSpanCount)
	}
	if result.DebugSpanCount != 0 {
		t.Errorf("DebugSpanCount = %d, want 0", result.DebugSpanCount)
	}
	if result.Production.SpanCount() != 1 {
		t.Errorf("Production.SpanCount = %d, want 1", result.Production.SpanCount())
	}
}

func TestProcessTraces_DebugFalseIsProduction(t *testing.T) {
	proc := newTestProcessor(t, 10)
	td := newTraces(withDebugSpan("checkout", false))

	result := proc.ProcessTraces(context.Background(), td)

	if result.ProductionSpanCount != 1 {
		t.Errorf("debug.trace=false should route to production, got DebugSpanCount=%d", result.DebugSpanCount)
	}
}

func TestProcessTraces_MixedSpans(t *testing.T) {
	proc := newTestProcessor(t, 10)
	td := newTraces(
		withDebugSpan("debug-span-1", true),
		withDebugSpan("debug-span-2", true),
		withPlainSpan("prod-span-1"),
		withPlainSpan("prod-span-2"),
		withPlainSpan("prod-span-3"),
	)

	result := proc.ProcessTraces(context.Background(), td)

	if result.DebugSpanCount != 2 {
		t.Errorf("DebugSpanCount = %d, want 2", result.DebugSpanCount)
	}
	if result.ProductionSpanCount != 3 {
		t.Errorf("ProductionSpanCount = %d, want 3", result.ProductionSpanCount)
	}
}

func TestProcessTraces_EmptyTraces(t *testing.T) {
	proc := newTestProcessor(t, 10)
	td := ptrace.NewTraces()

	result := proc.ProcessTraces(context.Background(), td)

	if result.DebugSpanCount != 0 || result.ProductionSpanCount != 0 || result.DroppedSpanCount != 0 {
		t.Errorf("empty traces should produce zero counts, got debug=%d prod=%d dropped=%d",
			result.DebugSpanCount, result.ProductionSpanCount, result.DroppedSpanCount)
	}
}

func TestProcessTraces_RateLimitDropsExcessDebugSpans(t *testing.T) {
	proc := newTestProcessor(t, 3) // allow only 3 per minute

	// Send 5 debug spans.
	td := newTraces(
		withDebugSpan("span-1", true),
		withDebugSpan("span-2", true),
		withDebugSpan("span-3", true),
		withDebugSpan("span-4", true),
		withDebugSpan("span-5", true),
	)

	result := proc.ProcessTraces(context.Background(), td)

	if result.DebugSpanCount != 3 {
		t.Errorf("DebugSpanCount = %d, want 3 (rate limit)", result.DebugSpanCount)
	}
	if result.DroppedSpanCount != 2 {
		t.Errorf("DroppedSpanCount = %d, want 2 (rate limited)", result.DroppedSpanCount)
	}
}

func TestProcessTraces_RateLimitDoesNotAffectProduction(t *testing.T) {
	proc := newTestProcessor(t, 1) // very tight rate limit

	td := newTraces(
		withDebugSpan("debug-span", true),
		withDebugSpan("debug-span-2", true), // this one gets dropped
		withPlainSpan("prod-span-1"),
		withPlainSpan("prod-span-2"),
		withPlainSpan("prod-span-3"),
	)

	result := proc.ProcessTraces(context.Background(), td)

	// Rate limit should only affect debug spans, not production.
	if result.ProductionSpanCount != 3 {
		t.Errorf("ProductionSpanCount = %d, want 3 (rate limit must not affect production)", result.ProductionSpanCount)
	}
	if result.DebugSpanCount != 1 {
		t.Errorf("DebugSpanCount = %d, want 1", result.DebugSpanCount)
	}
	if result.DroppedSpanCount != 1 {
		t.Errorf("DroppedSpanCount = %d, want 1", result.DroppedSpanCount)
	}
}

func TestProcessTraces_SpanContextPreserved(t *testing.T) {
	proc := newTestProcessor(t, 10)

	// Create a trace with resource and scope attributes.
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "billing-service")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("unsample-test")
	ss.Scope().SetVersion("1.0.0")

	span := ss.Spans().AppendEmpty()
	span.SetName("process-payment")
	span.Attributes().PutBool("debug.trace", true)

	result := proc.ProcessTraces(context.Background(), td)

	// Verify resource attributes are preserved.
	debugRS := result.Debug.ResourceSpans().At(0)
	svcName, exists := debugRS.Resource().Attributes().Get("service.name")
	if !exists || svcName.Str() != "billing-service" {
		t.Errorf("service.name = %q, want %q", svcName.Str(), "billing-service")
	}

	// Verify scope is preserved.
	debugSS := debugRS.ScopeSpans().At(0)
	if debugSS.Scope().Name() != "unsample-test" {
		t.Errorf("scope name = %q, want %q", debugSS.Scope().Name(), "unsample-test")
	}

	// Verify span name is preserved.
	debugSpan := debugSS.Spans().At(0)
	if debugSpan.Name() != "process-payment" {
		t.Errorf("span name = %q, want %q", debugSpan.Name(), "process-payment")
	}
}

// --- Config Tests ---

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid defaults", *DefaultConfig(), false},
		{"empty attribute", Config{DebugAttribute: "", MaxPerMinute: 10}, true},
		{"zero rate", Config{DebugAttribute: "debug.trace", MaxPerMinute: 0}, true},
		{"negative rate", Config{DebugAttribute: "debug.trace", MaxPerMinute: -1}, true},
		{"custom attribute", Config{DebugAttribute: "custom.debug", MaxPerMinute: 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Rate Limiter Tests ---

func TestRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := NewRateLimiter(3)
	for i := 0; i < 3; i++ {
		if !rl.Allow() {
			t.Errorf("Allow() = false on call %d, want true (within limit)", i+1)
		}
	}
}

func TestRateLimiter_DenyExceedingLimit(t *testing.T) {
	rl := NewRateLimiter(2)
	rl.Allow() // 1
	rl.Allow() // 2

	if rl.Allow() {
		t.Error("Allow() = true on call 3, want false (exceeded limit of 2)")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := NewRateLimiter(1)
	rl.Allow() // use up the quota

	if rl.Allow() {
		t.Error("should be rate limited before window reset")
	}

	// Simulate time advancing past the window.
	rl.nowFunc = func() time.Time {
		return time.Now().Add(61 * time.Second)
	}

	if !rl.Allow() {
		t.Error("Allow() = false after window reset, want true")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100)
	done := make(chan bool, 200)

	for i := 0; i < 200; i++ {
		go func() {
			rl.Allow()
			done <- true
		}()
	}

	for i := 0; i < 200; i++ {
		<-done
	}
	// If we get here without a race condition, the test passes.
	// The -race flag will catch any data races.
}

// --- Benchmarks ---

// BenchmarkProcessTraces_NoDebugSpans benchmarks processing a batch of
// production-only spans (the common case). Target: zero debug-path overhead.
func BenchmarkProcessTraces_NoDebugSpans(b *testing.B) {
	proc, _ := NewProcessor(DefaultConfig(), zap.NewNop())
	td := newTraces(
		withPlainSpan("span-1"),
		withPlainSpan("span-2"),
		withPlainSpan("span-3"),
	)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		proc.ProcessTraces(context.Background(), td)
	}
}

// BenchmarkProcessTraces_AllDebugSpans benchmarks processing a batch of
// debug spans (worst-case for the processor).
func BenchmarkProcessTraces_AllDebugSpans(b *testing.B) {
	proc, _ := NewProcessor(&Config{
		DebugAttribute: "debug.trace",
		MaxPerMinute:   1_000_000, // effectively unlimited for benchmark
	}, zap.NewNop())
	td := newTraces(
		withDebugSpan("span-1", true),
		withDebugSpan("span-2", true),
		withDebugSpan("span-3", true),
	)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		proc.ProcessTraces(context.Background(), td)
	}
}

// --- Test Helpers ---

func newTestProcessor(t *testing.T, maxPerMinute int) *Processor {
	t.Helper()
	proc, err := NewProcessor(&Config{
		DebugAttribute: "debug.trace",
		MaxPerMinute:   maxPerMinute,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}
	return proc
}

// spanOption is a function that adds a span to the first ScopeSpans in a Traces.
type spanOption func(ss ptrace.ScopeSpans)

func withDebugSpan(name string, debugValue bool) spanOption {
	return func(ss ptrace.ScopeSpans) {
		span := ss.Spans().AppendEmpty()
		span.SetName(name)
		span.Attributes().PutBool("debug.trace", debugValue)
	}
}

func withPlainSpan(name string) spanOption {
	return func(ss ptrace.ScopeSpans) {
		span := ss.Spans().AppendEmpty()
		span.SetName(name)
	}
}

func newTraces(opts ...spanOption) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for _, opt := range opts {
		opt(ss)
	}
	return td
}
