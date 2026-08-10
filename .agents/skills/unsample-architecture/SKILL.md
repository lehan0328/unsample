---
name: unsample-architecture
description: Architecture patterns, component specifications, data flow, and design decisions for the Unsample CLI, SDK interceptor, and OTel Collector processor. Covers system architecture, token lifecycle, span routing, and deployment topology. Reference before adding new components, modifying the data flow, or making architectural decisions.
---

# Unsample Architecture

## System Overview

Unsample has 3 components that work together:

```
CLI (developer machine)
  → Generates HMAC token, injects into W3C baggage
  → Sends HTTP request, polls for trace, outputs deep link

SDK Interceptor (installed in each microservice)
  → Reads baggage, verifies HMAC token
  → Overrides sampler to AlwaysOn
  → Copies debug flag to span attribute: debug.trace=true
  → Propagates baggage to downstream calls

Collector Processor (OTel Collector plugin)
  → Stateless per-span routing (NO groupbytrace)
  → Checks span.attributes["debug.trace"]
  → Routes debug spans to separate backend (7-day TTL)
  → Rate limits: max N debug traces/min
```

## Data Flow

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
If true → route to debug pipeline (separate backend, 7-day TTL)
If false → route to production pipeline (normal sampling, 30-day TTL)
```

### Why Two Layers (SDK + Collector)?

- **SDK only**: Bad actor floods the network with debug traces that are ultimately dropped at Collector = wasted bandwidth
- **Collector only**: Upstream services may have already sampled-out spans before reaching Collector → partial/broken traces
- **Both (defense in depth)**: SDK ensures spans are created; Collector ensures they're routed correctly

### Why Span Attributes, Not Headers?

By the time telemetry reaches the OTel Collector, transport headers (like `baggage`) are stripped. The Collector operates on `pdata.Traces` — structured span data only. The SDK interceptor bridges this gap by copying `baggage` → `span.attributes["debug.trace"]`.

## Token Lifecycle

```
Token format: <trace_id>:<unix_timestamp>:<hmac_base64url>

CLI generates:
  1. trace_id = random 128-bit hex (W3C format)
  2. timestamp = current unix time
  3. payload = trace_id + ":" + timestamp
  4. signature = HMAC-SHA256(secret, payload)
  5. token = trace_id + ":" + timestamp + ":" + base64url(signature)

Verification (at SDK and Collector):
  1. Parse token into 3 parts
  2. Check timestamp not expired (max 2h)
  3. Check timestamp not from future (1 min tolerance)
  4. Recompute HMAC and constant-time compare
```

## Project Structure

```
unsample/
├── cmd/unsample/           # CLI entry point (Cobra)
├── internal/
│   ├── cli/                # CLI command implementations
│   ├── token/              # HMAC token generation + verification
│   ├── trace/              # Trace poller + summary formatter
│   ├── collector/          # OTel Collector processor plugin
│   └── version/            # Build version (ldflags)
├── sdk/go/                 # Public Go SDK (middleware + interceptor)
├── examples/demo-app/      # 3-service demo application
├── docker/                 # Docker Compose + Collector config
├── docs/                   # Public documentation
└── .agents/                # Project rules + skills
```

## Component Boundaries

| Package | Public? | Can import from |
|---|---|---|
| `cmd/unsample` | Binary | `internal/*` |
| `internal/token` | No | stdlib only |
| `internal/cli` | No | `internal/token`, `internal/trace` |
| `internal/collector` | No | `internal/token`, OTel Collector SDK |
| `internal/trace` | No | stdlib, HTTP client |
| `sdk/go` | **Yes** | OTel SDK, `crypto/*` (NO `internal/` imports) |

> **Critical**: `sdk/go/` is a separate Go module published independently. It must NOT import from `internal/`. Token verification logic is duplicated (or extracted to a shared public package if needed).

## Deployment Topology

### Local Development
```
docker-compose up → Collector + Tempo (7d) + Jaeger UI
unsample debug <url> → trace appears in Jaeger
```

### Production
```
Developer laptop → unsample debug <url>
  → hits Service A (with SDK middleware)
  → propagates to Service B, C (all with SDK middleware)
  → spans flow to OTel Collector (DaemonSet, with unsample processor)
  → debug spans → Tempo-debug (S3, 7-day TTL)
  → production spans → Tempo-prod (S3, 30-day TTL)
```

## CLI Output Spec

```bash
$ unsample debug https://api.myapp.com/checkout?user=123

─── HTTP Response ───────────────────────────────
HTTP/1.1 500 Internal Server Error
{"error": "subscription_not_found"}

─── Debug Trace ─────────────────────────────────
⏳ Waiting for trace (4bf92f3577b34da6)...
✅ Trace captured (5 spans, 847ms)

   → http://localhost:16686/trace/4bf92f3577b34da6

   gateway          12ms  ✅ 200
   billing-service  340ms ❌ 500  subscription_not_found
     └─ postgres    312ms       SELECT * FROM subscriptions...
   notification      8ms  ⏭ skipped
─────────────────────────────────────────────────
```

**Sequence**: HTTP response → spinner (poll backend) → deep link + summary.
**Never**: terminal ASCII waterfall (unreadable for 50+ span debug traces).

## References

- [Design Doc](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/unsample_design_doc.md)
- [Design Lessons (Sherlog)](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/unsample_design_lessons.md)
- [Implementation Plan](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/implementation_plan.md)
