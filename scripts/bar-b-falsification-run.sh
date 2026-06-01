#!/usr/bin/env bash
# Bar B falsification: baseline capture vs A-only worker slowdown (docs/p2-bar-b-falsification.md).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

SLOW_NS="${P0B_WORKER_SLOW_NS:-8000000}"
SLOW_TOKEN="${P0B_WORKER_SLOW_TOKEN:-A}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "bar-b-falsification-run: requires Linux" >&2
  exit 1
fi

cd "$ROOT"
ensure_bin "$ROOT"
ensure_bpf "$ROOT"

run_phase() {
  local label="$1"
  local slow_env="$2"
  echo "=== falsification phase: ${label} ==="
  pkill -x p0b-server wrk 2>/dev/null || true
  sleep 2
  : > /tmp/p0b-gt.log
  eval "${slow_env}" DETACH=1 ./scripts/run-p0b.sh >>/tmp/p0b-gt.log 2>&1
  sleep 2
  PID="$(pgrep -nx p0b-server)"
  CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh &
  LP=$!
  sleep 2
  OUT="/tmp/p0b-trace-${label}.criticast"
  OUT="$OUT" DUR=30s GO_VER=go1.24.0 PID="$PID" ./scripts/record-p0b.sh 2>&1 | tee "/tmp/p0b-record-${label}.log"
  pkill wrk 2>/dev/null || true
  wait "$LP" 2>/dev/null || true
  pkill -x p0b-server 2>/dev/null || true

  export P0B_TRACE="$OUT" GT_LOG=/tmp/p0b-gt.log
  echo "--- batch ${label} ---"
  BAR_B_SAMPLE_COUNT="${BAR_B_FALSIFY_SAMPLES:-30}" \
    ./scripts/bar-b-scoped-batch.sh || true
  for t in A B C; do
    echo "--- live ${label} token=${t} ---"
    ./scripts/bar-b-scoped-live.sh "$t" || true
  done
}

run_phase baseline ""
run_phase "slow-${SLOW_TOKEN}" "P0B_WORKER_SLOW_TOKEN=${SLOW_TOKEN} P0B_WORKER_SLOW_NS=${SLOW_NS} P0B_SLOW_WORKER_ID=${P0B_SLOW_WORKER_ID:-0}"

echo "bar-b-falsification-run: compare A path_weight and dominant edge baseline vs slow-${SLOW_TOKEN}"
echo "  expect: A wall and A path_weight grow; dominant path names worker; B/C stable"
