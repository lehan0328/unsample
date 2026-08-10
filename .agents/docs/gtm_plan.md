# Unsample — Go-to-Market Plan

> **Goal:** $2K MRR within 6 months. 500 GitHub stars within 3 months.  
> **Constraint:** Nights and weekends only. $0 marketing budget. Solo operator.

---

## The Touchstone Mistake (Don't Repeat)

| What You Did With Touchstone | What To Do Differently With Unsample |
|---|---|
| Built first, posted on Reddit after | Post on Reddit BEFORE writing code |
| Posted in niche subs (r/androiddev) | Post in massive subs (r/devops = 2.3M+) |
| Generic "I built this" post | Lead with the PAIN, not the product |
| No follow-up after launch | Sustained content plan for 12 weeks |
| No monetization path | Cloud tier built into the plan from day 1 |

---

## Phase 0: Validation (Days 1-2) — BEFORE WRITING CODE

### Goal: Confirm ≥5 people say "I'd use this today"

#### Post #1: Reddit r/devops (2.3M members)

**Title:** "When you're debugging a prod issue and the trace was sampled away, what do you do?"

**Body:**
```
We run ~40 microservices with OTel + Jaeger. Our Datadog bill was getting 
insane so we dropped to 5% sampling.

Last week I was debugging a checkout failure at 2 AM. Opened Jaeger — trace 
wasn't there. Sampled away. Spent 45 minutes grep-ing through logs to 
reconstruct what happened.

I've been thinking about building a CLI that injects a "debug" flag into the 
traceparent header so the OTel Collector always keeps that specific trace. 
Something like:

    $ unsample debug https://api.myapp.com/checkout?user=123

Would any of you actually use this? Or is there already a solution I'm missing?
```

**Why this works:**
- Leads with a STORY (2 AM debugging) not a product pitch
- Asks "would you use this?" — validates demand
- Asks "am I missing something?" — catches competitors you don't know about
- Doesn't link to a repo or website (not self-promo)

#### Post #2: Reddit r/sre (100K+ members)

Same story, slightly different angle — focus on the on-call experience:

**Title:** "Traces sampled away during incidents — how do you handle it?"

#### Post #3: CNCF Slack #opentelemetry (technical audience)

**Message:**
```
Question for the group: has anyone built an OTel Collector processor that 
bypasses tail-sampling for requests with a specific debug header? 

I'm thinking of building one — a CLI injects `x-unsample-debug: true` into 
the traceparent baggage, and a custom processor always routes those traces 
to a separate backend.

Before I build it: does this already exist? Would you use it?
```

#### Post #4: Twitter/X

Quote-tweet or reply to someone complaining about Datadog costs (there are dozens daily):

```
"One thing I've noticed: we lowered our trace sampling to save money, 
but now we can't debug anything. Building a CLI that forces full tracing 
for a single request on demand. Anyone else hit this?"
```

### Decision Gate (48 hours after posting)

| Signal | Action |
|---|---|
| ≥5 comments saying "I'd use this" or "I have this problem" | ✅ **Go.** Start Phase 1 immediately. |
| 2-4 lukewarm ("cool idea, but...") | ⚠️ Dig into the "but." Refine the pitch. Repost with different framing. |
| 0-1 responses | ❌ **Kill it.** The pain isn't acute enough. Move to next idea. |
| Someone links an existing tool that does this | ❌ **Kill it.** Competitor exists. Unless their tool is terrible/abandoned. |

---

## Phase 1: OSS Launch (Weeks 1-3)

### Build the MVP

| Component | Time |
|---|---|
| `unsample` CLI (Go, Cobra) | 5 days |
| OTel Collector processor | 4 days |
| README + install docs + demo GIF | 2 days |
| Landing page (simple, unsample.dev) | 1 day |

### Launch Sequence

#### Day of Launch: The "Show" Posts

**Reddit r/devops:**
```
Title: "I built a CLI to force full distributed tracing for any request 
(open source, works with OTel)"

Remember my post 3 weeks ago about traces getting sampled away during 
debugging? I built the thing.

`unsample debug https://api.myapp.com/checkout`

It injects a debug flag into the traceparent header. An OTel Collector 
processor detects it and bypasses your sampling rules — so that specific 
trace always gets kept.

- Works with any OTel-instrumented service
- 3 lines of Collector YAML to set up
- Routes debug traces to your existing Jaeger/Tempo/SigNoz
- Open source (MIT)

GitHub: [link]

Demo: [30-second GIF showing the full flow]

Built this because I was tired of grep-ing through logs at 2 AM when 
Datadog threw away the trace I needed.

What do you think? Any edge cases I'm missing?
```

**Why this works:**
- References the validation post ("remember my post 3 weeks ago?") — shows follow-through
- Concrete demo GIF — people can SEE it working
- Asks for feedback — invites engagement
- "Any edge cases?" — engineers love finding flaws (it drives comments)

#### Same Day: Cross-post to

| Channel | Post Style |
|---|---|
| r/sre | Same post, emphasis on on-call use case |
| r/kubernetes | If relevant (K8s-deployed OTel Collectors) |
| Hacker News ("Show HN") | Shorter, technical, link to repo |
| CNCF Slack #opentelemetry | "Built the thing I mentioned. Feedback welcome." |
| Twitter/X | Thread: "I was tired of traces getting sampled away. So I built a CLI to force full tracing on demand. Here's how it works (thread):" |
| OTel Discord | Share in #contrib or #general |

#### Week 1 Target Metrics

| Metric | Target | Why It Matters |
|---|---|---|
| GitHub stars | 50+ | Social proof for future users |
| GitHub issues filed | 5+ | People are trying it and hitting edge cases = real usage |
| README page views | 500+ | Visibility |
| Unique cloners/installers | 20+ | Actual adoption |

---

## Phase 2: Content & SEO (Weeks 3-6)

### Blog Posts (Published on Dev.to, Hashnode, personal blog)

| Week | Title | SEO Target Keyword | Purpose |
|---|---|---|---|
| 3 | "How to Debug Production Issues Without Increasing Your Datadog Bill" | "datadog cost debugging" | Captures Datadog cost complaint traffic |
| 4 | "The Problem with Trace Sampling (And How to Fix It)" | "opentelemetry trace sampling" | Educational, positions Unsample as the answer |
| 5 | "Force Full Tracing for Any Request with OpenTelemetry" | "opentelemetry force trace" | Tutorial, ranks for the exact problem Unsample solves |
| 6 | "Why I Switched from Datadog to OTel + Unsample for Production Debugging" | "datadog alternative tracing" | Migration story, captures switching intent |

### Strategic Commenting

Every week, search Reddit and HN for posts about:
- "Datadog expensive"
- "trace sampled away"
- "OTel sampling"
- "production debugging microservices"

Drop a helpful comment that naturally mentions Unsample. **Not spam — genuine advice** with a mention.

Example:
```
"We hit the same issue. Ended up building an OTel Collector processor 
that forces full tracing for specific debug requests. Open sourced it 
here: [link]. Saved us from having to bump sampling to 100% every time 
someone got paged."
```

### GitHub SEO

- Add topics: `opentelemetry`, `distributed-tracing`, `debugging`, `observability`, `developer-tools`
- Write a comprehensive README with a "Quick Start" section
- Add a `CONTRIBUTING.md` to encourage community PRs
- Create "good first issue" labels to attract contributors

---

## Phase 3: Unsample Cloud — Monetization (Weeks 6-9)

### The Conversion Funnel

```
OSS User (free)
  │
  │  "I love the CLI but I don't want to run Jaeger"
  │
  ▼
Unsample Cloud ($29/mo)
  │
  │  "My team wants shared debug sessions"
  │
  ▼  
Unsample Team ($99/mo)
  │
  │  "We want AI trace summaries + Slack integration"
  │
  ▼
Unsample Pro ($199/mo)
```

### What Triggers the Upgrade

| Pain Point (OSS) | Unsample Cloud Solves It |
|---|---|
| "I don't want to run Jaeger/Tempo just for debug traces" | Hosted trace viewer — zero infra |
| "My colleague needs to see this trace too" | Shareable trace URLs |
| "I want to know WHY it failed, not just WHERE" | AI trace summary |
| "I want to debug from Slack during an incident" | `/unsample debug <url>` Slack command |

### Launch Unsample Cloud

**Reddit post:**
```
Title: "Unsample Cloud is live — debug any production request without 
running your own trace backend ($29/mo)"

3 weeks ago I open-sourced Unsample (CLI + OTel processor for on-demand 
debug tracing). 200+ GitHub stars later, the #1 request was: "I love 
this but I don't want to run Jaeger."

So I built Unsample Cloud:
- Hosted trace viewer (no infra to manage)
- Shareable trace URLs (send to your team)
- AI trace summary ("the 500 error was caused by...")
- $29/mo for solo devs, $99/mo for teams

Free tier: 50 debug traces/month
Self-hosted: still free forever (MIT)

[link]
```

---

## Phase 4: Community Compounding (Weeks 9-12)

### Integrations That Drive Adoption

| Integration | Effort | Distribution Impact |
|---|---|---|
| **Grafana plugin** | 1 week | Appears in Grafana marketplace → discovered by Grafana users |
| **VS Code extension** | 1 week | "Debug this endpoint" button in editor → discovered by VS Code users |
| **Slack bot** | 3 days | `/unsample debug <url>` → team-wide adoption within a company |
| **GitHub Action** | 2 days | Debug failing CI requests → discovered by GitHub users |

### Conference & Meetup Talks

| Event Type | Talk Title | Timeline |
|---|---|---|
| Local meetup (OTel/K8s) | "On-Demand Debug Tracing with OpenTelemetry" | Month 2 |
| KubeCon CFP | "Beyond Sampling: Developer-Triggered Debug Tracing" | Month 4 (CFP) |
| Podcast guest (DevOps/SRE shows) | "Why Trace Sampling Breaks Production Debugging" | Month 3 |

### The Viral Loop (Aspiration)

```
Developer debugs an issue with Unsample
  │
  ▼
Shares the trace URL in Slack incident channel
  │
  ▼
3 teammates see it → "What's Unsample?"
  │
  ▼
They install the CLI → one upgrades to Cloud
  │
  ▼
Next incident → they share a trace → more teammates see it
```

---

## Conversion Funnel Math

### Assumptions (Conservative)

| Stage | Count | Conversion Rate |
|---|---|---|
| See a Reddit/HN post | 10,000 | — |
| Click through to GitHub | 1,000 | 10% CTR |
| Star the repo | 200 | 20% star rate |
| Install and try it | 50 | 5% of viewers |
| Use it regularly (weekly) | 25 | 50% retention |
| Need hosted viewer (Cloud) | 10 | 40% of regular users |
| Convert to paid ($29/mo) | 5 | 50% of Cloud trialists |

**Per major launch post: ~5 paid users × $29/mo = $145 MRR added.**

To hit $2K MRR: need ~14 successful content posts/launches over 6 months, or organic growth from SEO + word-of-mouth filling the gap.

### Revenue Ramp Projection

| Month | MRR | Cumulative Paid Users | Key Driver |
|---|---|---|---|
| 1 | $0 | 0 | OSS launch only (no cloud yet) |
| 2 | $145 | 5 | Cloud launch + first Reddit post |
| 3 | $435 | 15 | HN launch + blog SEO kicks in |
| 4 | $725 | 25 | Slack/VS Code integrations drive team adoption |
| 5 | $1,160 | 40 | Word-of-mouth + content compounding |
| 6 | $2,030 | 70 | SEO + organic inbound |

---

## Anti-Patterns to Avoid

| Don't Do This | Do This Instead |
|---|---|
| ❌ Post "I built a thing" with no context | ✅ Lead with the pain story, then reveal the tool |
| ❌ Spam the same post across 10 subs on the same day | ✅ Stagger posts across days. Customize each one. |
| ❌ Argue with critics in comments | ✅ Thank them, ask what they'd improve |
| ❌ Focus on features | ✅ Focus on the moment: "2 AM, paged, trace is gone" |
| ❌ Build Cloud before validating OSS demand | ✅ 200+ stars before writing Cloud billing code |
| ❌ Target generic "developer" audience | ✅ Target SREs/backend devs who run OTel Collectors |
| ❌ Launch once and hope for organic growth | ✅ 12-week sustained content plan |

---

## Weekly Rhythm (Ongoing)

| Day | Activity | Time |
|---|---|---|
| **Monday** | Scan Reddit/HN for relevant threads to comment on | 15 min |
| **Wednesday** | Write or publish one blog post / tutorial | 1-2 hours |
| **Friday** | Ship one small feature or fix, tweet about it | 30 min |
| **Saturday** | Build next integration (Slack bot, Grafana plugin, etc.) | 2-3 hours |

Total time commitment: **~5-6 hours/week** after initial build.

---

## Success Criteria (Kill/Continue Gates)

| Checkpoint | Timeframe | Continue If... | Kill If... |
|---|---|---|---|
| **Validation** | Day 2 | ≥5 "I'd use this" | 0-1 responses |
| **OSS Launch** | Week 3 | ≥50 stars, ≥5 issues filed | <10 stars, 0 issues |
| **Cloud Launch** | Week 8 | ≥3 paid users | 0 paid users after 2 weeks |
| **Growth** | Month 6 | ≥$1K MRR | <$500 MRR with no growth trend |
