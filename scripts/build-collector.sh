#!/usr/bin/env bash
# Build a custom OTel Collector binary with the unsample processor.
#
# Prerequisites:
#   go install go.opentelemetry.io/collector/cmd/builder@latest
#
# Usage:
#   ./scripts/build-collector.sh
#
# Output:
#   ./collector-build/unsample-collector

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/collector-build"

echo "=== Building custom OTel Collector ==="

# Check for OCB binary.
if ! command -v builder &> /dev/null; then
    echo "Installing OCB (OpenTelemetry Collector Builder)..."
    go install go.opentelemetry.io/collector/cmd/builder@latest
fi

# Build the custom Collector.
echo "Running OCB with builder-config.yaml..."
builder --config="${PROJECT_ROOT}/builder-config.yaml"

echo ""
echo "=== Build complete ==="
echo "Binary: ${BUILD_DIR}/unsample-collector"
echo ""
echo "To run locally:"
echo "  ${BUILD_DIR}/unsample-collector --config=docker/otel-collector-config.yaml"
echo ""
echo "To build a Docker image:"
echo "  docker build -t unsample-collector -f docker/Dockerfile.collector ."
