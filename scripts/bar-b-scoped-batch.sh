#!/usr/bin/env bash
# Bar B batch: sample ≥50 handler epochs; report pass rate + residual distribution.
# Requires trace recorded after EV_TASK_STATE (make bpf). See docs/p2-bar-b-epoch-path.md.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

MIN_PASS_PCT="${BAR_B_MIN_PASS_PCT:-80}"
MIN_SAMPLES="${BAR_B_MIN_TOKENS:-50}"
SAMPLE_COUNT="${BAR_B_SAMPLE_COUNT:-$MIN_SAMPLES}"
SAMPLE_SEED="${BAR_B_SAMPLE_SEED:-42}"
TRACE="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
GT="${GT_LOG:-/tmp/p0b-gt.log}"
TSV="$(mktemp)"
trap 'rm -f "$TSV"' EXIT

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "bar-b-scoped-batch: requires Linux" >&2
  exit 1
fi
if [[ ! -f "$TRACE" || ! -f "$GT" ]]; then
  echo "bar-b-scoped-batch: need P0B_TRACE and GT_LOG" >&2
  exit 1
fi

pass=0
fail=0
total=0

run_epoch() {
  local token goid from to wall
  token=$1 goid=$2 from=$3 to=$4 wall=$5
  total=$((total + 1))
  if BAR_B_EPOCH_TSV="$TSV" P0B_TRACE="$TRACE" GT_LOG="$GT" \
    "$ROOT/scripts/bar-b-scoped-epoch.sh" "$token" "$goid" "$from" "$to" "$wall"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi
}

mapfile -t rows < <(
  python3 "$ROOT/scripts/bar-b-sample-handlers.py" "$GT" --trace "$TRACE" \
    -n "$SAMPLE_COUNT" --seed "$SAMPLE_SEED"
)
if [[ "${#rows[@]}" -eq 0 ]]; then
  echo "bar-b-scoped-batch: no epochs sampled from ${GT}" >&2
  exit 1
fi
for row in "${rows[@]}"; do
  IFS=$'\t' read -r token goid from to wall <<<"$row"
  echo "=== epoch token=${token} goid=${goid} wall=${wall}ns ==="
  run_epoch "$token" "$goid" "$from" "$to" "$wall"
done

pct=0
if [[ "$total" -gt 0 ]]; then
  pct=$(( pass * 100 / total ))
fi
echo "bar-b-scoped-batch: ${pass}/${total} PASS (${pct}%, ${fail} FAIL)"

python3 - "$TSV" "$pass" "$fail" "$total" "$MIN_PASS_PCT" "$MIN_SAMPLES" <<'PY'
import sys

path, p, f, n = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4])
min_pct, min_n = int(sys.argv[5]), int(sys.argv[6])
rows = []
with open(path) as fp:
    for line in fp:
        parts = line.strip().split("\t")
        if len(parts) < 11:
            continue
        rows.append(
            {
                "status": parts[0],
                "token": parts[1],
                "wall": int(parts[3]),
                "pw": int(parts[4]),
                "residual": int(parts[5]),
                "pct_res": float(parts[6]),
                "occ": int(parts[7]),
            }
        )
if not rows:
    print("bar-b-scoped-batch: no TSV rows (epoch script failed?)")
    sys.exit(1)
residuals = sorted(r["residual"] for r in rows)
p50 = residuals[len(residuals) // 2]
p90 = residuals[int(len(residuals) * 0.9)]
short = [r for r in rows if r["status"] == "FAIL" and r["pw"] < r["wall"] * 0.7]
print(f"residual_ns: median={p50} p90={p90}  short_of_wall_failures={len(short)}")
if short:
    print("  largest shortfalls (wall - path_weight):")
    for r in sorted(short, key=lambda x: x["residual"], reverse=True)[:5]:
        print(
            f"    token={r['token']} residual={r['residual']}ns ({r['pct_res']:.0f}%) "
            f"pw={r['pw']} wall={r['wall']} occupancy={r['occ']}"
        )
print("  interpret: large residual + low occupancy → missing edge; large residual + high occupancy → compute-bound (document or narrow gate)")
pct = 100 * p / n if n else 0
if n < min_n:
    print(f"bar-b-scoped-batch: WARN only {n} samples (want ≥{min_n})", file=sys.stderr)
if pct < min_pct:
    print(f"bar-b-scoped-batch: FAIL pass rate {pct:.0f}% < {min_pct}%", file=sys.stderr)
    sys.exit(1)
if f > 0 and n >= min_n:
    sys.exit(1)
PY
