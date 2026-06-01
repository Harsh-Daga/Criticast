#!/usr/bin/env bash
# Phase 1 end-to-end demo: record → analyze → pprof (Linux + capabilities required).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/usr/local/go/bin:${PATH:-}"

TRACE="${TRACE:-/tmp/criticast-demo.criticast}"
PPROF="${PPROF:-/tmp/criticast-demo.pb.gz}"
DUR="${DUR:-10s}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "demo-p1: requires Linux (BPF attach). On macOS, run analyzer-only:" >&2
  echo "  ./bin/criticast analyze testdata/traces/golden_chain.jsonl" >&2
  echo "  ./bin/criticast export testdata/traces/golden_chain.jsonl --pprof /tmp/demo.pb.gz" >&2
  exit 1
fi

make bpf go workloads

if ! pgrep -nx httpgo >/dev/null 2>&1; then
  echo "Starting httpgo on :8080..."
  ./bin/httpgo &
  sleep 1
fi
PID="$(pgrep -nx httpgo)"
echo "Target httpgo tgid=$PID"

echo "=== record ==="
sudo -E env PATH="$PATH" ./bin/criticast record \
  --pid "$PID" --dur "$DUR" \
  --bpf-object bpf/collector.bpf.o \
  --go-binary "/proc/$PID/exe" --go-version go1.22.0 \
  --out "$TRACE"

echo "=== analyze (text) ==="
./bin/criticast analyze "$TRACE" --top 10

echo "=== analyze (json) ==="
./bin/criticast analyze "$TRACE" --format json | head -40

echo "=== export pprof ==="
./bin/criticast export "$TRACE" --pprof "$PPROF"

if command -v go >/dev/null; then
  echo "=== go tool pprof -top (first 15 lines) ==="
  go tool pprof -top "$PPROF" 2>/dev/null | head -15 || true
fi

echo ""
echo "demo-p1: OK trace=$TRACE pprof=$PPROF"
