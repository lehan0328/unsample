# Quickstart Guide

Get Unsample running and debug your first trace in under 5 minutes.

## Prerequisites

- Go 1.23+
- Docker and Docker Compose
- A service instrumented with OpenTelemetry (or use the demo app)

## Step 1: Install the CLI

```bash
go install github.com/unsample/unsample/cmd/unsample@latest
```

Verify:
```bash
unsample --version
```

## Step 2: Start the observability stack

Clone the repo and start the local stack:

```bash
git clone https://github.com/lehan0328/unsample.git
cd unsample/docker
docker-compose up -d
```

This starts three containers:

| Service | Port | Purpose |
|---|---|---|
| OTel Collector | 4317 (gRPC), 4318 (HTTP) | Receives traces, routes debug vs production |
| Grafana Tempo | 3200 | Trace storage backend |
| Grafana | 3000 | Trace viewer UI (no login) |

Verify everything is healthy:
```bash
curl -s http://localhost:3200/ready   # Tempo → "ready"
curl -s http://localhost:3000/api/health  # Grafana → {"commit":"...","database":"ok"}
```

## Step 3: Add the SDK to your service

Install the SDK:
```bash
go get github.com/unsample/unsample/sdk/go
```

Add the middleware to your HTTP server:

```go
package main

import (
    "net/http"
    "os"

    unsample "github.com/unsample/unsample/sdk/go"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, world!"))
    })

    // Add Unsample middleware — 2 lines.
    handler := unsample.Middleware(unsample.Config{
        Secret: os.Getenv("UNSAMPLE_SECRET"),
    })(mux)

    http.ListenAndServe(":8080", handler)
}
```

For gRPC:

```go
import (
    unsample "github.com/unsample/unsample/sdk/go"
    "google.golang.org/grpc"
)

srv := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        unsample.UnaryServerInterceptor(unsample.Config{
            Secret: os.Getenv("UNSAMPLE_SECRET"),
        }),
    ),
    grpc.ChainStreamInterceptor(
        unsample.StreamServerInterceptor(unsample.Config{
            Secret: os.Getenv("UNSAMPLE_SECRET"),
        }),
    ),
)
```

**Important:** Set `UNSAMPLE_SECRET` to the same value in all services and the CLI.

## Step 4: Configure your OTel SDK

Make sure your services export traces to the Collector and propagate **both** TraceContext and Baggage:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
)

// CRITICAL: Register both propagators.
// Without Baggage propagation, the debug token won't flow downstream.
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
))
```

For HTTP clients calling other services, use `otelhttp.NewTransport`:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

client := &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}
```

This automatically propagates the trace context and baggage to downstream services.

## Step 5: Debug a request

```bash
export UNSAMPLE_SECRET="your-shared-secret"
unsample debug 'http://localhost:8080/'
```

Click the link in the output → full trace in Grafana.

## Try the Demo App

If you don't have a service handy, use the included 3-service demo:

```bash
cd unsample/examples/demo-app
docker-compose up --build -d

UNSAMPLE_SECRET=demo-secret-do-not-use-in-production \
  unsample debug 'http://localhost:8080/checkout?user=123'
```

## Next Steps

- [Troubleshooting](troubleshooting.md) — common issues and fixes
- [Architecture](architecture.md) — how the system works under the hood
- [README](../README.md) — full CLI reference and FAQ
