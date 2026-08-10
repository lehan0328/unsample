# Unsample — Design Lessons from Internal Sherlog Review

> Applied insights from Google's internal Sherlog architecture to Unsample's design.
> All guidance is based on general architectural patterns — no proprietary code.

---

## 7 Critical Design Changes Based on Sherlog's Architecture

### 1. ❌ Don't Trust a Bare `debug=true` Flag

**Before (naive design):**
```
baggage: unsample-debug=true
→ Collector sees it → bypasses sampling → done
```

**After (Sherlog-informed):**
```
baggage: unsample-debug=<HMAC-signed-token>
→ Collector verifies HMAC against shared secret
→ Only then bypasses sampling
```

**Why:** Without verification, anyone can add `debug=true` to any request and flood your trace backend. Sherlog uses cryptographically signed opt-in tokens. For Unsample v1, use a simple HMAC with a shared secret configured in the Collector YAML.

```yaml
# Collector config
processors:
  unsample:
    secret: "your-shared-secret-here"  # HMAC verification key
    max_debug_traces_per_minute: 10    # Rate limit
```

```bash
# CLI generates the signed token
$ unsample debug https://api.myapp.com/checkout
# Internally: HMAC-SHA256(secret, timestamp + trace_id) → token
# Injects: baggage: unsample-debug=<token>
```

---

### 2. Defense in Depth: Override Sampler at BOTH SDK and Collector

**Sherlog's pattern:**
- SDK level: Check debug flag BEFORE evaluating any logging/tracing → O(1) bitmask check
- Collector level: Verify token and enforce quotas

**Unsample implementation:**

```
Layer 1 (SDK/Interceptor):
  → Check baggage for unsample-debug token
  → If present: override local Sampler to AlwaysOn for this context
  → This ensures ALL downstream spans are created, even if head sampling was off

Layer 2 (Collector Processor):
  → Verify HMAC token
  → Check rate limits (max N debug traces/min)
  → Route verified debug traces to separate backend
  → Drop unverified debug-flagged traces
```

**Why both layers:**
- SDK-only: Bad actor floods the network (traces sent but dropped at Collector = waste)
- Collector-only: Upstream services may have already sampled-out the spans before they reach the Collector → partial/broken traces

---

### 3. Handle Partially Sampled Traces

**The problem:** Service A has `sampled=false`. The debug request reaches Service B. Service B sees the debug flag. But Service A's spans are already gone.

**Sherlog's solution:** The debug flag OVERRIDES the sampling bit at every service entry point.

**Unsample implementation:**
- The OTel SDK interceptor at each service must:
  1. Check incoming baggage for `unsample-debug` token
  2. If present → set `ParentBased(AlwaysOn)` sampler for this trace context
  3. Propagate the debug baggage to ALL outgoing requests
  
- Result: Even if the root service was sampled-out, all downstream services that receive the debug flag will create and export their spans → you get a partial trace from the debug-flagged service onwards

> [!IMPORTANT]
> **This means Unsample needs a lightweight SDK component (interceptor/middleware), not just a Collector processor.** This is an architecture change from the original "just a Collector processor" design.

---

### 4. Separate Storage Backend with Short TTL

**Sherlog's pattern:** Debug traces stored in dedicated tables, 6-day TTL. Never mixed with production metrics/traces.

**Unsample implementation:**

```yaml
# Collector pipeline — two separate pipelines
service:
  pipelines:
    traces/production:
      receivers: [otlp]
      processors: [tail_sampling, batch]
      exporters: [tempo-production]  # 30-90 day retention
    
    traces/debug:
      receivers: [otlp]
      processors: [unsample_filter, batch]
      exporters: [tempo-debug]  # 7-day retention, separate instance
```

**Why separate:**
- Debug traces contain potentially sensitive payloads (request/response bodies)
- Volume spikes are unpredictable (depends on developer behavior)
- Short TTL keeps storage costs near-zero
- Isolates debug traffic from production monitoring

---

### 5. Skip Async Queue Stitching in v1

**Sherlog's approach:** Uses a `DataID` association model — publisher attaches `Output DataID = <message_id>`, consumer attaches `Input DataID = <message_id>`. Backend stitches them via graph traversal at query time.

**Unsample v1:** Skip this entirely. Focus on synchronous HTTP/gRPC only.

**Why:**
- Async stitching requires a custom query engine (not just a trace viewer)
- OTel's context propagation across Kafka/SQS is already fragile
- 80% of debugging pain is synchronous request flow
- Add async support in v2 after proving the core value

**v1 scope:**
```
✅ Sync HTTP requests (REST APIs)
✅ Sync gRPC calls
✅ Database queries (as child spans)
❌ Kafka/SQS/PubSub message flows (v2)
❌ WebSocket streams (v2)
❌ Batch/cron jobs (v2)
```

---

### 6. Performance: Sub-ms Hot Path When Debug is OFF

**Sherlog's mandate:** Debug flag check must be O(1) and skip ALL serialization if false.

**Unsample implementation:**
```go
// In the OTel interceptor — this runs on EVERY request
func (m *UnsampleMiddleware) Handle(ctx context.Context, req *http.Request) {
    // O(1) check — just read one baggage key
    token := baggage.FromContext(ctx).Member("unsample-debug").Value()
    
    if token == "" {
        // Fast path: 99.99% of requests take this path
        // NO additional work, NO serialization, NO allocation
        next.ServeHTTP(w, req)
        return
    }
    
    // Slow path: only debug-flagged requests
    if !verifyHMAC(token, m.secret) {
        next.ServeHTTP(w, req) // Invalid token, ignore
        return
    }
    
    // Override sampler for this context
    ctx = overrideSampler(ctx, AlwaysOn)
    next.ServeHTTP(w, req.WithContext(ctx))
}
```

**Key principle:** The debug system must have ZERO observable cost when nobody is debugging. This is non-negotiable.

---

### 7. Don't Store Payloads in Span Attributes

**Sherlog's pattern:** Separates graph structure (spans/processors) from event payloads (logs). Spans stay lightweight.

**Unsample implementation:**
- Capture request/response bodies as **OTel Log Records** correlated to the `span_id` and `trace_id`
- NOT as span attributes (OTel has strict attribute size limits)
- This keeps spans fast to query while still preserving full debug payloads

```
Span: POST /checkout → 847ms → billing-service → 340ms
  └─ Linked LogRecord: { request_body: {...}, response_body: {...} }
```

---

## Updated Architecture Diagram

```
Developer's terminal
     │
     │  $ unsample debug https://api.myapp.com/checkout
     │
     ▼
[Unsample CLI]
     │  1. Generate HMAC token = sign(secret, timestamp)
     │  2. Inject baggage: unsample-debug=<token>
     │  3. Send HTTP request
     ▼
[API Gateway / Service A]
     │  Unsample SDK Interceptor:
     │    → Read baggage "unsample-debug"
     │    → Verify HMAC (O(1))
     │    → Override sampler to AlwaysOn
     │    → Capture req/res bodies as LogRecords
     │    → Propagate debug baggage downstream
     │
     ├──► [Service B] ──► [Service C]
     │    (same interceptor at each service)
     ▼
[OTel Collector]
     │
     ├─► [Unsample Processor]
     │     → Verify HMAC token (again, defense in depth)
     │     → Check rate limit (≤10 debug traces/min)
     │     → Route to debug pipeline
     │
     ├─► traces/debug pipeline → [Tempo-Debug] (7-day TTL)
     └─► traces/production pipeline → [Tempo-Prod] (30-day TTL)
     
     ▼
[Unsample Trace Viewer]
     → Developer sees full trace + request/response bodies
     → Link: http://localhost:3000/trace/<trace-id>
```

---

## v1 MVP Scope (Revised)

| Component | Include? | Notes |
|---|---|---|
| **CLI** (Go, sends HTTP request with signed debug baggage) | ✅ v1 | Core product |
| **SDK Interceptor** (Go middleware, overrides sampler) | ✅ v1 | **NEW** — wasn't in original design. Critical for partial-trace handling. |
| **Collector Processor** (Go, verifies token, routes to debug pipeline) | ✅ v1 | Core product |
| **Shared secret HMAC** (simple abuse prevention) | ✅ v1 | Lightweight, no JWT infrastructure needed |
| **Rate limiting** (max N debug traces/min) | ✅ v1 | Essential safety valve |
| **Separate Tempo-debug backend** | ✅ v1 | Docker compose for local dev |
| **Simple trace viewer** (link to existing Jaeger/Tempo UI) | ✅ v1 | Don't build custom UI |
| JWT/PKI token infrastructure | ❌ v2 | Overkill for v1 |
| Async queue stitching (Kafka/SQS) | ❌ v2 | Complex, skip for now |
| Request/response body capture as LogRecords | ❌ v2 | Nice to have, not critical |
| Custom trace viewer UI | ❌ v3 | Use Jaeger/Tempo UI for now |
| AI trace summary | ❌ v3 | Differentiation feature, not core |

---

## Key Risks Sherlog Flagged

| Risk | Severity | Mitigation |
|---|---|---|
| Debug flag stripped by API gateways/proxies | 🔴 High | Document known proxy configurations (Nginx, Envoy, Istio). Provide troubleshooting guide. |
| Unauthenticated debug flooding | 🔴 High | HMAC-signed tokens + rate limiting in Collector |
| Partial traces when upstream already sampled out | 🟡 Medium | SDK interceptor overrides sampler at each service boundary |
| Memory pressure from debug trace buffering | 🟡 Medium | Batch flush with short intervals. Don't buffer waiting for trace completion. |
| Developers treating debug traces like analytics | 🟢 Low | UI guidance: "This is for single-trace investigation, not aggregation" |

---

## Collector Processor Implementation (from Follow-up #1)

### 🔴 THE OOM TRAP: Never Use `groupbytrace`

> [!CAUTION]
> **Do NOT use OTel's `groupbytrace` + `tail_sampling` processors for debug routing.**
> This WILL crash your Collector in production.

**The tempting (wrong) approach:**
```yaml
# ❌ WRONG — will OOM and crash
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

**Why it crashes:** `groupbytrace` buffers ALL spans in memory until the trace completes or times out. A debug-flagged request that fans out to 50 services, each emitting thick spans with payloads, sends tens of thousands of spans simultaneously. The Collector runs out of RAM → OOM kill → your entire observability pipeline goes down.

**The correct (stateless) approach:**
```yaml
# ✅ CORRECT — stateless per-span routing, O(1) memory
processors:
  unsample_router:
    # Evaluates each span individually
    # No buffering, no waiting for trace completion
    debug_attribute: "debug.trace"
    debug_value: "true"

service:
  pipelines:
    traces/debug:
      receivers: [otlp]
      processors: [unsample_router]
      exporters: [otlp/tempo-debug]    # 7-day TTL
    
    traces/production:
      receivers: [otlp]
      processors: [probabilistic_sampler]
      exporters: [otlp/tempo-prod]     # 30-day TTL
```

**The key insight:** Because the SDK interceptor injects `debug.trace=true` into EVERY span (via baggage propagation), the routing decision is deterministic per-span. No need to buffer the whole trace to decide.

---

### SDK → Span Attribute Propagation Pattern

**Problem:** By the time telemetry reaches the OTel Collector, transport headers (like `baggage`) are stripped. The Collector only sees `pdata.Traces` — structured span data.

**Solution:** The SDK interceptor must copy the baggage value into a span attribute:

```go
// SDK Interceptor — runs at each service entry point
func (m *UnsampleInterceptor) HandleRequest(ctx context.Context) context.Context {
    token := baggage.FromContext(ctx).Member("unsample-debug").Value()
    if token == "" {
        return ctx // Fast path — no debug flag
    }
    
    if !verifyHMAC(token, m.secret) {
        return ctx // Invalid token — ignore
    }
    
    // 1. Override sampler for this request's trace context
    ctx = overrideSamplerToAlwaysOn(ctx)
    
    // 2. Set span attribute so the Collector can route it
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(attribute.Bool("debug.trace", true))
    
    // 3. Propagate debug baggage to all outgoing requests
    //    (automatic via OTel baggage propagation)
    
    return ctx
}
```

**The flow:**
```
CLI injects baggage: unsample-debug=<HMAC-token>
         ↓
SDK interceptor reads baggage → verifies HMAC
         ↓
SDK copies to span attribute: debug.trace=true
         ↓
SDK overrides sampler: AlwaysOn for this trace
         ↓
Spans exported to Collector with debug.trace=true attribute
         ↓
Collector processor: O(1) check on span.attributes["debug.trace"]
         ↓
If true → route to debug pipeline (separate backend)
If false → route to production pipeline (normal sampling)
```

---

### Collector Processor: Go Implementation Skeleton

```go
// processor.go — the core routing logic
func (p *UnsampleProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
    debugTraces := pdata.NewTraces()
    prodTraces := pdata.NewTraces()
    
    // Iterate resource spans
    for i := 0; i < td.ResourceSpans().Len(); i++ {
        rs := td.ResourceSpans().At(i)
        for j := 0; j < rs.ScopeSpans().Len(); j++ {
            ss := rs.ScopeSpans().At(j)
            for k := 0; k < ss.Spans().Len(); k++ {
                span := ss.Spans().At(k)
                
                // O(1) attribute check — no buffering, no grouping
                val, exists := span.Attributes().Get("debug.trace")
                if exists && val.Bool() {
                    // Route to debug pipeline
                    appendSpan(debugTraces, rs, ss, span)
                } else {
                    // Route to production pipeline
                    appendSpan(prodTraces, rs, ss, span)
                }
            }
        }
    }
    
    // Export to respective pipelines
    if debugTraces.SpanCount() > 0 {
        p.debugExporter.ConsumeTraces(ctx, debugTraces)
    }
    if prodTraces.SpanCount() > 0 {
        p.prodExporter.ConsumeTraces(ctx, prodTraces)
    }
    return nil
}
```

**Properties of this design:**
- ✅ **O(1) memory** — no buffering, no `groupbytrace`
- ✅ **Stateless** — each span evaluated independently
- ✅ **Constant overhead** — same cost whether 0 or 10,000 debug spans
- ✅ **No OOM risk** — spans flow through, never accumulate

---

## CLI Design (from Follow-up #2)

### 🔴 Don't Build a `curl` Replacement

> [!WARNING]
> **Sherlog evolved AWAY from "send a manual request" toward intercepting natural traffic.**
> Developers struggle to hand-craft complex requests. They miss auth tokens, cookies, proxy headers — often the exact things causing the bug.

**Two CLI modes instead:**

#### Mode 1: Proxy Mode (Primary — for reproducing bugs)
```bash
$ unsample proxy --target api.staging.com --port 8080

🔍 Unsample proxy running on localhost:8080
   → Forwarding to api.staging.com
   → Debug headers auto-injected on all traffic
   
   Point your browser/app at localhost:8080 and reproduce the bug.
   Press Ctrl+C to stop.
```

The developer uses their **real frontend/browser** pointed at `localhost:8080`. The CLI acts as a transparent proxy, injecting `baggage: unsample-debug=<token>` into ALL traffic passing through it. This guarantees perfect reproduction — same auth, same cookies, same headers.

#### Mode 2: Direct Request (Secondary — for quick API checks)
```bash
# Simple URL (like the original design)
$ unsample debug https://api.myapp.com/checkout

# Accept a curl string copy-pasted from Chrome DevTools
$ unsample debug --curl 'curl -X POST https://api.myapp.com/checkout \
    -H "Authorization: Bearer eyJ..." \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": 123}"'

# Accept a .http file (VS Code REST Client format)
$ unsample debug --file checkout.http
```

Accept curl strings so developers can copy-paste directly from Chrome Network tab → "Copy as cURL."

---

### CLI Output: Response → Spinner → Deep Link

> [!IMPORTANT]
> **Do NOT attempt terminal ASCII waterfall traces.** Debug traces are 50+ spans with thick payloads. They are unreadable in a terminal. Sherlog engineers exclusively use a Web UI.

**The correct output sequence:**

```bash
$ unsample debug https://api.myapp.com/checkout?user=123

# Step 1: Show the actual HTTP response (did the bug reproduce?)
HTTP/1.1 500 Internal Server Error
Content-Type: application/json
X-Request-Id: req-abc-123

{"error": "subscription_not_found", "user_id": 123}

# Step 2: Spinner while waiting for trace to be indexed
⏳ Waiting for trace to be indexed in Tempo... (trace_id: 4bf92f3577b34da6)
   ████████████░░░░░░░░ 12/~15 spans received

# Step 3: Deep link when ready
✅ Full trace captured (15 spans, 847ms)

   → View trace: http://localhost:16686/trace/4bf92f3577b34da6
   
   Summary:
   ├─ api-gateway     12ms  ✅
   ├─ auth-service    34ms  ✅
   ├─ billing-service 340ms ❌ 500 "subscription_not_found"
   │  └─ postgres     312ms    SELECT * FROM subscriptions WHERE user_id=123
   └─ notification    8ms   ⏭ (skipped, upstream failed)
```

**Why this sequence:**

| Step | What it does | Why it matters |
|---|---|---|
| **HTTP response first** | Shows if the bug actually reproduced | Developers need to know before investigating the trace |
| **Spinner with poll** | Polls Tempo/Jaeger API for the trace_id | Avoids "Trace Not Found" 404 when clicking too early |
| **Deep link** | One-click to full trace in Jaeger/Tempo UI | Where the real investigation happens |
| **Inline summary** | Lightweight text summary (not a waterfall) | Quick orientation: which service failed? |

---

### v1 CLI Scope (Revised)

| Feature | Include? | Notes |
|---|---|---|
| `unsample debug <url>` (direct request) | ✅ v1 | Simplest entry point |
| `unsample debug --curl '<string>'` | ✅ v1 | Copy-paste from Chrome DevTools |
| HTTP response display | ✅ v1 | Essential for reproduction confirmation |
| Spinner + trace polling | ✅ v1 | Prevents "trace not found" UX |
| Deep link to Jaeger/Tempo | ✅ v1 | Core output |
| Lightweight inline summary | ✅ v1 | Quick orientation (service + status + timing) |
| `unsample proxy` (transparent proxy mode) | ❌ v2 | Higher value but more complex to build |
| `unsample debug --file request.http` | ❌ v2 | Nice to have |
| Terminal ASCII waterfall | ❌ Never | Unreadable for thick debug traces |

---

## Production Incidents from Sherlog (Follow-up #3)

> [!CAUTION]
> These are real production incidents from Google's Sherlog infrastructure. Each one WILL happen to Unsample if not proactively mitigated.

---

### Incident 1: The Retry Storm DoS

**What happened:** A developer debug-traced a request that fanned out to thousands of downstream batch queries. The backend rate-limited the flood (`THROTTLED`). But the SDK treated the throttle as a transient error and **aggressively retried** — turning the debug infrastructure into a self-inflicted DDoS attack on production servers.

**Unsample safeguard:**

```go
// SDK Interceptor — MUST distinguish between errors and throttles
func (e *UnsampleExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
    err := e.client.UploadTraces(ctx, spans)
    
    if isThrottleResponse(err) {
        // ⚠️ NEVER RETRY on throttle — drop immediately
        log.Warn("Unsample: debug trace throttled, dropping spans (not retrying)")
        return nil // Return nil so the SDK doesn't retry
    }
    
    return err // Only retry on genuine network errors
}
```

**Rule:** If the Collector or backend says "rate limited" → the SDK MUST permanently drop that payload. No buffering, no retry queue. Production health > trace completeness.

---

### Incident 2: Stack Overflow from Recursive Payloads

**What happened:** A developer debug-traced a request where the response body contained a deeply nested recursive protobuf structure. The Collector tried to parse/index the payload → blew past the 64KB thread stack size → **instant crash** → partial observability pipeline outage.

**Unsample safeguard:**

```go
// SDK Interceptor — truncate payloads BEFORE export
const (
    MaxBodyBytes   = 64 * 1024  // 64KB max per request/response body
    MaxNestDepth   = 10         // Max JSON/protobuf nesting depth
    MaxStringLen   = 4096       // Max string attribute length
)

func sanitizePayload(body []byte) []byte {
    if len(body) > MaxBodyBytes {
        return append(body[:MaxBodyBytes], []byte("... [TRUNCATED]")...)
    }
    // Also enforce nesting depth limits on JSON parsing
    return truncateNesting(body, MaxNestDepth)
}
```

**Rule:** ALL payload capture (request/response bodies, span attributes) must be structurally truncated at the SDK BEFORE hitting the network. The Collector should never parse untrusted, unbounded data structures.

---

### Incident 3: Hot-Path Latency from Context Propagation

**What happened:** Early Sherlog attached a thick debug context object to EVERY RPC — even for the 99.9% of requests not being debugged. This caused measurable CPU overhead, memory allocations, and latency regressions fleet-wide.

**Unsample safeguard:**

```go
// ❌ WRONG — thick context on every request
func middleware(ctx context.Context) {
    debugCtx := buildDebugContext()  // Allocates memory
    ctx = context.WithValue(ctx, debugKey, debugCtx)  // Copied on every RPC
    // ... even though 99.99% of requests aren't being debugged
}

// ✅ CORRECT — O(1) bitmask check, lazy evaluation
func middleware(ctx context.Context) {
    // Single string lookup — no allocation if empty
    token := baggage.FromContext(ctx).Member("unsample-debug").Value()
    if token == "" {
        return  // Sub-microsecond exit for 99.99% of traffic
    }
    
    // Only allocate debug context for actual debug requests
    debugCtx := buildDebugContext(token)
    ctx = context.WithValue(ctx, debugKey, debugCtx)
}
```

**Rule:** The debug check on the hot path must be:
- O(1) — single key lookup
- Zero-allocation when debug is OFF
- No serialization, no deep-copy, no protobuf parsing
- Sub-microsecond exit path

---

## Consolidated Safeguards Checklist

Every one of these MUST be implemented before Unsample touches production traffic:

| # | Safeguard | Where | Priority |
|---|---|---|---|
| 1 | **HMAC-signed debug tokens** (not bare `debug=true`) | CLI + Collector | ✅ v1 |
| 2 | **Rate limit** (max N debug traces/min per token) | Collector processor | ✅ v1 |
| 3 | **Never retry on throttle** — drop immediately | SDK interceptor | ✅ v1 |
| 4 | **Payload truncation** (64KB max body, 10 depth, 4KB strings) | SDK interceptor | ✅ v1 |
| 5 | **O(1) hot-path check** — zero-alloc when debug OFF | SDK interceptor | ✅ v1 |
| 6 | **Stateless per-span routing** — no `groupbytrace` | Collector processor | ✅ v1 |
| 7 | **Separate debug backend** with 7-day TTL | Collector pipeline | ✅ v1 |
| 8 | **Trace polling before deep link** — avoid 404 | CLI output | ✅ v1 |
| 9 | **Time-bound tokens** (expire after 2 hours) | CLI + Collector | ✅ v1 |



