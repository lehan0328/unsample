# CLI Reference

## Commands

### `unsample debug <url>`

Send a debug request to the given URL. Unsample signs an HMAC token, injects it as W3C baggage, sends the request, and polls the trace backend for the complete trace.

```bash
# GET request
unsample debug 'https://api.myapp.com/users/123'

# POST with curl syntax
unsample debug --curl 'curl -X POST -H "Content-Type: application/json" -d "{}" https://api.myapp.com/checkout'
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30s` | HTTP request timeout |
| `--config` | `~/.unsample/config.yaml` | Config file path |
| `--curl` | — | Parse the argument as a curl command |

## Environment Variables

| Variable | Description |
|---|---|
| `UNSAMPLE_SECRET` | HMAC shared secret (**required**) |
| `UNSAMPLE_BACKEND_ENDPOINT` | Trace backend URL for polling |
| `UNSAMPLE_VIEWER_URL` | Trace viewer base URL |

Environment variables override config file values.

## Config File

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

### Config Precedence

1. Environment variables (highest)
2. Config file (`~/.unsample/config.yaml`)
3. Built-in defaults
