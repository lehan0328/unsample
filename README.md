<p align="center">
  <h1 align="center">⚡ Unsample</h1>
  <p align="center">
    <strong>On-demand debug tracing for OpenTelemetry — like a breakpoint, but for distributed systems.</strong>
  </p>
  <p align="center">
    <a href="#quickstart">Quickstart</a> •
    <a href="#how-it-works">How It Works</a> •
    <a href="#cli-reference">CLI Reference</a> •
    <a href="docs/architecture.md">Architecture</a> •
    <a href="#faq">FAQ</a>
  </p>
</p>

---

**Ever been unable to reproduce a bug because your sampler dropped the trace?**

Unsample forces OpenTelemetry to capture 100% of spans for a single request — across every microservice in the call chain — without changing your sampling config.

```bash
# Debug a specific request. Every span, every service, guaranteed.
$ unsample debug 'https://api.myapp.com/checkout?user=123'

─── HTTP Response ───────────────────────────────
HTTP/1.1 500 Internal Server Error
{"error": "subscription_not_found"}

─── Debug Trace ─────────────────────────────────
✅ Trace captured (5 spans, 847ms)
   → http://localhost:3000/explore?traceId=4bf92f3577b34da6

   gateway          12ms  ✅ OK
   billing-service  340ms ❌ ERROR  subscription_not_found
     └─ postgres    312ms       SELECT * FROM subscriptions...
   notification      8ms  ✅ OK
─────────────────────────────────────────────────
```

## Why Unsample?

| Problem | Without Unsample | With Unsample |
|---|---|---|
| **Dropped traces** | "I can't reproduce the bug — the sampler didn't capture it" | Force-capture the exact request |
| **Partial traces** | "I see the API gateway span but the DB query is missing" | 100% of spans across all services |
| **Config changes** | "Let me change the sampling rate to 100% and redeploy..." | Zero config changes, instant |
| **Security** | "Anyone could set `debug=true` and flood our backend" | HMAC-signed tokens, rate limited |

## Quickstart

### 1. Install the CLI

```bash
go install github.com/unsample/unsample/cmd/unsample@latest
```

### 2. Start the observability stack

```bash
git clone https://github.com/lehan0328/unsample.git
cd unsample/docker
docker-compose up -d
```

This starts:
- **OTel Collector** (port 4317) — receives and routes traces
- **Grafana Tempo** (port 3200) — stores traces
- **Grafana** (port 3000) — trace viewer UI (no login needed)

### 3. Add the SDK to your services

```bash
go get github.com/unsample/unsample/sdk/go
```

```go
import unsample "github.com/unsample/unsample/sdk/go"

// HTTP middleware — 2 lines to add.
handler := unsample.Middleware(unsample.Config{
    Secret: os.Getenv("UNSAMPLE_SECRET"),
})(yourHandler)
```

```go
// gRPC interceptor — also 2 lines.
srv := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        unsample.UnaryServerInterceptor(unsample.Config{
            Secret: os.Getenv("UNSAMPLE_SECRET"),
        }),
    ),
)
```

### 4. Debug a request

```bash
export UNSAMPLE_SECRET="your-shared-secret"
unsample debug 'https://your-service.com/api/endpoint'
```

Open the link in the output → full trace in Grafana. Done.

## How It Works

```
                    ┌──────────────┐
                    │ unsample CLI │
                    │  generates   │
                    │  HMAC token  │
                    └──────┬───────┘
                           │ baggage: unsample-debug=<token>
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         Service A    Service B    Service C
         (SDK)        (SDK)        (SDK)
         verify()     verify()     verify()
         sample=100%  sample=100%  sample=100%
              │            │            │
              └────────────┼────────────┘
                           ▼
                    ┌──────────────┐
                    │     OTel     │
                    │  Collector   │
                    │ route debug  │
                    │ spans to     │
                    │ debug backend│
                    └──────────────┘
```

**Three components:**

1. **CLI** → Generates an HMAC-signed token, injects it as W3C `baggage`, sends the request
2. **SDK** (in each service) → Verifies the token, overrides the sampler to capture 100% of spans, propagates the debug flag downstream
3. **Collector** → Routes debug-flagged spans to a separate backend (7-day retention)

### Performance

The SDK check is **sub-microsecond with zero allocations** on the hot path:

```
BenchmarkMiddleware_HotPath-10     170,694,930    7.25 ns/op    0 B/op    0 allocs/op
```

99.99% of requests (non-debug) exit in <10ns. Only actual debug requests pay the HMAC verification cost.

## CLI Reference

### `unsample debug <url>`

Send a debug-traced request and display the trace.

```bash
# Simple GET request
unsample debug 'https://api.myapp.com/users/123'

# With curl syntax (POST, headers, body)
unsample debug --curl 'curl -X POST -H "Content-Type: application/json" -d "{\"item\":\"abc\"}" https://api.myapp.com/checkout'
```

**Flags:**
| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30s` | HTTP request timeout |
| `--config` | `~/.unsample/config.yaml` | Config file path |

**Environment variables:**
| Variable | Description |
|---|---|
| `UNSAMPLE_SECRET` | HMAC shared secret (required) |
| `UNSAMPLE_BACKEND_ENDPOINT` | Trace backend URL for polling |
| `UNSAMPLE_VIEWER_URL` | Trace viewer base URL |

### Configuration

```yaml
# ~/.unsample/config.yaml
secret: "your-shared-secret"

backend:
  type: tempo
  endpoint: http://localhost:3200

viewer:
  type: grafana
  url: http://localhost:3000
```

## Demo

Try the included 3-service demo app:

```bash
cd examples/demo-app
docker-compose up --build -d

# Happy path
UNSAMPLE_SECRET=demo-secret-do-not-use-in-production \
  unsample debug 'http://localhost:8080/checkout?user=123'

# Error path (billing returns 500)
UNSAMPLE_SECRET=demo-secret-do-not-use-in-production \
  unsample debug 'http://localhost:8080/checkout?user=666'
```

View traces at: http://localhost:3000/explore

## Security

Unsample is designed with defense-in-depth, informed by production incidents at scale:

| Safeguard | What It Prevents |
|---|---|
| HMAC-signed tokens | Unauthenticated debug flooding |
| Time-bound tokens (2h expiry) | Token replay attacks |
| Rate limiting (10 debug/min) | Storage saturation from fan-out |
| Never retry on throttle | Retry storm DoS |
| Payload truncation (64KB) | Stack overflow from recursive payloads |
| Zero-alloc hot path | Latency regression on production traffic |
| Stateless per-span routing | OOM crash from trace buffering |
| Separate debug backend (7-day TTL) | Cost isolation + PII separation |

## FAQ

<details>
<summary><strong>Does this add latency to production requests?</strong></summary>

No measurable impact. The SDK check is a single O(1) key lookup on the hot path. Benchmarked at 7.25ns with 0 allocations — well under 1 microsecond. Only debug requests (0.01%) pay the HMAC verification cost.
</details>

<details>
<summary><strong>What if my API gateway strips baggage headers?</strong></summary>

Some proxies (Nginx, Envoy, Istio) strip the W3C `baggage` header by default. You need to configure them to forward it. See [Troubleshooting](docs/troubleshooting.md).
</details>

<details>
<summary><strong>Can two developers debug simultaneously?</strong></summary>

Yes. Each debug session generates a unique trace ID and token. Sessions are completely independent.
</details>

<details>
<summary><strong>What trace backends are supported?</strong></summary>

Unsample works with any OTel-compatible backend. Tested with Grafana Tempo. The Collector config can export to Jaeger, Datadog, Honeycomb, etc.
</details>

<details>
<summary><strong>Do I need to change my sampling config?</strong></summary>

No. Unsample overrides the sampler per-request at the SDK level. Your production sampling config is untouched.
</details>

## Project Status

Unsample is in **active development**. The core engine (CLI, SDK, Collector routing) is complete and tested with 102 unit tests + 8 benchmarks.

| Component | Status |
|---|---|
| CLI (`unsample debug`) | ✅ Stable |
| Go SDK (HTTP middleware) | ✅ Stable |
| Go SDK (gRPC interceptor) | ✅ Stable |
| OTel Collector routing | ✅ Stable |
| Trace polling + summary | ✅ Stable |
| Demo app (3-service) | ✅ Stable |
| Documentation | 🔄 In progress |
| `unsample init` (scaffolding) | ⬜ Planned |
| Homebrew / GoReleaser | ⬜ Planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for build instructions and guidelines.

## License

MIT — see [LICENSE](LICENSE).
