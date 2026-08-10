# Unsample — Implementation Plan

> **Constraint:** Nights and weekends only (~3-4 hours/weeknight, ~6-8 hours/weekend day)
> **Total estimated effort:** ~60 hours over 3 weeks
> **Goal:** Ship OSS v1, post on Reddit/HN, get 50+ GitHub stars in week 1

---

## Phase Overview

```
Phase 0: Validation          ██████████░░░░░░░░░░ IN PROGRESS
Phase 1: Core Engine         ░░░░░░░░░░░░░░░░░░░░ ~20 hours (5 days)
Phase 2: SDK + Integration   ░░░░░░░░░░░░░░░░░░░░ ~18 hours (5 days)
Phase 3: Demo + Docs + Dist  ░░░░░░░░░░░░░░░░░░░░ ~14 hours (4 days)
Phase 4: Launch              ░░░░░░░░░░░░░░░░░░░░ ~6 hours  (2 days)
```

| Phase | What | Gate to Next Phase |
|---|---|---|
| **0** | Reddit validation posts | ≥5 "I have this problem" signals |
| **1** | CLI + Collector processor + local infra | `unsample debug <url>` → trace in Jaeger |
| **2** | SDK interceptor + multi-service integration | Debug trace spans 3 services with sampler override |
| **3** | Demo app + README + distribution | End-to-end install → debug in 5 minutes |
| **4** | Launch posts + community seeding | 50+ GitHub stars in week 1 |

---

## Phase 0: Validation (IN PROGRESS)

**Status:** 4/5 validation signals collected

| Task | Status | Signal |
|---|---|---|
| Post in r/opentelemetry | ✅ Done | 3 responses — one person built it custom, one asked for a tool |
| Post in r/sre | ⬜ Pending | Needs karma or lower-barrier sub |
| Post in r/devops | ⬜ Blocked | Need 10 karma — build this weekend |
| Build r/devops karma | ⬜ Pending | Comment on 5-10 posts tonight/tomorrow |
| Post in r/devops (Tue/Wed) | ⬜ Pending | Tuesday 9-10 AM ET for max engagement |

### Gate: Phase 0 → Phase 1

| Signal | Threshold | Current |
|---|---|---|
| People confirming the pain | ≥5 | **4** (close) |
| Someone linking an existing product | 0 (must stay zero) | ✅ 0 |
| Someone who built it custom | ≥1 | ✅ 1 |
| Someone asking for a tool | ≥1 | ✅ 1 |

> [!IMPORTANT]
> **Decision:** Given the strong signal quality (not just quantity), you can start Phase 1 in parallel with the remaining r/devops validation. If the r/devops post gets zero engagement, reassess.

---

## Phase 1: Core Engine (~20 hours)

**Goal:** `unsample debug <url>` sends a request and the trace appears in Jaeger.

No SDK interceptor yet — this phase proves the CLI → Collector → Backend pipeline works.

---

### Day 1: Project Scaffold + Token System (~4 hours)

**Tasks:**

- [ ] **Create GitHub repo** (`unsample/unsample`)
  - MIT license
  - Go module: `github.com/unsample/unsample`
  - `.gitignore` for Go
  - Makefile with `build`, `test`, `lint` targets

- [ ] **Implement HMAC token system** (`internal/token/`)
  - `hmac.go`: `GenerateToken(secret, traceID) → tokenString`
  - `verify.go`: `VerifyToken(tokenString, secret, maxAge) → bool`
  - Token format: `<trace_id>:<unix_timestamp>:<hmac_base64>`
  - `hmac_test.go`: Test generation, verification, expiry, invalid tokens

**Deliverable:** `go test ./internal/token/...` passes

**Files created:**
```
cmd/unsample/main.go
internal/token/hmac.go
internal/token/verify.go
internal/token/hmac_test.go
go.mod
go.sum
Makefile
LICENSE
.gitignore
```

---

### Day 2: CLI `debug` Command (~4 hours)

**Tasks:**

- [ ] **Set up Cobra CLI** (`cmd/unsample/main.go`)
  - Root command with version flag
  - `debug` subcommand

- [ ] **Implement `debug` command** (`internal/cli/debug.go`)
  - Parse URL argument
  - Generate W3C `trace_id` and `span_id`
  - Generate HMAC token using shared secret
  - Build HTTP request with injected headers:
    - `traceparent: 00-<trace_id>-<span_id>-01`
    - `baggage: unsample-debug=<token>`
  - Send request
  - Display HTTP response (status, headers, body)

- [ ] **Implement `--curl` flag** (`internal/cli/curl_parser.go`)
  - Parse curl command string → HTTP request
  - Support: method, URL, headers, body

- [ ] **Config loading** (`internal/cli/config.go`)
  - Read `~/.unsample/config.yaml`
  - Fallback to `UNSAMPLE_SECRET` env var

**Deliverable:** `unsample debug https://httpbin.org/get` sends request with debug headers and displays response

**Files created:**
```
internal/cli/debug.go
internal/cli/curl_parser.go
internal/cli/config.go
internal/cli/output.go
```

---

### Day 3: Collector Processor (~4 hours)

**Tasks:**

- [ ] **Implement stateless span router** (`internal/collector/processor.go`)
  - `ConsumeTraces()`: iterate spans, check `debug.trace` attribute
  - Route debug spans to debug exporter
  - Route production spans to production exporter
  - **NO `groupbytrace`** — stateless per-span, O(1) memory

- [ ] **Rate limiter** (`internal/collector/ratelimit.go`)
  - Token bucket: max N debug traces per minute
  - If rate exceeded: silently drop (return nil, don't retry)

- [ ] **Processor factory** (`internal/collector/factory.go`)
  - Register processor type: `unsample`
  - Default config: `debug_attribute=debug.trace`, `max_per_minute=10`

- [ ] **Processor tests** (`internal/collector/processor_test.go`)
  - Test: debug span routed to debug exporter
  - Test: normal span routed to production exporter
  - Test: rate limiting drops excess debug spans
  - Test: no `debug.trace` attribute → production pipeline

**Deliverable:** Processor unit tests pass

**Files created:**
```
internal/collector/processor.go
internal/collector/factory.go
internal/collector/config.go
internal/collector/ratelimit.go
internal/collector/processor_test.go
```

---

### Day 4: Local Infrastructure (~4 hours)

**Tasks:**

- [ ] **Docker Compose stack** (`docker/docker-compose.yaml`)
  - OTel Collector (custom build with unsample processor)
  - Grafana Tempo (debug backend, 7-day TTL)
  - Jaeger UI (trace viewer, connected to Tempo)

- [ ] **Collector config** (`docker/otel-collector-config.yaml`)
  - OTLP receiver (gRPC :4317, HTTP :4318)
  - Unsample processor → debug pipeline → Tempo
  - Probabilistic sampler → production pipeline → Tempo

- [ ] **OCB manifest** (`builder-config.yaml`)
  - Custom Collector binary including `unsampleprocessor`
  - Standard receivers, exporters, processors

- [ ] **Build script** (`scripts/build-collector.sh`)
  - Run OCB to build custom Collector
  - Package as Docker image

**Deliverable:** `docker-compose up` → Collector + Tempo + Jaeger running

**Files created:**
```
docker/docker-compose.yaml
docker/otel-collector-config.yaml
builder-config.yaml
scripts/build-collector.sh
```

---

### Day 5: End-to-End Integration + Trace Poller (~4 hours)

**Tasks:**

- [ ] **Trace poller** (`internal/trace/poller.go`)
  - Poll Tempo HTTP API: `GET /api/traces/<trace_id>`
  - Retry with backoff (max 30 seconds)
  - Return span count when trace is found

- [ ] **Trace summary** (`internal/trace/summary.go`)
  - Parse Tempo trace response
  - Extract: service name, duration, status code, error message
  - Format as lightweight tree (NOT a waterfall)

- [ ] **Wire everything together**
  - CLI sends request → Collector receives spans → routes to Tempo
  - CLI polls Tempo → displays deep link + summary

- [ ] **Integration test**
  - Start Docker Compose
  - Run `unsample debug https://httpbin.org/get`
  - Verify trace appears in Jaeger

**Deliverable:** Full end-to-end flow working locally

### ✅ Phase 1 Gate

```bash
$ unsample debug https://httpbin.org/get

─── HTTP Response ───────────────────────────────
HTTP/1.1 200 OK
Content-Type: application/json
{ "headers": { "Baggage": "unsample-debug=..." } }

─── Debug Trace ─────────────────────────────────
⏳ Waiting for trace...
✅ Trace captured (1 span, 120ms)
   → http://localhost:16686/trace/4bf92f3577b34da6
─────────────────────────────────────────────────
```

---

## Phase 2: SDK Interceptor + Multi-Service (~18 hours)

**Goal:** Debug trace propagates across 3 microservices with sampler override at each boundary.

---

### Day 6: Go HTTP Middleware (~4 hours)

**Tasks:**

- [ ] **HTTP middleware** (`sdk/go/middleware.go`)
  - Read `unsample-debug` from baggage
  - O(1) fast path: return immediately if no debug flag (zero-alloc)
  - Verify HMAC token
  - Set `debug.trace=true` span attribute
  - Baggage auto-propagates via OTel propagators

- [ ] **Sampler override** (`sdk/go/sampler.go`)
  - When debug flag detected: override trace sampler to `AlwaysOn`
  - Research best approach: custom `ParentBased` wrapper vs `SpanProcessor`

- [ ] **Middleware tests** (`sdk/go/middleware_test.go`)
  - Test: no debug flag → zero overhead, no attributes set
  - Test: valid token → `debug.trace=true` attribute set
  - Test: invalid token → ignored
  - Test: expired token → ignored
  - Benchmark: hot-path latency < 1 microsecond

**Deliverable:** Middleware passes tests + benchmark

**Files created:**
```
sdk/go/middleware.go
sdk/go/sampler.go
sdk/go/middleware_test.go
sdk/go/go.mod
```

---

### Day 7: gRPC Interceptor + Token Sharing (~3 hours)

**Tasks:**

- [ ] **gRPC unary interceptor** (`sdk/go/grpc.go`)
  - Same logic as HTTP middleware
  - Read baggage from gRPC metadata context

- [ ] **Shared token verification** (`sdk/go/verify.go`)
  - Extract shared verification logic
  - Make it importable by SDK users

- [ ] **gRPC tests** (`sdk/go/grpc_test.go`)

**Deliverable:** gRPC interceptor passes tests

---

### Day 8: Demo Multi-Service App (~4 hours)

**Tasks:**

- [ ] **Service A: API Gateway** (`examples/demo-app/gateway/`)
  - Go HTTP server, receives external requests
  - Calls Service B via HTTP
  - OTel instrumented + Unsample middleware

- [ ] **Service B: Billing** (`examples/demo-app/billing/`)
  - Go HTTP server, calls Service C
  - Simulates slow DB query (sleep 300ms)
  - Returns 500 for specific user IDs (simulated bug)

- [ ] **Service C: Notification** (`examples/demo-app/notification/`)
  - Go HTTP server, sends fake notification, returns 200

- [ ] **Demo Docker Compose** (`examples/demo-app/docker-compose.yaml`)
  - All 3 services + OTel Collector + Tempo + Jaeger

**Deliverable:** `docker-compose up` → 3 services running with OTel + Unsample

---

### Day 9: Multi-Service Integration Test (~3 hours)

**Tasks:**

- [ ] **End-to-end test: happy path**
  - `unsample debug http://localhost:8080/checkout?user=123`
  - Verify: trace has spans from all 3 services

- [ ] **End-to-end test: sampler override**
  - Set production sampling to 0%
  - Send debug request → verify debug trace still captured

- [ ] **End-to-end test: rate limiting**
  - Send 20 rapid debug requests → only 10/min captured
  - Verify: Collector doesn't crash or OOM

**Deliverable:** Debug trace spans 3 services with full span tree

---

### Day 10: Edge Cases + Safety (~4 hours)

**Tasks:**

- [ ] **Invalid token handling** (malformed, expired, tampered → all silently ignored)
- [ ] **Concurrent debug sessions** (two developers simultaneously → traces don't interfere)
- [ ] **CLI error handling** (timeouts, invalid URLs, trace polling failures)
- [ ] **Payload size limits** (enforce truncation constants)

**Deliverable:** All edge cases handled gracefully

### ✅ Phase 2 Gate

```bash
$ unsample debug http://localhost:8080/checkout?user=500

─── HTTP Response ───────────────────────────────
HTTP/1.1 500 Internal Server Error
{"error": "subscription_not_found"}

─── Debug Trace ─────────────────────────────────
✅ Trace captured (5 spans, 847ms)
   → http://localhost:16686/trace/abc123

   gateway          12ms  ✅ 200
   billing-service  340ms ❌ 500  subscription_not_found
     └─ postgres    312ms       SELECT * FROM subscriptions...
   notification      8ms  ⏭ skipped
─────────────────────────────────────────────────
```

---

## Phase 3: Demo + Docs + Distribution (~14 hours)

**Goal:** Anyone can install Unsample and debug their first trace in under 5 minutes.

---

### Day 11: README + Quickstart (~4 hours)

- [ ] **README.md** — hero + demo GIF + quickstart + CLI reference + FAQ
- [ ] **docs/quickstart.md** — detailed 4-step setup guide
- [ ] **docs/troubleshooting.md** — proxy stripping, partial traces, Collector config

---

### Day 12: Demo GIF + Architecture Doc (~3 hours)

- [ ] **Record demo GIF** (30 seconds, `asciinema` + screen record)
- [ ] **docs/architecture.md** — public-facing version of design doc
- [ ] **GitHub repo polish** — topics, description, social preview image

---

### Day 13: `unsample init` + Distribution (~4 hours)

- [ ] **`unsample init`** — generate config, Collector YAML, Docker Compose, print next steps
- [ ] **Distribution** — `go install`, Homebrew formula, GoReleaser, GitHub Releases
- [ ] **`.goreleaser.yaml`** — cross-compile linux/darwin amd64/arm64

---

### Day 14: CI/CD + Final Polish (~3 hours)

- [ ] **GitHub Actions** — test, lint, release workflows
- [ ] **CONTRIBUTING.md** — build locally, run tests, good first issues
- [ ] **Final checklist** — fresh clone → install → init → debug → trace in Jaeger (< 5 min)

### ✅ Phase 3 Gate

Fresh clone → `go install` → `unsample init` → `docker-compose up` → `unsample debug` → trace in Jaeger (**under 5 minutes**)

---

## Phase 4: Launch (~6 hours)

### Day 15: Launch Day (Target: Tuesday 9-10 AM ET)

- [ ] **GitHub Release** — tag v0.1.0
- [ ] **Reddit r/devops** (flair: Observability) — reference validation post from 3 weeks ago
- [ ] **Reddit r/sre** — on-call angle
- [ ] **Reddit r/opentelemetry** — technical angle
- [ ] **Hacker News "Show HN"**

### Day 16: Community Seeding

- [ ] **CNCF Slack #opentelemetry** + **OTel Discord #contrib**
- [ ] **Twitter/X thread** — walkthrough
- [ ] **Reply to original validation thread** with update
- [ ] **Monitor + engage** — reply to every comment within 2 hours

### ✅ Phase 4 Gate (72 hours post-launch)

| Metric | Target | If Missed |
|---|---|---|
| GitHub stars | ≥50 | Repost with different angle |
| GitHub issues | ≥5 | People are trying it |
| Unique cloners | ≥20 | Real adoption |

---

## Post-Launch: Weekly Rhythm

| Day | Activity | Time |
|---|---|---|
| Monday | Scan Reddit/HN for relevant threads | 15 min |
| Wednesday | Write one blog post (SEO) | 1-2 hours |
| Friday | Ship one improvement, tweet about it | 30 min |
| Saturday | Build next feature (proxy mode, Node.js SDK) | 2-3 hours |

---

## Total Effort Summary

| Phase | Days | Hours | Deliverable |
|---|---|---|---|
| **0: Validation** | Ongoing | ~2 | Reddit signals ✅ |
| **1: Core Engine** | 5 | ~20 | CLI + Collector + local infra |
| **2: SDK + Integration** | 5 | ~18 | Middleware + demo app + edge cases |
| **3: Docs + Distribution** | 4 | ~14 | README + GIF + Homebrew + CI |
| **4: Launch** | 2 | ~6 | Reddit + HN + CNCF + Twitter |
| **Total** | **16 days** | **~60 hours** | **Production-ready OSS v1** |

At ~3-4 hours/weeknight + ~6-8 hours/weekend day = **~3 calendar weeks**
