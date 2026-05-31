#!/usr/bin/env bash
# Overhead benchmark (Linux). Baseline vs criticast record for three modes.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
criticast_path

TARGET_URL="${TARGET_URL:-http://127.0.0.1:8080/}"
RUNS="${RUNS:-5}"
DUR="${DUR:-30s}"

cd "$ROOT"
ensure_bpf "$ROOT"
ensure_bin "$ROOT"

PID="$(ensure_httpgo "$ROOT")"
export PID
log() { echo "$*" >&2; }
log "bench: httpgo PID=$PID url=$TARGET_URL runs=$RUNS dur=$DUR"
log "bench: probing ${TARGET_URL} ..."
if ! curl -sf --max-time 5 "${TARGET_URL}" >/dev/null; then
  log "error: workload not reachable at ${TARGET_URL}"
  exit 1
fi
log "bench: workload OK — starting wrk loops (this takes a while)"

modes=(
  "full:1us:1"
  "min-block:50us:1"
  "sampled:1us:100"
)

for spec in "${modes[@]}"; do
  IFS=: read -r name min sample <<<"$spec"
  log "=== mode=$name min-block=$min sample=$sample dur=$DUR ==="
  for i in $(seq 1 "$RUNS"); do
    log "run $i baseline"
    wrk -t4 -c100 -d"$DUR" "$TARGET_URL" | tee "/tmp/p0a-${name}-base-${i}.txt"
    log "run $i treatment (record + wrk)"
    criticast_sudo ./bin/criticast record --pid "$PID" --dur "$DUR" \
      --min-block "$min" --sample "$sample" --bpf-object bpf/collector.bpf.o &
    rec=$!
    wrk -t4 -c100 -d"$DUR" "$TARGET_URL" | tee "/tmp/p0a-${name}-treat-${i}.txt"
    wait "$rec" || true
  done
done

log "bench: done — update results/phase0/p0a-overhead.md if publishing"
