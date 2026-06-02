#!/usr/bin/env bash
# One-terminal p0b capture + Bar B + mechanism eval. Run from repo root (Linux, sudo for record).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

cd "$ROOT"
ensure_bin "$ROOT"
ensure_bpf "$ROOT"

echo "=== build ==="
make bpf go

echo "=== stop old workloads ==="
pkill -x p0b-server wrk httpgo 2>/dev/null || true
sleep 2

echo "=== start p0b + fresh GT log ==="
: > /tmp/p0b-gt.log
DETACH=1 ./scripts/run-p0b.sh >>/tmp/p0b-gt.log 2>&1
sleep 2
PID="$(pgrep -nx p0b-server)"
EXE="$(readlink -f "/proc/${PID}/exe")"
echo "p0b-server PID=${PID} exe=${EXE}"
curl -sf "http://127.0.0.1:${PORT:-8080}/health" >/dev/null

echo "=== load + record (30s) ==="
CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh &
LOAD_PID=$!
sleep 2
OUT=/tmp/p0b-trace.criticast DUR=30s GO_VER=go1.24.0 PID="${PID}" \
  ./scripts/record-p0b.sh 2>&1 | tee /tmp/p0b-record.log
pkill wrk 2>/dev/null || true
wait "${LOAD_PID}" 2>/dev/null || true

CONSUMED="$(sed -n 's/^userspace: consumed=\([0-9]*\).*/\1/p' /tmp/p0b-record.log | tail -1)"
if [[ -z "${CONSUMED}" || "${CONSUMED}" -lt 100000 ]]; then
  echo "error: trace too small (consumed=${CONSUMED:-0}); check /tmp/p0b-record.log" >&2
  exit 1
fi

echo "=== trace clock bases ==="
head -1 /tmp/p0b-trace.criticast | python3 -c "import json,sys; h=json.load(sys.stdin); print('ktime_base',h.get('ktime_base_ns')); print('wall_base',h.get('wall_base_utc'))"

echo "=== Bar B literal ==="
./scripts/bar-b-scoped-live.sh A | tee /tmp/bar-b-A.log
./scripts/bar-b-scoped-live.sh B | tee /tmp/bar-b-B.log
./scripts/bar-b-scoped-live.sh C | tee /tmp/bar-b-C.log

echo "=== mechanism eval ==="
./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.criticast --mode all 2>&1 | tee /tmp/p0b-eval.log
grep '^trace join:' /tmp/p0b-eval.log || true
grep -A10 '^=== trace-joined ===' /tmp/p0b-eval.log || true

echo "=== validate-bar-b ==="
P0B_TRACE=/tmp/p0b-trace.criticast GT_LOG=/tmp/p0b-gt.log ./scripts/validate-bar-b.sh 2>&1 | tee /tmp/validate-bar-b.log

echo "=== done ==="
