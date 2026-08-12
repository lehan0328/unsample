# Troubleshooting

Common issues and solutions when using Unsample.

## "No secret configured"

```
Error: no secret configured
Set UNSAMPLE_SECRET environment variable or add 'secret' to ~/.unsample/config.yaml
```

**Fix:** Set the `UNSAMPLE_SECRET` environment variable or create a config file:

```bash
export UNSAMPLE_SECRET="your-shared-secret"
```

Or:
```yaml
# ~/.unsample/config.yaml
secret: "your-shared-secret"
```

The secret must be the same value in the CLI and all services running the SDK.

---

## Partial traces (missing spans)

**Symptom:** You see spans from some services but not others.

**Cause 1: Missing Baggage propagation**

The most common cause. Your OTel SDK must propagate both TraceContext AND Baggage:

```go
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},  // ← This is often missing!
))
```

Without Baggage propagation, downstream services never receive the debug token.

**Cause 2: Proxy stripping baggage header**

Some proxies strip the W3C `baggage` header by default.

| Proxy | Fix |
|---|---|
| **Nginx** | Add `proxy_set_header baggage $http_baggage;` |
| **Envoy** | Add `baggage` to `custom_request_headers` |
| **Istio** | Baggage is forwarded by default since Istio 1.16 |
| **AWS ALB** | Baggage is forwarded by default |
| **Cloudflare** | Add `baggage` to "Custom Request Headers" in dashboard |

**Cause 3: Service missing the SDK middleware**

Every service in the call chain needs the Unsample SDK middleware. If Service B doesn't have it, it won't set `debug.trace=true` on its spans, and its sampler won't override to AlwaysOn.

---

## "Trace not found" after debug request

**Symptom:** The CLI shows "⏳ Trace not found" and the link goes to an empty page.

**Cause 1: Ingestion delay**

Traces take 2-5 seconds to appear in Tempo. The CLI polls for up to 30 seconds. If the Collector's batch processor has a long timeout, increase it:

```yaml
# otel-collector-config.yaml
processors:
  batch:
    timeout: 1s  # Don't set higher than 2s for debug traces
```

**Cause 2: Collector not running**

Check that the OTel Collector is accepting traces:
```bash
curl -v http://localhost:4318/v1/traces
# Should return 405 (Method Not Allowed) — means it's running
```

**Cause 3: Tempo not ready**

Check Tempo health:
```bash
curl http://localhost:3200/ready
# Should return "ready"
```

---

## Debug trace not captured (no debug.trace attribute)

**Symptom:** The trace appears in Grafana but has no `debug.trace=true` attribute, and it wasn't routed to the debug pipeline.

**Cause:** The SDK middleware verified the token but the OTel span wasn't active at that point.

Make sure your OTel HTTP handler wraps the Unsample middleware (Unsample reads the span from context):

```go
// ✅ Correct order: OTel creates span first, then Unsample reads it
handler := unsample.Middleware(cfg)(
    otelhttp.NewHandler(mux, "my-service"),
)

// ❌ Wrong order: Unsample runs before OTel creates a span
handler := otelhttp.NewHandler(
    unsample.Middleware(cfg)(mux),
    "my-service",
)
```

---

## Collector OOM crash

**Symptom:** The OTel Collector crashes with out-of-memory errors when debug tracing.

**Cause:** Using `groupbytrace` + `tail_sampling` processors. These buffer ALL spans in memory waiting for trace completion, which is catastrophic for fan-out debug traces.

**Fix:** Use stateless per-span routing (the Unsample approach):

```yaml
# ✅ Correct: stateless routing connector
connectors:
  routing:
    default_pipelines: [traces/production]
    table:
      - statement: route() where attributes["debug.trace"] == true
        pipelines: [traces/debug]

# ❌ Wrong: memory-hungry groupbytrace
processors:
  groupbytrace:
    wait_duration: 10s  # Will OOM on large fan-out traces
```

---

## Docker Compose ports already in use

```
Error: bind: address already in use
```

**Fix:** Check what's using the ports and stop it:

```bash
lsof -i :4317  # OTel Collector
lsof -i :3200  # Tempo
lsof -i :3000  # Grafana
```

Or change the host ports in `docker-compose.yaml`:
```yaml
ports:
  - "14317:4317"  # Map to a different host port
```

---

## zsh: no matches found

```
zsh: no matches found: http://localhost:8080/checkout?user=123
```

**Fix:** Quote URLs that contain `?` (zsh treats it as a glob):

```bash
unsample debug 'http://localhost:8080/checkout?user=123'
```
