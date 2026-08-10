# Unsample — Product Overview

> **On-demand debug tracing for OpenTelemetry.**  
> Debug any production request with full distributed tracing. No Datadog bill required.

---

## 1. The Problem

Modern companies use distributed tracing (Datadog, Honeycomb, New Relic) to understand how requests flow through microservices. But **tracing every request is expensive** — Datadog alone can cost $50K+/year. So companies sample: they only keep 1-5% of traces.

**The result:** When a developer is debugging a live production issue, the trace they need was in the 95% that got thrown away. They're flying blind.

### The Current Workarounds (All Bad)

| Workaround | Why It Fails |
|---|---|
| Bump sampling to 100% temporarily | Explodes your APM bill. $10K+ surprise invoice. |
| Add `console.log` statements, redeploy | Takes 15-30 min. Changes the system behavior. Messy. |
| Reproduce in staging | Prod-only bugs can't be reproduced. Data is different. |
| Scroll through logs and guess | Slow, error-prone, doesn't show cross-service flow. |
| Ask SRE to increase retention filters in Datadog | Requires ops team involvement. Takes hours/days. |

### Who Feels This Pain

- **Backend developers** debugging 500 errors or slow endpoints in microservice architectures
- **SREs** investigating production incidents where the trace was sampled away
- **On-call engineers** who get paged at 3 AM and need to understand what happened
- **Teams migrating from Datadog to OTel** who lose the "just look it up" convenience

---

## 2. The Solution

### What Unsample Does (Simple Version)

```bash
$ unsample debug https://api.myapp.com/checkout?user=123

🔍 Injecting debug trace header...
📡 Request sent with trace ID: abc-123-def
⏳ Waiting for spans to arrive...

✅ Full trace captured (12 spans, 847ms total)
   → View: http://localhost:16686/trace/abc-123-def

   api-gateway     → 12ms
   auth-service    → 34ms
   billing-service → 340ms ⚠️ SLOW
     └─ postgres   → 312ms ⚠️ Query: SELECT * FROM subscriptions WHERE...
   notification-svc → 8ms
```

### What Unsample Does (Technical Version)

Unsample is three things:

1. **A CLI** that injects a `x-unsample-debug: true` attribute into the W3C `traceparent` header of an HTTP request
2. **An OTel Collector processor** that inspects incoming spans — if the `x-unsample-debug` attribute is present, it bypasses all tail-sampling policies and routes the trace to a dedicated debug backend
3. **A lightweight trace viewer** (self-hosted Jaeger/Tempo, or Unsample Cloud) that stores and displays the forced debug traces

### How It Works (Architecture)

```
Developer's terminal
     │
     │  $ unsample debug https://api.myapp.com/checkout
     │
     ▼
[Unsample CLI] ──► Injects x-unsample-debug=true into traceparent header
     │
     ▼
[Your API Gateway / Service A]
     │  (OTel SDK propagates the debug flag to all downstream services)
     ├──► [Service B] ──► [Service C]
     │                        │
     ▼                        ▼
[OTel Collector]          [OTel Collector]
     │                        │
     ▼                        ▼
[Unsample Processor] ◄─── checks for x-unsample-debug attribute
     │
     ├─► If debug=true  → Route to Unsample backend (always keep, full payloads)
     └─► If debug=false → Route to normal APM (subject to sampling)
     
     ▼
[Unsample Trace Viewer] ──► Developer sees full trace in browser or terminal
```

### Prerequisites for the User

1. Services must be instrumented with OpenTelemetry (any language)
2. OTel Collector must be deployed (standard setup)
3. Add Unsample processor to Collector config (3 lines of YAML)

---

## 3. Why Now

| Trend | Impact on Unsample |
|---|---|
| **OTel won the instrumentation war** | Universal standard means one processor works everywhere. No vendor lock-in. |
| **Datadog cost backlash** | Companies actively migrating to OTel + cheaper backends. They NEED debugging tools that don't cost $50K/yr. |
| **W3C Trace Context is standard** | The protocol for forced sampling exists (`traceparent` sampled flag). Nobody wrapped it in a dev tool. |
| **LLMs can summarize traces** | Future feature: "The 500 error occurred because auth-service returned 403. The JWT expired 2 min before the request." |

---

## 4. Competitive Landscape

### The Key Insight

> **The protocol for forced tracing exists. But no developer-friendly tool wraps it.**
> Unsample is Docker-for-containers: the building blocks existed (LXC, cgroups), Docker made them usable.

### Competitive Map

```
                              Developer-Friendly
                                     ▲
                                     │
                                     │    ★ UNSAMPLE
                                     │    (CLI + processor + cheap viewer)
                                     │
                                     │
          otel-cli                   │
          (manual span creation)     │
                                     │
  Cheap / Free ──────────────────────┼──────────────────── Expensive
                                     │
          Grafana Tempo              │    Honeycomb ($500/mo+)
          Jaeger                     │    (query-first, Refinery tail-sampling)
          SigNoz ($49/mo)            │
                                     │    Datadog ($31-40/host/mo)
                                     │    (full APM, head-based sampling)
                                     │
                                     ▼
                              Infrastructure-Heavy
```

### Detailed Competitor Analysis

| Tier | Player | Funding | Overlap | Why Unsample Is Different |
|---|---|---|---|---|
| **Enterprise APM** | Datadog | Public ($2.5B ARR) | 30% | Unsample solves the gap Datadog's sampling creates. Costs $29/mo, not $50K/yr. |
| | New Relic | Public ($900M ARR) | 25% | "Infinite tracing" still requires Collector config. Not developer-triggered. |
| | Dynatrace | Public ($1.5B ARR) | 15% | AI-driven auto-sampling. Opaque. Enterprise-only. |
| **Dev-First Tracing** | Honeycomb | $97M raised | **50%** | Closest conceptually. But Honeycomb is a $500/mo APM replacement. Unsample is a $29/mo add-on. |
| | Lightrun | $110M raised | 40% | Different mechanism: injects logs/snapshots into running code. Complementary, not competitive. |
| **OSS Backends** | Jaeger, Tempo, SigNoz | N/A | Partners | Unsample routes debug traces TO these. They're backends, not competitors. |
| **OTel Ecosystem** | W3C `traceparent` flag | N/A | **70%** | The PROTOCOL exists. No dev tool wraps it. This is the gap. |
| | Envoy `x-envoy-force-trace` | N/A | 60% | Only works with Envoy. Not a CLI. |
| | `otel-cli` | OSS | 30% | Creates spans manually. Doesn't force sampling on existing services. |

### Key Risk: OTel Native Feature

The biggest threat is OpenTelemetry shipping a native `otel debug` CLI or built-in debug mode. Mitigation: **move fast, build community before OTel reacts.**

---

## 5. Market Sizing

| Level | Segment | Size |
|---|---|---|
| TAM | Global observability market | ~$10-12B (2026) |
| SAM | Companies using OTel + frustrated with sampling | ~$500M-1B |
| SOM | Teams that would adopt a $29/mo debug tool | ~$10-50M |

### Revenue Scenarios

| Scenario | Users | ARPU | ARR |
|---|---|---|---|
| **Bootstrapper Target** | 500 | $29/mo | **$174K** |
| **Growth** | 2,000 | $49/mo (avg) | **$1.2M** |
| **Scale** | 10,000 | $59/mo (avg) | **$7M** |

---

## 6. Product Roadmap

### Phase 1: Open-Source CLI + Processor (Weeks 1-3)

**Goal:** Ship the core open-source tool. Get GitHub stars. Prove the concept.

| Component | Tech | Time |
|---|---|---|
| `unsample` CLI (Go binary) | Go, Cobra CLI framework | 1 week |
| OTel Collector processor (Go) | OTel Collector SDK | 1 week |
| Documentation + install script | Markdown, shell | 2-3 days |
| README, demo GIF, landing page | Hugo or plain HTML | 2-3 days |

**Deliverable:** `unsample debug <url>` works end-to-end with a local Jaeger instance.

**Distribution:**
- Post on r/devops, r/sre, Hacker News ("Show HN")
- CNCF Slack #opentelemetry channel
- Twitter/X to OTel community

### Phase 2: Unsample Cloud (Weeks 4-6)

**Goal:** Monetize. Provide a hosted trace viewer so users don't need to run Jaeger.

| Component | Tech | Time |
|---|---|---|
| Trace ingestion API | Go + ClickHouse or SQLite | 1 week |
| Web trace viewer | React/Next.js, waterfall UI | 1.5 weeks |
| Auth + billing | Clerk + Stripe | 3-4 days |
| OTLP exporter config for Unsample Cloud | YAML template | 1 day |

**Pricing:**
- **Free:** Self-hosted (use your own Jaeger/Tempo)
- **Solo ($29/mo):** 1,000 debug traces/month, 7-day retention
- **Team ($99/mo):** 10,000 traces, 30-day retention, shared sessions, Slack integration

### Phase 3: AI Trace Analysis (Weeks 7-10)

**Goal:** Differentiate. Turn raw traces into instant root-cause answers.

| Feature | Description |
|---|---|
| **AI Summary** | "The 500 error occurred because billing-service returned a null subscription object for user 123. The subscription was deleted 2 hours ago." |
| **Diff Mode** | Compare a failing debug trace against a known-good trace. Highlight what's different. |
| **Slack Bot** | `/unsample debug https://api.myapp.com/checkout` → posts trace summary in Slack |
| **"Record" Mode** | Always-on debug tracing for a specific user/session (customer support use case) |

---

## 7. Tech Stack

| Layer | Technology | Why |
|---|---|---|
| CLI | **Go** | Single binary, cross-platform, matches OTel Collector ecosystem |
| OTel Processor | **Go** (OTel Collector SDK) | Must be Go to integrate with the Collector |
| Trace Storage | **ClickHouse** or **SQLite** (for cloud) | ClickHouse is fast for trace queries. SQLite for simplicity at small scale. |
| Trace Viewer | **React** + waterfall visualization | Fork or adapt Jaeger UI |
| API | **Go** (net/http or Fiber) | Keep the stack consistent |
| Auth | **Clerk** | Fast integration, handles OAuth/magic links |
| Billing | **Stripe** | Industry standard |
| Hosting | **Fly.io** or **Railway** | Cheap, fast deployment |

---

## 8. Go-to-Market Strategy

### Distribution Channels (Ranked by Fit)

| Channel | Fit | Why |
|---|---|---|
| **Reddit (r/devops, r/sre, r/kubernetes)** | ✅ Best | These communities actively discuss Datadog costs and OTel migration |
| **Hacker News (Show HN)** | ✅ Best | "Debug any production request for $0" is HN catnip |
| **CNCF Slack / OTel Discord** | ✅ Strong | The people who already run OTel Collectors |
| **Twitter/X (DevTools community)** | ✅ Strong | OTel maintainers, SREs, DevRel people |
| **Dev.to / Hashnode blog posts** | 🟡 Medium | SEO long-tail |
| **Product Hunt** | 🟡 Medium | Dev tools do OK, not primary audience |

### Launch Sequence

1. **Week 0 (pre-build):** Validation post on r/devops — "When your trace gets sampled away during debugging, what do you do?"
2. **Week 3:** Ship OSS CLI + processor. Post on GitHub, r/devops, CNCF Slack.
3. **Week 4:** Show HN post. Target 100+ GitHub stars.
4. **Week 6:** Launch Unsample Cloud. Convert OSS users to $29/mo.
5. **Week 8:** Blog post: "How we debug production issues without Datadog" — SEO play.

### Positioning Statement

> **For backend developers** who debug production issues in microservice architectures,  
> **Unsample** is an open-source CLI that forces full distributed tracing for any request on demand.  
> **Unlike** Datadog or Honeycomb, which require $500+/mo and still sample away the traces you need,  
> **Unsample** works with your existing OTel setup and costs $0 self-hosted or $29/mo for cloud.

---

## 9. Risk Matrix

| Risk | Severity | Probability | Mitigation |
|---|---|---|---|
| OTel ships native debug CLI | 🔴 High | 🟡 Medium | Move fast. Ship in 3 weeks. Build community first. |
| "Vitamin" not painkiller | 🟡 Medium | 🟡 Medium | Validate with r/devops post BEFORE building. |
| Datadog adds "debug button" | 🟡 Medium | 🟢 Low | Datadog is incentivized to sell full ingestion, not cheap debug mode. |
| Honeycomb Refinery becomes easier | 🟡 Medium | 🟡 Medium | Unsample is $29/mo, not $500/mo. Different buyer. |
| Too small for SaaS revenue | 🟡 Medium | 🟡 Medium | $29/mo × 500 users = $174K ARR. Viable for solo bootstrapper. |
| Requires OTel adoption | 🟢 Low | 🟢 Low | OTel IS the standard in 2026. Adoption is accelerating. |

---

## 10. Pre-Build Validation Plan (48 Hours)

Before writing a single line of code:

- [ ] **Post #1 (r/devops):** "When you're debugging a production issue and the trace was sampled away by your APM, what do you do? We're building a CLI that forces 100% tracing for a single request. Would you use it?"
- [ ] **Post #2 (r/sre):** Same question, different community
- [ ] **Post #3 (CNCF Slack #opentelemetry):** More technical framing — "We're building an OTel Collector processor that bypasses tail-sampling for debug-flagged requests. Interested?"
- [ ] **Search GitHub issues** on `open-telemetry/opentelemetry-collector-contrib` for "debug" or "force sample" feature requests
- [ ] **DM 5 OTel users** on Twitter who have tweeted about Datadog costs

### Decision Gate

| Responses | Action |
|---|---|
| ≥ 5 "I'd use this" | ✅ Build it. Start Phase 1 immediately. |
| 2-4 lukewarm | ⚠️ Refine the pitch. Try a different framing. |
| 0-1 | ❌ Kill the idea. Move on. |

---

## 11. Key Metrics to Track

| Stage | Metric | Target |
|---|---|---|
| Validation | Reddit/HN comment sentiment | ≥ 5 "I'd use this" |
| OSS Launch | GitHub stars (week 1) | ≥ 50 |
| OSS Growth | GitHub stars (month 1) | ≥ 200 |
| Cloud Launch | Free → paid conversion | ≥ 5% |
| Revenue | MRR (month 3) | ≥ $500 |
| Revenue | MRR (month 6) | ≥ $2,000 |

---

## 12. Reference Documents

| Document | Location |
|---|---|
| Competitive Analysis (full) | [unsample_competitive_analysis.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/unsample_competitive_analysis.md) |
| Idea Pressure Test (all 5 ideas) | [idea_pressure_test.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/idea_pressure_test.md) |
| Market Validation Evidence | [market_validation_evidence.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/market_validation_evidence.md) |
| Google Internal Prompt v2 | [google_internal_prompt_v2.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/google_internal_prompt_v2.md) |
| Arbitrage Analysis | [arbitrage_analysis.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/arbitrage_analysis.md) |
| BuildSignal Analysis | [buildsignal_competitive_analysis.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/buildsignal_competitive_analysis.md) |
| Coroner Analysis | [coroner_competitive_analysis.md](file:///Users/lehanouyang/.gemini/antigravity-ide/brain/501d4572-f117-4bd9-93c1-11f2e847d48f/coroner_competitive_analysis.md) |

---

## Summary

**Unsample fills a specific, underserved gap:** the moment between "I need to debug this production request" and "the trace was sampled away." It wraps existing OpenTelemetry protocol capabilities (W3C trace context, tail-sampling policies) into a developer-friendly CLI that anyone can use in 10 seconds.

**What makes it viable for a solo bootstrapper:**
- ✅ 2-3 week MVP (Go CLI + OTel processor)
- ✅ Self-serve GTM (Reddit, HN, CNCF Slack — you ARE the target user)
- ✅ Open-source core → $29/mo cloud upsell
- ✅ Rides the anti-Datadog + pro-OTel wave
- ⚠️ Thin moat (OTel community could build this)
- ⚠️ Must validate demand BEFORE building (Touchstone lesson)
