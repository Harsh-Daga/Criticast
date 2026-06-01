#!/usr/bin/env bash
# Verify scheduler tracepoints see the criticast httpgo TGID under load (bpftrace).
# If this passes but record shows bpf emitted=0, file a criticast BPF bug.
# If this fails, host policy or tracing is blocked (lockdown, paranoid, etc.).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
cd "$ROOT"
criticast_path

PORT="${PORT:-8080}"
DUR="${DUR:-5}"
TARGET_URL="${TARGET_URL:-http://127.0.0.1:${PORT}/}"

need bpftrace
need wrk

PID="$(ensure_httpgo "$ROOT")"
echo "sched-smoke: tgid=$PID url=$TARGET_URL dur=${DUR}s"

wrk -t2 -c10 -d"${DUR}s" "$TARGET_URL" >/tmp/sched-smoke-wrk.txt 2>&1 &
WRK_PID=$!
trap 'kill "$WRK_PID" 2>/dev/null || true' EXIT

OUT="/tmp/sched-smoke.bt.out"
# Match all threads of the Go process (sched_switch args are per-thread TIDs, not TGID).
sudo bpftrace -e "
tracepoint:sched:sched_switch
/str(args->next_comm) == \"httpgo\" || str(args->prev_comm) == \"httpgo\"/
{ @n++; }
interval:s:${DUR} { exit(); }
END { printf(\"sched_switch hits for comm httpgo (tgid ${PID}): %d\\n\", @n); }
" 2>"$OUT" || {
  echo "sched-smoke: bpftrace failed:" >&2
  cat "$OUT" >&2
  exit 1
}

kill "$WRK_PID" 2>/dev/null || true
trap - EXIT

if grep -q 'sched_switch hits for tgid.*: 0' "$OUT" 2>/dev/null; then
  echo "sched-smoke: FAIL — kernel tracepoints saw 0 switches for target" >&2
  echo "  Check: kernel lockdown, perf_event_paranoid, or wrong PID" >&2
  exit 1
fi
grep 'sched_switch hits' "$OUT" || cat "$OUT"
echo "sched-smoke: OK"
