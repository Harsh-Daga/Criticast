#!/usr/bin/env bash
# bpftrace wake-rate spike: httpgo + wrk + criticast-spike.bt (Linux). See docs/GETTING_STARTED.md.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMM="${COMM:-httpgo}"
PORT="${PORT:-8080}"
DURATION_SEC="${DURATION_SEC:-30}"
WRK_THREADS="${WRK_THREADS:-4}"
WRK_CONN="${WRK_CONN:-100}"
WRK_DUR="${WRK_DUR:-${DURATION_SEC}s}"
OUT="${OUT:-results/phase0/spike-$(date -u +%Y%m%dT%H%M%SZ).log}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }
need bpftrace
need wrk
need go

if [[ ! -f /sys/kernel/btf/vmlinux ]]; then
  echo "BTF missing at /sys/kernel/btf/vmlinux"
  exit 1
fi

mkdir -p results/phase0 bin

# Stop prior httpgo from an earlier spike/run.
pkill -x httpgo 2>/dev/null || pkill -f '/bin/httpgo' 2>/dev/null || true
sleep 1

if [[ ! -x bin/httpgo ]]; then
  echo "Building workloads (httpgo via go.work)..."
  make workloads
fi

echo "Starting httpgo on :${PORT}..."
PORT="$PORT" exec -a "$COMM" ./bin/httpgo &
SVC_PID=$!
trap 'kill $SVC_PID 2>/dev/null || true' EXIT

for i in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${PORT}/" >/dev/null; then
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo "httpgo did not become ready on :${PORT}"
    exit 1
  fi
  sleep 0.2
done

ACTUAL_COMM="$(cat /proc/${SVC_PID}/comm | tr -d '\n' 2>/dev/null || echo "$COMM")"
echo "PID=${SVC_PID} comm=${ACTUAL_COMM} (requested COMM=${COMM})"
if [[ "$ACTUAL_COMM" != "$COMM" ]]; then
  echo "WARN: comm mismatch — set COMM=${ACTUAL_COMM} for bpftrace filter"
  COMM="$ACTUAL_COMM"
fi

echo "Starting wrk..."
wrk -t"${WRK_THREADS}" -c"${WRK_CONN}" -d"${WRK_DUR}" "http://127.0.0.1:${PORT}/" &
WRK_PID=$!
sleep 2

BPFTRACE=(bpftrace)
if [[ "$(id -u)" -ne 0 ]]; then
  BPFTRACE=(sudo -E bpftrace)
fi

SPIKE_BT="$(mktemp "${TMPDIR:-/tmp}/criticast-spike.XXXXXX.bt")"
trap 'rm -f "$SPIKE_BT"' EXIT
# bpftrace has no $ENV in scripts; substitute placeholders from shell.
DEADLINE_NS=$((DURATION_SEC * 1000000000))
sed -e "s/__CRITICAST_SPIKE_COMM__/${COMM}/g" \
    -e "s/__CRITICAST_SPIKE_DURATION__/${DURATION_SEC}/g" \
    -e "s/__CRITICAST_SPIKE_DEADLINE_NS__/${DEADLINE_NS}/g" \
    scripts/criticast-spike.bt >"$SPIKE_BT"

echo "Running bpftrace spike (${DURATION_SEC}s, comm=${COMM}) -> ${OUT}"
"${BPFTRACE[@]}" "$SPIKE_BT" 2>&1 | tee "$OUT"

kill "$WRK_PID" 2>/dev/null || true
wait "$WRK_PID" 2>/dev/null || true

# Release :8080 before other services (trap also runs on exit).
kill "$SVC_PID" 2>/dev/null || true
pkill -x httpgo 2>/dev/null || pkill -f '/bin/httpgo' 2>/dev/null || true
sleep 1

echo ""
echo "Done. Log: $OUT"
