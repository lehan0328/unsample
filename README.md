<h1 align="center">
  ⚡ Unsample
</h1>

<p align="center">
  <strong>On-demand debug tracing for OpenTelemetry.</strong><br/>
  Like a breakpoint, but for distributed systems.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg?colorA=1b2528&colorB=ccfbc7&style=for-the-badge" /></a>
  <a href="https://github.com/lehan0328/unsample/actions"><img src="https://img.shields.io/badge/tests-102%20passed-brightgreen.svg?colorA=1b2528&colorB=b2d3ff&style=for-the-badge" /></a>
  <a href="https://pkg.go.dev/github.com/unsample/unsample"><img src="https://img.shields.io/badge/go-reference-blue.svg?colorA=1b2528&colorB=FFF8BA&style=for-the-badge&logo=go" /></a>
</p>

---

Unsample forces OpenTelemetry to capture **100% of spans** for a single request — across every microservice in the call chain — without changing your sampling config.

```bash
$ unsample debug 'https://api.myapp.com/checkout?user=123'

✅ Trace captured (5 spans, 847ms)
   → http://localhost:3000/explore?traceId=4bf92f3577b34da6

   gateway          12ms  ✅ OK
   billing-service  340ms ❌ ERROR  subscription_not_found
   notification      8ms  ✅ OK
```

## Getting Started

Read through the [Quickstart Guide](docs/quickstart.md) or jump straight to the demo:

```bash
git clone https://github.com/lehan0328/unsample.git
cd unsample/examples/demo-app
docker-compose up --build -d

UNSAMPLE_SECRET=demo-secret-do-not-use-in-production \
  unsample debug 'http://localhost:8080/checkout?user=123'
```

## Packages

| Component | Install | Source | Docs |
|---|---|---|---|
| **CLI** | `go install github.com/unsample/unsample/cmd/unsample@latest` | [cmd/unsample](cmd/unsample) | [CLI Reference](#cli-reference) |
| **Go SDK** (HTTP) | `go get github.com/unsample/unsample/sdk/go` | [sdk/go](sdk/go) | [Quickstart](docs/quickstart.md) |
| **Go SDK** (gRPC) | Same package | [sdk/go](sdk/go) | [Quickstart](docs/quickstart.md) |
| **Collector** | Docker image | [docker/](docker) | [Architecture](docs/architecture.md) |

## How It Works

```
Developer terminal                Service A → B → C                OTel Collector
─────────────────                 ─────────────────                ──────────────
unsample debug <url>              SDK middleware:                  Route by attribute:
  → HMAC-sign token                 → Verify token                  debug.trace=true → Tempo (debug)
  → Inject as W3C baggage           → Override sampler to 100%      everything else  → Tempo (prod)
  → Send HTTP request               → Set debug.trace=true
  → Poll for trace                  → Propagate downstream
```

**Three components, 2 lines to add per service:**

1. **CLI** → Signs a token, injects it, sends the request
2. **SDK** → Verifies the token, overrides the sampler, propagates
3. **Collector** → Routes debug spans to a separate backend

### Performance

```
BenchmarkMiddleware_HotPath     170,694,930    7.25 ns/op    0 B/op    0 allocs/op
```

Zero allocations on the hot path. 99.99% of requests exit in <10ns.

## CLI Reference

```bash
# GET request
unsample debug 'https://api.myapp.com/users/123'

# POST with curl syntax
unsample debug --curl 'curl -X POST -H "Content-Type: application/json" -d "{}" https://api.myapp.com/checkout'
```

| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30s` | HTTP request timeout |
| `--config` | `~/.unsample/config.yaml` | Config file path |

| Environment Variable | Description |
|---|---|
| `UNSAMPLE_SECRET` | HMAC shared secret (required) |
| `UNSAMPLE_BACKEND_ENDPOINT` | Trace backend URL for polling |
| `UNSAMPLE_VIEWER_URL` | Trace viewer base URL |

### Config File

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

## Security

| Safeguard | Prevents |
|---|---|
| HMAC-signed tokens | Unauthenticated flooding |
| 2h token expiry | Replay attacks |
| 10 debug/min rate limit | Storage saturation |
| Never retry on throttle | Retry storm DoS |
| 64KB payload truncation | Stack overflow from recursive payloads |
| Zero-alloc hot path | Production latency regression |
| Stateless per-span routing | Collector OOM crash |
| Separate debug backend (7d TTL) | Cost isolation + PII separation |

## Documentation

| Doc | Description |
|---|---|
| [Quickstart](docs/quickstart.md) | Install → first trace in 5 minutes |
| [Troubleshooting](docs/troubleshooting.md) | Proxy stripping, partial traces, OOM |
| [Architecture](docs/architecture.md) | How the system works under the hood |

## FAQ

<details>
<summary><strong>Does this add latency to production?</strong></summary>
No. The SDK check is 7.25ns with 0 allocations. Only debug requests pay the HMAC cost.
</details>

<details>
<summary><strong>What if my proxy strips baggage headers?</strong></summary>
Configure your proxy to forward the W3C <code>baggage</code> header. See <a href="docs/troubleshooting.md">Troubleshooting</a>.
</details>

<details>
<summary><strong>Can two developers debug simultaneously?</strong></summary>
Yes. Each session has a unique trace ID and token.
</details>

<details>
<summary><strong>What backends are supported?</strong></summary>
Any OTel-compatible backend. Tested with Grafana Tempo. The Collector can export to Jaeger, Datadog, Honeycomb, etc.
</details>

<details>
<summary><strong>Do I need to change my sampling config?</strong></summary>
No. Unsample overrides the sampler per-request at the SDK level.
</details>

## Status

| Component | Status |
|---|---|
| CLI (`unsample debug`) | ✅ Stable |
| Go SDK (HTTP + gRPC) | ✅ Stable |
| Collector routing | ✅ Stable |
| Trace polling | ✅ Stable |
| Demo app (3-service) | ✅ Stable |
| Docs | 🔄 In progress |
| `unsample init` | ⬜ Planned |
| Homebrew / GoReleaser | ⬜ Planned |

## License

MIT — see [LICENSE](LICENSE).
