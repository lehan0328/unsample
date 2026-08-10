# Pressure Test: 5 New Ideas from Google Internal Model

Evidence gathered 2026-08-08. Applying the Touchstone lesson: **analysis ≠ reality.**

---

## Idea #1: Sherlog for OTel (On-Demand Debug Tracing) — Score: 92/100

### ✅ Verified Claims

| Claim | Evidence | Verdict |
|---|---|---|
| Companies ripping out Datadog due to cost | ✅ Multiple sources confirm. Datadog's multi-dimensional pricing (hosts + spans + metrics + logs) causes bills to "explode unexpectedly." Companies migrating to OTel + SigNoz/Grafana. | **Strong** |
| OTel has won the instrumentation war | ✅ OTel Collector is the standard. Every major vendor supports it. | **Strong** |
| Sampling causes missing traces during debugging | ✅ Documented pain. Tail-sampling + head-sampling gap is real. | **Strong** |

### ⚠️ What the Model Missed

| Issue | Reality |
|---|---|
| "No solo-friendly tool dominates this niche" | **Partially wrong.** W3C Trace Context already has a `sampled` flag in the `traceparent` header. Setting `01` forces downstream sampling. The PROTOCOL exists — what's missing is the **developer experience wrapper** (CLI tool to inject the header + a cheap viewer for the forced traces). |
| "Write an OTel Collector processor" | **Feasible but nuanced.** The tail-sampling processor already supports attribute-based policies (e.g., `debug=true`). You could configure it WITHOUT writing custom code. The value-add is the DX layer: "type one command, see the full trace" — not the sampling logic itself. |
| OTel might ship this natively | **Real risk.** The building blocks exist in the spec. A "debug mode" SDK feature could wipe this out. |

### Honest Assessment

**The pain is real and the money is real** (Datadog bills are the #1 complaint in observability). But the actual technical gap is narrower than described — it's a DX/workflow gap, not an infrastructure gap. The product is less "OTel processor" and more "CLI tool + cheap trace viewer that wraps existing OTel capabilities."

**Painkiller test:** Would a developer interrupt their sprint? **YES** — when they're actively debugging a production issue and the trace was sampled away, they would absolutely install a CLI tool that forces full tracing for their next request.

**Risk:** Medium. OTel community could ship a native `otel debug` CLI.

---

## Idea #2: Sponge for CI (AI CI Log Comments) — Score: 84/100

### 🔴 Critical Finding: Competition Already Exists

The model said "there is a gap for a dumb-simple, 1-click GitHub App." **This is wrong.** The space is already active:

| Tool | What It Does | Status |
|---|---|---|
| **FailBrief** | GitHub App that categorizes CI failures + AI-generated summaries + fix recommendations | Already shipping |
| **CTRF (github-test-reporter)** | Framework-agnostic test reporter + AI analysis via `npx` | Active, open-source |
| **dorny/test-reporter** | Parses JUnit/Jest results → PR annotations | Very popular, open-source |
| **ExplainThisError** | API/GitHub Action that parses logs → root causes + fix commands | Already shipping |
| **Optibot** | AI agent that reads logs, applies patches, re-triggers CI | Emerging |
| **Trunk.io** | $31M raised. Flaky test quarantine + AI failure summaries | Well-funded |

### Honest Assessment

**The pain is absolutely real** (developers spending 15-25% of time triaging CI logs). But this is NOT a blue ocean — it's an active, competitive space with both funded startups and free open-source tools. The Google model gave this 84/100 based on outdated competitive data.

**Painkiller test:** Yes, but developers already have free options (dorny/test-reporter, CTRF). Competing with free + funded is the Touchstone trap all over again.

**Verdict: DOWNGRADED to 65/100.** Skip this one.

---

## Idea #3: Coroner for K8s (JIT OOM Snapshotting) — Score: 83/100

### ✅ Verified Claims

| Claim | Evidence | Verdict |
|---|---|---|
| OOMKilled gives zero context | ✅ Confirmed. SIGKILL is instant — no time for heap dump. Documented SRE pain. | **Strong** |
| Current workaround is fragile | ✅ The "lower internal limit to trigger managed OOM" trick is well-known but error-prone and runtime-specific (only works for JVM with -Xmx). | **Strong** |
| SREs spend weekends catching OOMs | ✅ Plausible based on the volume of SO/Reddit questions. | **Moderate** |

### ⚠️ What the Model Missed

| Issue | Reality |
|---|---|
| eBPF makes this possible | **True but complex.** eBPF hooks into kernel cgroup events, but writing a reliable eBPF program that triggers heap dumps without itself consuming memory is hard. This is NOT a 2-4 week MVP for a solo dev. |
| "Focus purely on Node.js first" | **Smart scoping.** Node's inspector protocol (`v8.getHeapSnapshot`) can be triggered remotely. This is the most feasible runtime. |
| Robusta.dev doesn't do preemptive OOM snapshotting | **Need to verify.** Robusta does K8s event-driven automation. They could add this feature. |

### Honest Assessment

**The pain is severe** (3 AM pages with zero debugging context). But the engineering is hard — harder than the model implies. A reliable "dump at 95% memory" agent that works across Node/Python/Java without itself causing the OOM is a tricky systems problem.

**Painkiller test:** **YES, absolutely.** The SRE who was paged at 3 AM last night would install this TODAY.

**Risk:** Engineering complexity may push MVP beyond 4 weeks. Also, you need to run this in production K8s — trust barrier is high for a solo-dev product.

---

## Idea #4: Placer / API Hub — Score: 76/100

### 🔴 Competition: Backstage Exists

Backstage (by Spotify) is open-source, widely adopted, and backed by the CNCF. The "fully managed Backstage" niche already has:
- **Roadie.io** — managed Backstage ($50K+/year enterprise)
- **Cortex.io** — service catalog ($100K+/year)
- **Port (getport.io)** — developer portal platform

**Verdict:** Enterprise-sales territory. Not a bootstrapper play. Skip.

---

## Idea #5: Rapid Auto-Rollback — Score: 73/100

The model correctly identified the trust barrier as disqualifying. Agreed. Skip.

---

## Revised Ranking After Evidence Check

| Rank | Idea | Original Score | **Revised Score** | Key Change |
|---|---|---|---|---|
| **1** | **Sherlog for OTel** | 92 | **85** | Pain is real, money is real, but the gap is DX not infra. Risk of OTel native feature. |
| **2** | **Coroner for K8s** | 83 | **75** | Pain is severe, but engineering is harder than claimed. Trust barrier for prod K8s. |
| **3** | Sponge CI Comments | 84 | **65** | FailBrief, CTRF, dorny/test-reporter already exist. Space is crowded. |
| **4** | Placer API Hub | 76 | **50** | Backstage + Roadie + Cortex + Port. Enterprise sales required. |
| **5** | Rapid Auto-Rollback | 73 | **45** | Trust barrier correctly identified. |

---

## Recommendation

**Sherlog is the strongest candidate**, but with a refined product vision:

**What to build is NOT an OTel Collector processor** (that's infrastructure). What to build is:

```
sherlog debug <curl command or URL>
```

A CLI that:
1. Injects `x-debug-trace: true` into the request header
2. Tells your OTel Collector (via a simple config addition) to always sample traces with that attribute
3. Routes those traces to a lightweight self-hosted viewer (or your SaaS)
4. Shows you the full trace with payloads in your terminal or a browser tab

**The pitch:** "Debug any production request with full distributed tracing. No Datadog required. $0 for self-hosted, $29/mo for cloud."

## 48-Hour Validation Plan (Before Writing Code)

- [ ] Post to r/devops and r/sre: "When you're debugging a production issue and the trace was sampled away by your APM, what do you do? We're building a CLI that forces 100% tracing for a single request. Would you use it?"
- [ ] Post to CNCF Slack #opentelemetry: same question
- [ ] Search GitHub issues on `open-telemetry/opentelemetry-collector-contrib` for "debug" or "force sample" requests
- [ ] **Gate:** ≥5 "I'd use this" responses → build MVP. 0-2 → reconsider.
