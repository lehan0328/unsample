---
name: unsample-safety
description: Safety guardrails and production incident learnings derived from Google's internal Sherlog tracing infrastructure. Covers the 3 critical production incidents (retry storm DoS, stack overflow from recursive payloads, hot-path latency regression) and the 9 mandatory safeguards. Reference before modifying the SDK interceptor, Collector processor, or any code on the request hot path.
---

# Unsample Safety Guardrails

> These guardrails are derived from real production incidents at Google's Sherlog infrastructure.
> Every one is a **mandatory requirement** — not optional, not "nice to have."

## The 3 Incidents That Shaped This Design

### Incident 1: Retry Storm DoS

**What happened:** Developer debug-traced a fan-out endpoint (thousands of downstream queries). Backend rate-limited the flood. SDK treated throttle as transient error and aggressively retried → self-inflicted DDoS.

**Rule:** If Collector/backend says "rate limited" → SDK MUST permanently drop the payload. Return `nil`, not an error. No buffering, no retry queue.

```go
if isThrottleResponse(err) {
    log.Warn("debug trace throttled, dropping (not retrying)")
    return nil  // SDK won't retry on nil error
}
```

### Incident 2: Stack Overflow from Recursive Payloads

**What happened:** Debug trace captured a deeply nested recursive protobuf response body. Collector tried to parse it → stack overflow → crash → partial pipeline outage.

**Rule:** ALL payload capture must be truncated at the SDK BEFORE export:

```go
const (
    MaxBodyBytes = 64 * 1024  // 64KB per body
    MaxNestDepth = 10         // Max JSON nesting
    MaxStringLen = 4096       // Max string attribute
)
```

### Incident 3: Hot-Path Latency from Context Propagation

**What happened:** Early design attached thick debug context to EVERY RPC (even non-debug). Caused measurable CPU overhead and latency regression fleet-wide.

**Rule:** Debug check on the hot path must be:
- O(1) — single key lookup
- Zero-allocation when debug is OFF
- No serialization, no deep-copy
- Sub-microsecond exit

```go
token := baggage.FromContext(ctx).Member("unsample-debug").Value()
if token == "" {
    return  // 99.99% of requests exit here — zero cost
}
// Only allocate/verify for actual debug requests below this line
```

---

## 9 Mandatory Safeguards

| # | Safeguard | Where | Rationale |
|---|---|---|---|
| 1 | HMAC-signed tokens | CLI, SDK, Collector | Prevents unauthenticated debug flooding |
| 2 | Time-bound tokens (2h expiry) | CLI, SDK | Prevents token replay attacks |
| 3 | Rate limit (10 debug traces/min) | Collector | Prevents storage saturation from fan-out |
| 4 | Never retry on throttle | SDK | Prevents retry storm DoS (Incident #1) |
| 5 | Payload truncation (64KB, depth 10) | SDK | Prevents stack overflow crash (Incident #2) |
| 6 | O(1) hot-path check (zero-alloc) | SDK | Prevents latency regression (Incident #3) |
| 7 | Stateless per-span routing | Collector | Prevents OOM from trace buffering |
| 8 | Separate debug backend (7-day TTL) | Collector config | Cost isolation + PII separation |
| 9 | Trace polling before deep link | CLI | Prevents "Trace Not Found" 404 UX |

---

## Anti-Patterns to Avoid

### Never Use `groupbytrace` for Debug Routing

```yaml
# ❌ WRONG — will OOM crash the Collector
processors:
  groupbytrace:
    wait_duration: 10s
  tail_sampling:
    policies:
      - name: debug-traces
        type: string_attribute
        string_attribute:
          key: debug.trace
          values: ["true"]
```

**Why it crashes:** `groupbytrace` buffers ALL spans in memory until trace completes. A debug fan-out sends tens of thousands of thick spans → Collector OOMs → entire observability pipeline goes down.

**Correct approach:** Stateless per-span routing. Because the SDK injects `debug.trace=true` into EVERY span via baggage propagation, routing is deterministic per-span. No buffering needed.

### Never Store Thick Payloads in Span Attributes

OTel spans have strict attribute size limits. Capture request/response bodies as OTel LogRecords correlated to `span_id` + `trace_id`, not as span attributes.

### Never Build a Terminal ASCII Waterfall

Debug traces are 50+ spans with thick payloads. Terminal waterfalls are unreadable. Output a deep link to Jaeger/Tempo UI instead. Inline summary is OK (service + status + timing).

### Never Trust Bare `debug=true`

Without HMAC verification, anyone can add `debug=true` to any request and flood your trace backend.

---

## Collector Processor: Correct Implementation Pattern

```go
func (p *Processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
    // Iterate resource spans — stateless, O(1) memory
    for i := 0; i < td.ResourceSpans().Len(); i++ {
        rs := td.ResourceSpans().At(i)
        for j := 0; j < rs.ScopeSpans().Len(); j++ {
            ss := rs.ScopeSpans().At(j)
            for k := 0; k < ss.Spans().Len(); k++ {
                span := ss.Spans().At(k)
                
                // O(1) attribute check — no buffering, no grouping
                val, exists := span.Attributes().Get("debug.trace")
                if exists && val.Bool() {
                    // Route to debug pipeline (if within rate limit)
                    if p.rateLimit.Allow() {
                        copySpan(debugTraces, rs, ss, span)
                    }
                    // If rate limited: silently drop (NEVER retry)
                } else {
                    copySpan(prodTraces, rs, ss, span)
                }
            }
        }
    }
    return nil
}
```

## SDK Interceptor: Correct Hot-Path Pattern

```go
func Middleware(cfg Config) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // ─── HOT PATH ─────────────────────────────
            // O(1) check, zero allocation, sub-microsecond
            token := baggage.FromContext(r.Context()).Member("unsample-debug").Value()
            if token == "" {
                next.ServeHTTP(w, r) // 99.99% of requests
                return
            }
            
            // ─── COLD PATH (debug request) ────────────
            if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
                next.ServeHTTP(w, r) // Invalid token
                return
            }
            
            span := trace.SpanFromContext(r.Context())
            span.SetAttributes(attribute.Bool("debug.trace", true))
            next.ServeHTTP(w, r)
        })
    }
}
```

## References

- [Design Lessons (Full)](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/unsample_design_lessons.md)
- [Design Doc](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/unsample_design_doc.md)
