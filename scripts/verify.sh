#!/usr/bin/env bash
# Local dev gate: go test/vet/build + Linux BPF compile (same as CI).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/usr/local/go/bin:${PATH:-}"
exec "$ROOT/scripts/ci-verify.sh"
