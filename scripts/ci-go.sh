#!/usr/bin/env bash
# CI Go gate (no root, no BPF). Mirrors the fast path contributors need on every PR.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/usr/local/go/bin:${PATH:-}"

echo "=== go version ==="
go version

echo "=== go test ==="
# BPF object test runs only in linux-bpf job (needs working bpftool from ci-install-deps).
export CRITICAST_BPF_GATE=0
go test ./...
go test ./testdata/p0a/httpgo ./testdata/p0b/server

echo "=== go vet ==="
go vet ./...

echo "=== build ==="
make go workloads

echo "ci-go: OK"
