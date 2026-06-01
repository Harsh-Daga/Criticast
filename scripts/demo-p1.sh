#!/usr/bin/env bash
# Phase 1 end-to-end demo: record → analyze → pprof (Linux + capabilities required).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
cd "$ROOT"
criticast_path

TRACE="${TRACE:-/tmp/criticast-demo.criticast}"
PPROF="${PPROF:-/tmp/criticast-demo.pb.gz}"
DUR="${DUR:-10s}"
PORT="${PORT:-8080}"
TARGET_URL="${TARGET_URL:-http://127.0.0.1:${PORT}/}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "demo-p1: requires Linux (BPF attach). On macOS, run analyzer-only:" >&2
  echo "  ./bin/criticast analyze testdata/traces/golden_chain.jsonl" >&2
  echo "  ./bin/criticast export testdata/traces/golden_chain.jsonl --pprof /tmp/demo.pb.gz" >&2
  exit 1
fi

make bpf go workloads

PID="$(ensure_httpgo "$ROOT")"
echo "Target httpgo tgid=$PID exe=$ROOT/bin/httpgo"

echo "=== record (with wrk load) ==="
wrk -t2 -c20 -d"$DUR" "$TARGET_URL" >/tmp/demo-p1-wrk.txt 2>&1 &
WRK_PID=$!
trap 'kill "$WRK_PID" 2>/dev/null || true' EXIT

RECORD_ARGS=(
  --pid "$PID" --dur "$DUR"
  --bpf-object bpf/collector.bpf.o
  --out "$TRACE"
)
if [[ "${GO_UPROBES:-1}" == "1" ]]; then
  RECORD_ARGS+=(--go-binary "$ROOT/bin/httpgo" --go-version "${GO_VERSION:-go1.22.0}")
fi

criticast_sudo ./bin/criticast record "${RECORD_ARGS[@]}" 2>&1 | tee /tmp/demo-p1-record.log

kill "$WRK_PID" 2>/dev/null || true
trap - EXIT

if ! grep -q 'bpf stats:.*emitted=[1-9]' /tmp/demo-p1-record.log; then
  echo "demo-p1: FAIL — no BPF events" >&2
  echo "--- record log ---" >&2
  cat /tmp/demo-p1-record.log >&2
  echo "--- wrk log ---" >&2
  tail -20 /tmp/demo-p1-wrk.txt >&2 || true
  echo "diagnose: ./scripts/sched-smoke.sh  (bpftrace sched activity for httpgo)" >&2
  exit 1
fi

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
