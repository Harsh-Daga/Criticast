#!/usr/bin/env bash
# Bar B (literal): scoped analyze on a *live* p0b trace vs GT handler wall time.
# Compares one representative handler entry→exit window (not all token traffic in the capture).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

TOKEN="${1:-A}"
TRACE="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
GT="${GT_LOG:-/tmp/p0b-gt.log}"
SLACK_PCT="${BAR_B_SLACK_PCT:-30}"
MAX_PATH_RATIO="${BAR_B_MAX_PATH_RATIO:-3.0}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "bar-b-scoped-live: requires Linux" >&2
  exit 1
fi
if [[ ! -f "$TRACE" || ! -f "$GT" ]]; then
  echo "bar-b-scoped-live: need TRACE and GT_LOG (record p0b first)" >&2
  exit 1
fi

cd "$ROOT"
ensure_bin "$ROOT"

OUT_JSON="$(mktemp)"
trap 'rm -f "$OUT_JSON"' EXIT

if ! read -r GOID WALL_NS SAMPLES T_FROM T_TO < <(python3 - "$GT" "$TOKEN" <<'PY'
import json, sys
from datetime import datetime, timezone

gt_path, token = sys.argv[1], sys.argv[2]
open_pairs = {}
walls = []
with open(gt_path, errors="replace") as f:
    for line in f:
        if "CRITICAST_GT " not in line:
            continue
        payload = line.split("CRITICAST_GT ", 1)[1].strip()
        try:
            rec = json.loads(payload)
        except json.JSONDecodeError:
            continue
        if rec.get("token") != token:
            continue
        site = rec.get("site")
        goid = int(rec.get("goid", 0))
        ts = datetime.fromisoformat(rec["ts"].replace("Z", "+00:00"))
        if site == "handler-entry":
            open_pairs[goid] = ts
        elif site == "handler-exit" and goid in open_pairs:
            t0 = open_pairs.pop(goid)
            dt = (ts - t0).total_seconds() * 1e9
            if dt > 0:
                walls.append((goid, int(dt), t0, ts))
if not walls:
    sys.exit(2)
walls.sort(key=lambda x: x[1])
goid, wall, t0, t1 = walls[len(walls) // 2]
print(
    goid,
    wall,
    len(walls),
    t0.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"),
    t1.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"),
)
PY
); then
  echo "bar-b-scoped-live: no handler-entry/exit pairs for token=${TOKEN} in ${GT}" >&2
  exit 1
fi

if [[ "$GOID" == "0" || "$WALL_NS" == "0" ]]; then
  echo "bar-b-scoped-live: invalid GT wall stats" >&2
  exit 1
fi

echo "bar-b-scoped-live: token=${TOKEN} GT_wall_median=${WALL_NS}ns (n=${SAMPLES}) handler_goid=${GOID}"

# Tighter pad than early runs (half-padding on ~14ms requests leaked neighbor edges).
PAD_MS=$(( WALL_NS / 4000000 ))
if [[ "$PAD_MS" -lt 4 ]]; then
  PAD_MS=4
fi
if [[ "$PAD_MS" -gt 25 ]]; then
  PAD_MS=25
fi

echo "bar-b-scoped-live: scope_window=${T_FROM} .. ${T_TO} pad=${PAD_MS}ms"

./bin/criticast analyze "$TRACE" \
  --request "token=${TOKEN}" \
  --gt-log "$GT" \
  --scope-from "$T_FROM" \
  --scope-to "$T_TO" \
  --scope-pad "${PAD_MS}ms" \
  --scope-handler-goid "$GOID" \
  --format json >"$OUT_JSON"

if ! python3 - "$OUT_JSON" "$WALL_NS" "$SLACK_PCT" "$MAX_PATH_RATIO" <<'PY'
import json, sys

out_path = sys.argv[1]
wall = int(sys.argv[2])
slack_pct = float(sys.argv[3])
max_ratio = float(sys.argv[4])

with open(out_path) as f:
    j = json.load(f)

pw = int(j.get("path_weight_ns", 0))
path = j.get("critical_path") or []
scoped_edges = int(j.get("scoped_edge_count", 0))
request_goids = int(j.get("request_goid_count", 0))
window_edges = int(j.get("window_edge_count", 0))
path_edges = len(path)

print(f"bar-b-scoped-live: path_weight_ns={pw} path_edges={path_edges} scoped_edges={scoped_edges} request_goids={request_goids} window_edges={window_edges}")
if window_edges > 0 and scoped_edges == 0:
    print("  hint: no edges for calibrated request goids — check /tmp/p0b-record.log uprobes")

slack = max(int(wall * slack_pct / 100), 1_000_000)
lo = max(wall - slack, 0)
hi = wall + slack
ratio = pw / wall if wall else 0
print(f"  wall_slack: ±{slack_pct}% ({slack}ns) band [{lo}, {hi}]")
print(f"  path/wall ratio: {ratio:.2f} (max {max_ratio})")
print(f"  path in band: {'OK' if lo <= pw <= hi else 'FAIL'}")

dominant_classes = ("WC_CHAN", "WC_CONN_POOL", "WC_MUTEX", "WC_NET", "WC_FUTEX", "WC_SEM")
has_dominant = any(e.get("wait_class") in dominant_classes for e in path)
non_idle = has_dominant
if has_dominant:
    print("  critical_path labeled dominant wait: OK")
else:
    print("  critical_path labeled dominant wait: FAIL")

# Non-idle on path is necessary but not sufficient — reject pathological overcount.
if non_idle and ratio <= max_ratio:
    print("  labeled non-idle on path (bounded): OK")
else:
    if not non_idle:
        print("  labeled non-idle on path (bounded): FAIL (no WC_* on path)")
    else:
        print(f"  labeled non-idle on path (bounded): FAIL (ratio {ratio:.1f} > {max_ratio})")
        print("    hint: pool-window leak or idle park — dump critical_path edges")

if path_edges > 0 and ratio > max_ratio:
    top = sorted(path, key=lambda e: int(e.get("blocked_ns", 0)), reverse=True)[:5]
    print("  top path edges (blocked_ns):")
    for e in top:
        print(f"    {e.get('wait_class')} blocked_ns={e.get('blocked_ns')} from={e.get('from')} to={e.get('to')}")

if pw < lo:
    print("  path≥wall−slack: WARN — path_weight << handler wall")

in_band = lo <= pw <= hi

fail = (
    (not has_dominant)
    or (not non_idle)
    or (not in_band)
    or ratio > max_ratio
)
sys.exit(1 if fail else 0)
PY
then
  echo "bar-b-scoped-live: FAIL" >&2
  exit 1
fi

echo "bar-b-scoped-live: OK"
