#!/usr/bin/env bash
# Bar B literal for one GT handler epoch (token, goid, scope-from, scope-to).
# Prints residual_ns = wall - path_weight (information; do not back-fill from wall).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
# shellcheck source=lib/analyze_bar_b.sh
source "$ROOT/scripts/lib/analyze_bar_b.sh"

TOKEN="${1:?token}"
GOID="${2:?handler_goid}"
T_FROM="${3:?scope_from RFC3339}"
T_TO="${4:?scope_to RFC3339}"
WALL_NS="${5:-0}"

TRACE="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
GT="${GT_LOG:-/tmp/p0b-gt.log}"
SLACK_PCT="${BAR_B_SLACK_PCT:-30}"
MAX_PATH_RATIO="${BAR_B_MAX_PATH_RATIO:-3.0}"
TSV_OUT="${BAR_B_EPOCH_TSV:-}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "bar-b-scoped-epoch: requires Linux" >&2
  exit 1
fi
if [[ ! -f "$TRACE" || ! -f "$GT" ]]; then
  echo "bar-b-scoped-epoch: need TRACE and GT_LOG" >&2
  exit 1
fi

cd "$ROOT"
ensure_bin "$ROOT"

if [[ "$WALL_NS" == "0" ]]; then
  WALL_NS="$(python3 - "$T_FROM" "$T_TO" <<'PY'
from datetime import datetime
t0 = datetime.fromisoformat(__import__("sys").argv[1].replace("Z", "+00:00"))
t1 = datetime.fromisoformat(__import__("sys").argv[2].replace("Z", "+00:00"))
print(int((t1 - t0).total_seconds() * 1e9))
PY
)"
fi

OUT_JSON="$(mktemp)"
trap 'rm -f "$OUT_JSON"' EXIT

PAD_MS="$(bar_b_pad_ms "$WALL_NS")"
bar_b_analyze "$TRACE" "$GT" "$TOKEN" "$GOID" "$T_FROM" "$T_TO" "$PAD_MS" "$OUT_JSON"

python3 - "$OUT_JSON" "$WALL_NS" "$SLACK_PCT" "$MAX_PATH_RATIO" "$TOKEN" "$GOID" "$TSV_OUT" <<'PY'
import json, sys

out_path, wall = sys.argv[1], int(sys.argv[2])
slack_pct, max_ratio = float(sys.argv[3]), float(sys.argv[4])
token, goid, tsv_out = sys.argv[5], sys.argv[6], sys.argv[7]

with open(out_path) as f:
    j = json.load(f)

pw = int(j.get("path_weight_ns", 0))
wall_json = int(j.get("epoch_wall_ns", 0)) or wall
occ = int(j.get("handler_occupancy_ns", 0))
residual = int(j.get("residual_ns", max(0, wall_json - pw)))
path = j.get("critical_path") or []
path_edges = len(path)
scoped = int(j.get("scoped_edge_count", 0))
rg = int(j.get("request_goid_count", 0))

slack = max(int(wall_json * slack_pct / 100), 1_000_000)
lo, hi = max(wall_json - slack, 0), wall_json + slack
ratio = pw / wall_json if wall_json else 0
pct_res = (100.0 * residual / wall_json) if wall_json else 0.0
dominant = ("WC_CHAN", "WC_CONN_POOL", "WC_MUTEX", "WC_NET", "WC_FUTEX", "WC_SEM")
has_dom = any(e.get("wait_class") in dominant for e in path)
in_band = lo <= pw <= hi
ok = path_edges >= 1 and has_dom and in_band and ratio <= max_ratio
status = "PASS" if ok else "FAIL"
print(
    f"epoch token={token} goid={goid} path_weight={pw} wall={wall_json} "
    f"residual_ns={residual} ({pct_res:.0f}% of wall) handler_occupancy_ns={occ} "
    f"path_edges={path_edges} scoped={scoped} request_goids={rg} ratio={ratio:.2f} {status}"
)
if residual > slack and not ok:
    print(
        f"  note: residual {residual}ns exceeds slack band — missing edge or compute-bound epoch; "
        "see handler_occupancy_ns (do not widen slack to force pass)",
        file=sys.stderr,
    )
if tsv_out:
    with open(tsv_out, "a") as tf:
        top_wc = path[0].get("wait_class", "") if path else ""
        tf.write(
            f"{status}\t{token}\t{goid}\t{wall_json}\t{pw}\t{residual}\t{pct_res:.1f}\t{occ}\t{path_edges}\t{rg}\t{top_wc}\n"
        )
sys.exit(0 if ok else 1)
PY
