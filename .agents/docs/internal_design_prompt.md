# Prompt for Internal Model — Unsample Design Review

> **Context for you (don't paste this part):**
> You're building an open-source CLI tool called "Unsample" that forces full distributed trace sampling for a specific HTTP request on demand. It works with OpenTelemetry. You want to learn from Google's internal Sherlog design to avoid mistakes and follow proven patterns.
>
> **What to paste into the internal model:**

---

## The Prompt

```
I'm building an open-source tool for on-demand debug tracing in OpenTelemetry 
environments. The concept is similar to what Sherlog does internally — a 
developer triggers a debug flag on a specific request, and the tracing 
infrastructure ensures that request's trace is fully captured regardless of 
sampling configuration.

My external implementation plan:
- A CLI that sends an HTTP request with a debug attribute injected into 
  the W3C traceparent/baggage headers
- An OTel Collector processor that detects the debug attribute and bypasses 
  tail-sampling policies for that trace
- Debug traces routed to a separate lightweight backend (Jaeger/Tempo)

I'd like your help validating this design by answering the following questions. 
Please focus on general architectural patterns and lessons learned — I'm NOT 
asking for internal code or implementation details, just design guidance 
based on what works (and what doesn't) at scale.

---

### 1. Architecture Validation

a) In Sherlog's design, how does the debug signal propagate across service 
   boundaries? Specifically:
   - Is the debug flag carried in W3C baggage, tracestate, a custom header, 
     or span attributes?
   - What are the trade-offs of each propagation mechanism?
   - Are there edge cases where the debug flag gets dropped (e.g., async 
     message queues, gRPC streams, batch jobs)?

b) At what layer is the "keep this trace" decision made?
   - At the SDK/instrumentation level (each service decides)?
   - At the Collector/pipeline level (centralized decision)?
   - Or both (defense in depth)?
   
   What are the failure modes of each approach?

c) How does Sherlog handle the case where a request has ALREADY been 
   partially sampled out before the debug flag reaches downstream services?
   (i.e., Service A sampled=false, but Service B receives the debug flag — 
   do you get a partial trace?)

---

### 2. Edge Cases & Failure Modes

Based on Sherlog's production experience, what are the most common failure 
modes or edge cases that an external implementation should anticipate?

Specifically:
a) What happens when the debug-flagged request fans out to 50+ downstream 
   services? Is there a risk of trace explosion?

b) How do you prevent abuse? (e.g., a script that debug-flags every request, 
   effectively bypassing all sampling and flooding the trace backend)

c) What happens with async workflows? If the debug-flagged request publishes 
   to a Kafka topic and a consumer processes it 30 seconds later, does the 
   debug flag propagate through the message queue?

d) What about long-running requests (WebSockets, streaming RPCs)? Does the 
   debug flag apply to the entire connection or just one message?

e) How are retries handled? If a debug-flagged request fails and is retried 
   by a client, does the retry automatically get the debug flag too?

---

### 3. Performance & Safety

a) What is the performance overhead of the debug flag inspection? 
   (i.e., does checking every span for a debug attribute add measurable 
   latency to the hot path?)

b) Are there rate limits or circuit breakers on debug tracing? 
   (e.g., max N debug traces per minute per user)

c) How does Sherlog avoid becoming a DoS vector? If debug traces bypass 
   sampling, a malicious actor could flood the trace backend.

d) What's the recommended storage strategy for debug traces? 
   - Same backend as production traces but different retention?
   - Completely separate backend?
   - In-memory with TTL?

---

### 4. Developer Experience

a) What does the developer workflow actually look like end-to-end? 
   Walk me through: developer has a bug report → they want to debug it → 
   they trigger Sherlog → they see the trace. What are the steps?

b) What's the latency from "trigger debug" to "see the full trace"? 
   Seconds? Minutes?

c) Is there a way to debug a request that has ALREADY happened 
   (retroactive debugging), or only future requests?

d) What's the most common misunderstanding developers have about Sherlog 
   when they first use it?

---

### 5. What NOT to Build (Anti-Patterns)

Based on Sherlog's evolution, what features or design decisions turned out 
to be mistakes or unnecessary complexity? Specifically:

a) Were there features that seemed important but nobody used?

b) Were there architectural decisions that had to be reversed later?

c) If you were designing Sherlog from scratch for an open-source, 
   external OTel environment (no Google-internal infrastructure), 
   what would you do differently?

d) What's the minimum viable implementation that delivers 80% of the 
   value? (i.e., what can I skip in v1?)
```

---

## Follow-Up Prompts (Use After Getting Initial Answers)

### Follow-up 1: Collector Processor Design
```
Based on your answer about the architecture, I'm planning to implement the 
debug detection as an OTel Collector processor (Go). 

Given Sherlog's experience:
- Should the processor inspect span attributes, resource attributes, 
  or the traceparent header directly?
- Should it modify the sampling decision in-place, or route to a 
  separate pipeline?
- Are there concurrency or memory concerns I should watch for?
```

### Follow-up 2: CLI Design
```
For the CLI that triggers debug tracing:

In Sherlog's experience, what information does the developer need to provide?
- Just a URL?
- URL + headers + body?
- A specific user ID or session ID?
- A trace ID from a previous (sampled-out) request?

And what should the CLI output?
- Just a link to the trace viewer?
- An inline summary of the trace (waterfall in terminal)?
- A "waiting for trace..." spinner with real-time span arrival?
```

### Follow-up 3: What Broke
```
In Sherlog's production history, what were the top 3 incidents or 
near-misses caused by the debug tracing system itself? 
(e.g., debug traces flooding storage, debug flag leaking to 
non-debug traffic, performance regression from flag inspection)

What safeguards were added after each incident?
```
