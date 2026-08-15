# Contributing to Unsample

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
# Clone
git clone https://github.com/lehan0328/unsample.git
cd unsample

# Build
make build

# Run tests
make test

# Install locally
make install
```

**Requirements:** Go 1.21+, Docker (for demo app).

## Running the Demo App

```bash
cd examples/demo-app
docker-compose up --build -d

# Send a debug request
UNSAMPLE_SECRET=demo-secret-do-not-use-in-production \
  unsample debug 'http://localhost:8080/checkout?user=666'

# View traces at http://localhost:3000/explore
```

## Project Structure

```
cmd/unsample/          CLI entry point (Cobra)
internal/cli/          CLI logic (debug, init, config, output)
internal/collector/    OTel Collector processor
internal/token/        HMAC token generation/verification
internal/trace/        Trace polling + Tempo integration
internal/version/      Build version injection
sdk/go/                Go SDK middleware (separate go.mod)
examples/demo-app/     3-service demo (gateway → billing → notification)
docker/                Collector, Tempo, Grafana configs
```

## Making Changes

1. **Fork** the repo and create a branch from `main`
2. **Write tests** for any new functionality
3. **Run the full test suite:** `make test`
4. **Open a PR** with a clear description of what changed and why

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- All public APIs must have doc comments
- Error messages should be lowercase, no trailing punctuation
- Use `fmt.Errorf("doing thing: %w", err)` for error wrapping

## Good First Issues

Look for issues labeled [`good first issue`](https://github.com/lehan0328/unsample/labels/good%20first%20issue). Some ideas:

- Add `unsample debug --method POST` flag
- Add `unsample debug --header "Key: Value"` flag
- Shell completion for bash/zsh/fish
- `unsample init --minimal` (config only, no Docker Compose)
- Python/Node.js SDK middleware

## Running Specific Tests

```bash
# All tests
make test

# Single package
go test -v ./internal/token/

# SDK tests (separate module)
cd sdk/go && go test -v -race ./...

# Benchmarks
make bench
```

## Release Process

Releases are automated via GoReleaser. To create a release:

```bash
git tag v0.1.0
git push --tags
```

The [release workflow](.github/workflows/release.yml) builds cross-platform binaries and publishes a GitHub Release.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
