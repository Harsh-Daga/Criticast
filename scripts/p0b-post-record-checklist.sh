#!/usr/bin/env bash
# After p0b record with EV_TASK_STATE: probe, batch residuals, mechanism eval.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

TRACE="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
GT="${GT_LOG:-/tmp/p0b-gt.log}"

cd "$ROOT"
ensure_bin "$ROOT"

echo "=== record log (running stat) ==="
grep -E 'bpf stats:|running=' /tmp/p0b-record.log 2>/dev/null | tail -3 || true

echo "=== mechanism eval ==="
./bin/criticast eval --gt-log "$GT" --trace "$TRACE" --mode all | tee /tmp/p0b-eval.log
grep '^trace join:' /tmp/p0b-eval.log || true

echo "=== probe epoch A ==="
P0B_TRACE="$TRACE" GT_LOG="$GT" ./scripts/bar-b-probe-epoch.sh A

echo "=== 50-sample batch (honest gate; failures may be real) ==="
P0B_TRACE="$TRACE" GT_LOG="$GT" ./scripts/bar-b-scoped-batch.sh

echo "=== overhead reminder ==="
echo "Re-run ./scripts/bench-p0a.sh after EV_TASK_STATE; update results/phase0/p0a-overhead.md (prior 0.93% is stale)."
