# Unsample — Engineering Design Document

> **On-demand debug tracing for OpenTelemetry.**
> Force full trace capture for any single request. No sampling. No Datadog bill.

**Version:** 1.0  
**Date:** 2026-08-09  
**Author:** Lehan Ouyang  
**Status:** Draft — Pending Approval

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Solution Overview](#2-solution-overview)
3. [System Architecture](#3-system-architecture)
4. [Component Specifications](#4-component-specifications)
5. [Data Flow](#5-data-flow)
6. [Security Model](#6-security-model)
7. [Safety Guardrails](#7-safety-guardrails)
8. [Configuration](#8-configuration)
9. [Project Structure](#9-project-structure)
10. [Tech Stack](#10-tech-stack)
11. [v1 Scope](#11-v1-scope)
12. [Development Milestones](#12-development-milestones)
13. [Deployment Topology](#13-deployment-topology)
14. [Open Questions](#14-open-questions)

---

## 1. Problem Statement

Companies using distributed tracing sample 1-5% of traces to control costs. When a developer needs to debug a production issue, the trace they need was in the 95% that got thrown away.

**Current workarounds (all bad):**

| Workaround | Cost |
|---|---|
| Bump sampling to 100% temporarily | $10K+ surprise APM invoice |
| Add console.log, redeploy | 15-30 min. Changes system behavior. |
| Reproduce in staging | Prod-only bugs can't be reproduced |
| Grep through logs | Slow. No cross-service flow visibility. |

**Target users:** Backend developers and SREs running microservices with OpenTelemetry + low sampling rates.

**Market validation:** 4 responses from Reddit validation posts confirmed the pain. One respondent independently built a custom forced-sampling header system internally, confirming both the problem and the approach.

---

## 2. Solution Overview

Unsample is three components:

```
┌─────────────────────────────────────────────────────────┐
│                      UNSAMPLE                            │
│                                                          │
│  ┌──────────┐    ┌──────────────────┐    ┌────────────┐ │
│  │   CLI    │    │ SDK Interceptor  │    │ Collector  │ │
│  │  (Go)    │    │ (Go middleware)  │    │ Processor  │ │
│  │          │    │                  │    │   (Go)     │ │
│  │ Sends    │    │ Reads baggage    │    │ Routes     │ │
│  │ request  │    │ Overrides sampler│    │ debug spans│ │
│  │ w/ debug │    │ Sets span attr   │    │ to separate│ │
│  │ token    │    │                  │    │ backend    │ │
│  └──────────┘    └──────────────────┘    └────────────┘ │
│                                                          │
│  Developer       Each microservice       OTel Collector  │
│  machine         (installed once)        (config change) │
└─────────────────────────────────────────────────────────┘
```

**Key design principles (from Sherlog review):**
- Debug flag carried via W3C `baggage` header
- HMAC-signed tokens prevent abuse (not bare `debug=true`)
- SDK interceptor overrides sampler at each service boundary (defense in depth)
- Collector processor routes per-span statelessly (no `groupbytrace` — avoids OOM)
- Separate debug backend with short TTL (7 days)
- O(1) hot-path cost when nobody is debugging

---

## 3. System Architecture

```
Developer's terminal
     │
     │  $ unsample debug https://api.myapp.com/checkout
     │
     ▼
┌──────────────┐
│ Unsample CLI │
│              │
│ 1. Generate HMAC token:
│    token = HMAC-SHA256(secret, trace_id + timestamp)
│ 2. Generate W3C trace_id
│ 3. Inject headers:
│    baggage: unsample-debug=<token>
│    traceparent: 00-<trace_id>-<span_id>-01
│ 4. Send HTTP request
│ 5. Display HTTP response
│ 6. Poll backend for trace
│ 7. Output deep link
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│                  Service Mesh                         │
│                                                       │
│  ┌─────────────┐   ┌─────────────┐   ┌────────────┐ │
│  │ Service A   │──▶│ Service B   │──▶│ Service C  │ │
│  │             │   │             │   │            │ │
│  │ Interceptor:│   │ Interceptor:│   │Interceptor:│ │
│  │ • Read bag. │   │ • Read bag. │   │• Read bag. │ │
│  │ • Verify    │   │ • Verify    │   │• Verify    │ │
│  │   HMAC      │   │   HMAC      │   │  HMAC      │ │
│  │ • Override  │   │ • Override  │   │• Override  │ │
│  │   sampler   │   │   sampler   │   │  sampler   │ │
│  │ • Set attr: │   │ • Set attr: │   │• Set attr: │ │
│  │   debug.    │   │   debug.    │   │  debug.    │ │
│  │   trace=T   │   │   trace=T   │   │  trace=T   │ │
│  │ • Propagate │   │ • Propagate │   │• Propagate │ │
│  │   baggage   │   │   baggage   │   │  baggage   │ │
│  └──────┬──────┘   └──────┬──────┘   └─────┬──────┘ │
│         │                 │                 │        │
└─────────┼─────────────────┼─────────────────┼────────┘
          │                 │                 │
          ▼                 ▼                 ▼
┌──────────────────────────────────────────────────────┐
│               OTel Collector                          │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │         Unsample Router Processor                │ │
│  │                                                  │ │
│  │  for each span:                                  │ │
│  │    if span.attr["debug.trace"] == true:          │ │
│  │      → route to debug pipeline                   │ │
│  │    else:                                         │ │
│  │      → route to production pipeline              │ │
│  │                                                  │ │
│  │  Properties:                                     │ │
│  │    • Stateless (no groupbytrace!)                │ │
│  │    • O(1) memory per span                        │ │
│  │    • Rate limited (max N debug traces/min)       │ │
│  └─────────────┬───────────────────┬────────────────┘ │
│                │                   │                  │
│                ▼                   ▼                  │
│  ┌──────────────────┐  ┌──────────────────────────┐  │
│  │ Debug Exporter   │  │ Production Exporter      │  │
│  │ → Tempo (7d TTL) │  │ → Tempo (30d TTL)        │  │
│  │   or Jaeger      │  │   or existing APM        │  │
│  └──────────────────┘  └──────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

---

## 4. Component Specifications

### 4.1 CLI (`unsample`)

**Language:** Go  
**Framework:** Cobra CLI  
**Distribution:** Single binary (cross-compiled for Linux, macOS, Windows)

#### Commands

```
unsample debug <url> [flags]       Send a debug-traced request
unsample debug --curl '<string>'   Parse and send a curl command
unsample init                      Generate Collector config + setup guide
unsample version                   Print version
```

#### `unsample debug` Flow

```go
func debugCmd(url string) {
    // 1. Load config (shared secret)
    secret := loadSecret()
    
    // 2. Generate trace identifiers
    traceID := generateTraceID()   // W3C 128-bit hex
    spanID  := generateSpanID()    // W3C 64-bit hex
    
    // 3. Generate HMAC token (time-bound)
    timestamp := time.Now().Unix()
    payload   := fmt.Sprintf("%s:%d", traceID, timestamp)
    token     := hmacSHA256(secret, payload)
    tokenStr  := fmt.Sprintf("%s:%d:%s", traceID, timestamp, 
                              base64.URLEncoding.EncodeToString(token))
    
    // 4. Build request with debug headers
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("traceparent", 
        fmt.Sprintf("00-%s-%s-01", traceID, spanID))
    
    member, _ := baggage.NewMember("unsample-debug", tokenStr)
    bag, _    := baggage.New(member)
    otel.GetTextMapPropagator().Inject(
        baggage.ContextWithBaggage(context.Background(), bag),
        propagation.HeaderCarrier(req.Header),
    )
    
    // 5. Send request
    resp, err := http.DefaultClient.Do(req)
    
    // 6. Display HTTP response
    printHTTPResponse(resp)
    
    // 7. Poll for trace (avoid 404)
    pollForTrace(traceID, tempoEndpoint, timeout=30*time.Second)
    
    // 8. Output deep link + summary
    printTraceLink(traceID, jaegerURL)
    printTraceSummary(traceID)  // lightweight: service, status, timing
}
```

#### CLI Output Spec

```bash
$ unsample debug https://api.myapp.com/checkout?user=123

─── HTTP Response ───────────────────────────────────────
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{"error": "subscription_not_found", "user_id": 123}

─── Debug Trace ─────────────────────────────────────────
⏳ Waiting for trace (4bf92f3577b34da6) to index...
   ████████████████████ 15/15 spans

✅ Trace captured (15 spans, 847ms)

   → http://localhost:16686/trace/4bf92f3577b34da6

   api-gateway      12ms  ✅ 200
   auth-service     34ms  ✅ 200
   billing-service  340ms ❌ 500  subscription_not_found
     └─ postgres    312ms       SELECT * FROM subscriptions...
   notification-svc   8ms ⏭ skipped
─────────────────────────────────────────────────────────
```

---

### 4.2 SDK Interceptor (`unsample-go`)

**Language:** Go (first), with Node.js and Python to follow  
**Type:** HTTP/gRPC middleware  
**Install:** `go get github.com/unsample/unsample-go`

#### Middleware Implementation

```go
package unsample

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "net/http"
    "strconv"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/baggage"
    "go.opentelemetry.io/otel/trace"
)

type Config struct {
    Secret         string        // HMAC shared secret
    TokenMaxAge    time.Duration // Max token age (default: 2h)
}

func Middleware(cfg Config) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()
            
            // ─── HOT PATH: O(1) check ───────────────────────
            // This runs on EVERY request. Must be sub-microsecond.
            token := baggage.FromContext(ctx).Member("unsample-debug").Value()
            if token == "" {
                // 99.99% of requests exit here
                // Zero allocation, zero serialization
                next.ServeHTTP(w, r)
                return
            }
            
            // ─── SLOW PATH: debug request ────────────────────
            // Only reached for debug-flagged requests
            
            // Verify HMAC token
            if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
                // Invalid or expired token — treat as normal request
                next.ServeHTTP(w, r)
                return
            }
            
            // Override sampler to AlwaysOn for this trace
            span := trace.SpanFromContext(ctx)
            span.SetAttributes(attribute.Bool("debug.trace", true))
            
            // Baggage propagation is automatic via OTel propagators
            // — all outgoing HTTP/gRPC calls will carry the debug baggage
            
            next.ServeHTTP(w, r)
        })
    }
}

// verifyToken checks HMAC signature and token expiry
func verifyToken(tokenStr, secret string, maxAge time.Duration) bool {
    // Token format: "<trace_id>:<timestamp>:<hmac_base64>"
    parts := splitToken(tokenStr)
    if len(parts) != 3 {
        return false
    }
    
    traceID   := parts[0]
    timestamp, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        return false
    }
    signature := parts[2]
    
    // Check expiry
    tokenTime := time.Unix(timestamp, 0)
    if time.Since(tokenTime) > maxAge {
        return false  // Token expired
    }
    
    // Verify HMAC
    payload := traceID + ":" + parts[1]
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(payload))
    expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

#### gRPC Interceptor

```go
func UnaryServerInterceptor(cfg Config) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, 
                info *grpc.UnaryServerInfo, 
                handler grpc.UnaryHandler) (interface{}, error) {
        
        token := baggage.FromContext(ctx).Member("unsample-debug").Value()
        if token == "" {
            return handler(ctx, req) // Hot path — zero cost
        }
        
        if !verifyToken(token, cfg.Secret, cfg.TokenMaxAge) {
            return handler(ctx, req)
        }
        
        span := trace.SpanFromContext(ctx)
        span.SetAttributes(attribute.Bool("debug.trace", true))
        
        return handler(ctx, req)
    }
}
```

#### Integration (User's Code)

```go
// 3 lines to add to each service
import "github.com/unsample/unsample-go"

router.Use(unsample.Middleware(unsample.Config{
    Secret: os.Getenv("UNSAMPLE_SECRET"),
}))
```

---

### 4.3 Collector Processor (`unsampleprocessor`)

**Language:** Go  
**Type:** OTel Collector processor plugin  
**Build:** Custom Collector distribution via OCB (OpenTelemetry Collector Builder)

#### Processor Logic

```go
package unsampleprocessor

import (
    "context"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

type processor struct {
    debugExporter  component.TracesExporter
    prodExporter   component.TracesExporter
    rateLimit      *rateLimiter
}

func (p *processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
    debugTraces := ptrace.NewTraces()
    prodTraces  := ptrace.NewTraces()
    
    for i := 0; i < td.ResourceSpans().Len(); i++ {
        rs := td.ResourceSpans().At(i)
        
        for j := 0; j < rs.ScopeSpans().Len(); j++ {
            ss := rs.ScopeSpans().At(j)
            
            for k := 0; k < ss.Spans().Len(); k++ {
                span := ss.Spans().At(k)
                
                // ─── O(1) stateless check per span ───
                val, exists := span.Attributes().Get("debug.trace")
                
                if exists && val.Bool() {
                    if p.rateLimit.Allow() {
                        copySpan(debugTraces, rs, ss, span)
                    }
                    // If rate limited: silently drop (never retry)
                } else {
                    copySpan(prodTraces, rs, ss, span)
                }
            }
        }
    }
    
    // Export to respective pipelines
    var errs []error
    if debugTraces.SpanCount() > 0 {
        errs = append(errs, p.debugExporter.ConsumeTraces(ctx, debugTraces))
    }
    if prodTraces.SpanCount() > 0 {
        errs = append(errs, p.prodExporter.ConsumeTraces(ctx, prodTraces))
    }
    return multierr.Combine(errs...)
}
```

#### Processor Config

```go
type Config struct {
    DebugAttribute string `mapstructure:"debug_attribute"`
    DebugValue     string `mapstructure:"debug_value"`
    MaxPerMinute   int    `mapstructure:"max_debug_traces_per_minute"`
}

func createDefaultConfig() component.Config {
    return &Config{
        DebugAttribute: "debug.trace",
        DebugValue:     "true",
        MaxPerMinute:   10,
    }
}
```

---

## 5. Data Flow

### 5.1 Token Lifecycle

```
CLI generates token:
  token = base64(HMAC-SHA256(secret, trace_id + ":" + timestamp))
  full_token = trace_id + ":" + timestamp + ":" + token
  expiry = timestamp + 2 hours
  
                    ┌──────────────────────────┐
                    │    Token: abc123:         │
                    │    1691625600:            │
                    │    dGhpcyBpcyBhIG...     │
                    └──────────┬───────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                 ▼
         Service A        Service B         Service C
         verify()         verify()          verify()
         ✅ valid         ✅ valid          ✅ valid
         set attr         set attr          set attr
              │                │                 │
              ▼                ▼                 ▼
         OTel Collector
         rate_limit()
         route to debug pipeline
```

### 5.2 Span Attribute Flow

```
                Transport Layer          Telemetry Layer
                (HTTP headers)           (OTel pdata)
                
CLI:            baggage: unsample-       (not yet in OTel)
                debug=<token>
                
SDK             reads baggage     ──→    span.attributes:
Interceptor:    from context             debug.trace = true
                
OTel            (headers stripped) ──→   reads span.attributes
Collector:                               debug.trace == true?
                                         → route to debug pipeline
```

> [!IMPORTANT]
> The Collector NEVER sees baggage headers. The SDK interceptor bridges the gap by copying the debug signal from baggage → span attribute.

---

## 6. Security Model

### 6.1 Threat Model

| Threat | Attack | Mitigation |
|---|---|---|
| **Debug flooding** | Attacker sends `debug=true` on every request | HMAC token — unsigned flags rejected |
| **Token replay** | Reuse old debug tokens to fill storage | Time-bound tokens (2h expiry) |
| **Rate saturation** | Valid user debugs fan-out endpoint | Rate limit: max 10 debug traces/min |
| **Trace backend DoS** | Massive debug payloads overwhelm backend | Payload truncation at SDK (64KB max) |
| **Retry storm** | SDK retries throttled debug payloads | Never retry on throttle — drop immediately |

### 6.2 Token Format

```
<trace_id>:<unix_timestamp>:<hmac_base64>

Example:
4bf92f3577b34da6a3ce929d0e0e4736:1691625600:dGhpcyBpcyBhIHNhbXBsZQ==
```

### 6.3 HMAC Verification

```
Shared secret configured in:
  - CLI: UNSAMPLE_SECRET env var or ~/.unsample/config.yaml
  - SDK: UNSAMPLE_SECRET env var or middleware config
  - Collector: processor config YAML

Verification at each layer:
  1. SDK interceptor: verify(token, secret) → reject invalid
  2. Collector processor: rate_limit(trace_id) → drop if exceeded
```

---

## 7. Safety Guardrails

> [!CAUTION]
> These are derived from real production incidents at Google. Each one is a **mandatory v1 requirement**.

| # | Guardrail | Component | What It Prevents |
|---|---|---|---|
| 1 | **HMAC-signed tokens** | CLI, SDK, Collector | Unauthenticated debug flooding |
| 2 | **Time-bound tokens** (2h expiry) | CLI, SDK | Token replay attacks |
| 3 | **Rate limit** (10 debug traces/min) | Collector | Storage saturation from fan-out |
| 4 | **Never retry on throttle** | SDK | Retry storm DoS (Sherlog Incident #1) |
| 5 | **Payload truncation** (64KB body, depth 10) | SDK | Stack overflow from recursive payloads (Sherlog Incident #2) |
| 6 | **O(1) hot-path check** (zero-alloc when OFF) | SDK | Hot-path latency regression (Sherlog Incident #3) |
| 7 | **Stateless per-span routing** (no `groupbytrace`) | Collector | OOM crash from trace buffering |
| 8 | **Separate debug backend** (7-day TTL) | Collector config | Cost isolation, PII separation |
| 9 | **Trace polling before deep link** | CLI | "Trace Not Found" 404 UX |

---

## 8. Configuration

### 8.1 CLI Config (`~/.unsample/config.yaml`)

```yaml
# Shared secret for HMAC token generation
secret: "your-shared-secret-here"

# Trace backend for polling
backend:
  type: tempo        # or "jaeger"
  endpoint: http://localhost:3200
  
# Trace viewer for deep links
viewer:
  type: jaeger       # or "grafana"
  url: http://localhost:16686
  
# Token settings
token:
  max_age: 2h        # Token expiry
```

### 8.2 SDK Interceptor Config

```go
unsample.Config{
    Secret:      os.Getenv("UNSAMPLE_SECRET"),  // Required
    TokenMaxAge: 2 * time.Hour,                  // Default: 2h
}
```

### 8.3 Collector Config (`otel-collector-config.yaml`)

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  # Unsample debug router
  unsample:
    debug_attribute: "debug.trace"
    debug_value: "true"
    max_debug_traces_per_minute: 10
  
  # Standard production sampling
  probabilistic_sampler:
    sampling_percentage: 5

  batch:

exporters:
  otlp/tempo-debug:
    endpoint: tempo-debug:4317
    tls:
      insecure: true
  
  otlp/tempo-prod:
    endpoint: tempo-prod:4317
    tls:
      insecure: true

service:
  pipelines:
    traces/debug:
      receivers: [otlp]
      processors: [unsample, batch]
      exporters: [otlp/tempo-debug]
    
    traces/production:
      receivers: [otlp]
      processors: [probabilistic_sampler, batch]
      exporters: [otlp/tempo-prod]
```

---

## 9. Project Structure

```
unsample/
├── cmd/
│   └── unsample/
│       └── main.go              # CLI entry point
│
├── internal/
│   ├── cli/
│   │   ├── debug.go             # `unsample debug` command
│   │   ├── init.go              # `unsample init` command
│   │   ├── config.go            # Config loading (~/.unsample/)
│   │   └── output.go            # HTTP response + trace display
│   │
│   ├── token/
│   │   ├── hmac.go              # HMAC token generation
│   │   ├── verify.go            # HMAC token verification
│   │   └── hmac_test.go         # Token tests
│   │
│   ├── trace/
│   │   ├── poller.go            # Poll Tempo/Jaeger for trace
│   │   ├── summary.go           # Lightweight trace summary
│   │   └── poller_test.go       # Poller tests
│   │
│   └── collector/
│       ├── processor.go         # Unsample Collector processor
│       ├── factory.go           # Processor factory
│       ├── config.go            # Processor config
│       ├── ratelimit.go         # Per-trace rate limiter
│       └── processor_test.go    # Processor tests
│
├── sdk/
│   └── go/
│       ├── middleware.go        # HTTP middleware
│       ├── grpc.go              # gRPC interceptor
│       ├── verify.go            # Token verification (shared)
│       └── middleware_test.go   # Middleware tests
│
├── docker/
│   ├── docker-compose.yaml      # Local dev: Collector + Tempo + Jaeger
│   └── otel-collector-config.yaml
│
├── docs/
│   ├── quickstart.md            # 5-minute setup guide
│   ├── architecture.md          # This design doc
│   └── troubleshooting.md       # Common issues (proxy stripping, etc.)
│
├── examples/
│   └── demo-app/                # Sample multi-service app
│       ├── service-a/
│       ├── service-b/
│       └── docker-compose.yaml
│
├── builder-config.yaml          # OCB manifest for custom Collector
├── go.mod
├── go.sum
├── LICENSE                      # MIT
├── README.md
└── Makefile
```

---

## 10. Tech Stack

| Layer | Technology | Rationale |
|---|---|---|
| **CLI** | Go + Cobra | Single binary. Matches OTel ecosystem. |
| **SDK Interceptor** | Go (net/http, gRPC) | First-class OTel SDK support. |
| **Collector Processor** | Go (OTel Collector SDK) | Required — Collector is written in Go. |
| **Custom Collector** | OCB (OTel Collector Builder) | Standard way to build custom distributions. |
| **Trace Storage (debug)** | Grafana Tempo | Cheapest OTel-native backend. Object storage (S3/local). |
| **Trace Viewer** | Jaeger UI or Grafana | Existing UIs — don't build custom. |
| **Local Dev** | Docker Compose | One command to run full stack locally. |
| **CI/CD** | GitHub Actions | Standard for OSS. |
| **Distribution** | Homebrew, `go install`, Docker | Max developer reach. |

---

## 11. v1 Scope

### In Scope

| Component | Feature | Priority |
|---|---|---|
| **CLI** | `unsample debug <url>` | P0 |
| **CLI** | `unsample debug --curl '<string>'` | P0 |
| **CLI** | HTTP response display | P0 |
| **CLI** | Trace polling + deep link | P0 |
| **CLI** | Lightweight inline summary | P1 |
| **CLI** | `unsample init` (generate configs) | P1 |
| **SDK** | Go HTTP middleware | P0 |
| **SDK** | Go gRPC interceptor | P1 |
| **SDK** | HMAC token verification | P0 |
| **SDK** | O(1) hot-path (zero-alloc when OFF) | P0 |
| **Collector** | Stateless span router | P0 |
| **Collector** | Rate limiting | P0 |
| **Infra** | Docker Compose (Collector + Tempo + Jaeger) | P0 |
| **Infra** | Demo multi-service app | P1 |
| **Docs** | README + quickstart | P0 |
| **Docs** | Demo GIF | P0 |

### Out of Scope (v2+)

| Feature | Version | Notes |
|---|---|---|
| `unsample proxy` (transparent proxy mode) | v2 | Higher value but complex |
| Node.js / Python SDK interceptors | v2 | Go first, others after validation |
| Request/response body capture (LogRecords) | v2 | Nice to have, not core |
| Async queue stitching (Kafka/SQS) | v2 | Complex, skip for now |
| `.http` file support | v2 | Nice to have |
| JWT/PKI token infrastructure | v2 | HMAC sufficient for v1 |
| Custom trace viewer UI | v3 | Use Jaeger/Tempo UI |
| AI trace summary | v3 | Differentiation feature |
| Unsample Cloud (hosted SaaS) | v3 | After OSS traction |
| Browser extension (Chrome) | v3 | High value, Sherlog-inspired |

---

## 12. Development Milestones

### Week 1: Core CLI + Collector Processor

| Day | Deliverable |
|---|---|
| Mon | Project scaffold, Go modules, Makefile |
| Tue | HMAC token generation + verification + tests |
| Wed | CLI `debug` command: send request, inject headers |
| Thu | Collector processor: stateless span router + rate limiter |
| Fri | Docker Compose: Collector + Tempo + Jaeger UI |

**Demo:** `unsample debug <url>` → trace appears in Jaeger

### Week 2: SDK Interceptor + Polish

| Day | Deliverable |
|---|---|
| Mon | Go HTTP middleware with HMAC verification |
| Tue | Go gRPC interceptor |
| Wed | CLI trace poller + deep link output |
| Thu | `unsample init` config generator |
| Fri | Demo multi-service app (3 services) |

**Demo:** Debug trace across 3 services with full span tree

### Week 3: Docs + Distribution + Launch Prep

| Day | Deliverable |
|---|---|
| Mon | README, quickstart guide, architecture doc |
| Tue | Demo GIF recording (30-second flow) |
| Wed | Homebrew formula, `go install` instructions |
| Thu | Troubleshooting guide (proxy stripping, etc.) |
| Fri | Draft Reddit/HN launch posts |

**Demo:** End-to-end install + debug in under 5 minutes

---

## 13. Deployment Topology

### Local Development

```
docker-compose up

┌─────────────────────────────────────────────────┐
│                Docker Compose                    │
│                                                  │
│  ┌──────────────┐  ┌───────────┐  ┌───────────┐│
│  │ OTel         │  │ Tempo     │  │ Jaeger UI ││
│  │ Collector    │  │ (debug)   │  │           ││
│  │ :4317/:4318  │  │ :3200     │  │ :16686    ││
│  │              │──│           │──│           ││
│  │ unsample     │  │ 7-day TTL │  │ Trace     ││
│  │ processor    │  │           │  │ viewer    ││
│  └──────────────┘  └───────────┘  └───────────┘│
│                                                  │
│  ┌──────────────┐                               │
│  │ Demo App     │                               │
│  │ service-a    │                               │
│  │ service-b    │                               │
│  │ service-c    │                               │
│  └──────────────┘                               │
└─────────────────────────────────────────────────┘
```

### Production

```
┌─────────────┐
│ Developer   │     $ unsample debug https://api.prod.com/checkout
│ Laptop      │
└──────┬──────┘
       │
       ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ K8s Pod:     │     │ K8s Pod:     │     │ K8s Pod:     │
│ service-a    │────▶│ service-b    │────▶│ service-c    │
│ + unsample   │     │ + unsample   │     │ + unsample   │
│   middleware │     │   middleware │     │   middleware │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────────────┼────────────────────┘
                            ▼
                   ┌──────────────────┐
                   │ OTel Collector   │
                   │ (DaemonSet)      │
                   │ + unsample proc. │
                   └────────┬─────────┘
                            │
                   ┌────────┼─────────┐
                   ▼                  ▼
          ┌──────────────┐  ┌──────────────┐
          │ Tempo-debug  │  │ Tempo-prod   │
          │ (7-day TTL)  │  │ (30-day TTL) │
          │ S3 bucket    │  │ S3 bucket    │
          └──────────────┘  └──────────────┘
```

---

## 14. Open Questions

> [!IMPORTANT]
> These need resolution before or during implementation.

| # | Question | Options | Leaning |
|---|---|---|---|
| 1 | **SDK interceptor: how to override sampler?** | a) Custom `ParentBased` sampler wrapper  b) `SpanProcessor` that re-records spans | Need to prototype both |
| 2 | **Collector: routing processor or connector?** | a) Custom processor with dual export  b) OTel Connector (newer API) | Processor (more stable API) |
| 3 | **Shared secret distribution** | a) Env var only  b) Config file  c) K8s Secret mount | All three for flexibility |
| 4 | **Which Tempo API for polling?** | a) Tempo HTTP API (`/api/traces/<id>`)  b) Jaeger gRPC query API | Tempo HTTP (simpler) |
| 5 | **Demo app: what languages?** | a) All Go  b) Go + Node + Python (realistic) | All Go for v1 simplicity |
| 6 | **OSS license** | a) MIT  b) Apache 2.0 | MIT (simpler, more permissive) |
