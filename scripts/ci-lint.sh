#!/usr/bin/env bash
# CI lint gate — install golangci-lint with the same Go toolchain as the module (avoids
# "undefined: profile" and stdlib typecheck noise from version skew).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="$(go env GOPATH)/bin:${PATH:-}"
# Build the linter with the module toolchain (go.mod: toolchain go1.24.0).
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.24.0}"

echo "=== go version ==="
go version

echo "=== go mod download ==="
go mod download

echo "=== install golangci-lint ==="
GOBIN="$(go env GOPATH)/bin"
rm -f "${GOBIN}/golangci-lint"
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
golangci-lint version

echo "=== golangci-lint ==="
golangci-lint run --timeout=5m

echo "ci-lint: OK"
