#!/usr/bin/env bash
# Read-only probe: compare GT handler wall vs analyze path_weight for one epoch (hypothesis H2).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
# shellcheck source=lib/analyze_bar_b.sh
source "$ROOT/scripts/lib/analyze_bar_b.sh"

TOKEN="${1:-A}"
TRACE="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
GT="${GT_LOG:-/tmp/p0b-gt.log}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "bar-b-probe-epoch: requires Linux" >&2
  exit 1
fi
cd "$ROOT"
ensure_bin "$ROOT"

read -r GOID WALL_NS T_FROM T_TO < <(
  python3 - "$GT" "$TOKEN" <<'PY'
import json, sys
from datetime import datetime, timezone
gt, token = sys.argv[1], sys.argv[2]
open_pairs = {}
walls = []
with open(gt, errors="replace") as f:
    for line in f:
        if "CRITICAST_GT " not in line:
            continue
        rec = json.loads(line.split("CRITICAST_GT ", 1)[1])
        if rec.get("token") != token:
            continue
        goid = int(rec.get("goid", 0))
        ts = datetime.fromisoformat(rec["ts"].replace("Z", "+00:00"))
        if rec.get("site") == "handler-entry":
            open_pairs[goid] = ts
        elif rec.get("site") == "handler-exit" and goid in open_pairs:
            t0 = open_pairs.pop(goid)
            dt = int((ts - t0).total_seconds() * 1e9)
            if dt > 0:
                walls.append((goid, dt, t0, ts))
if not walls:
    sys.exit(2)
walls.sort(key=lambda x: x[1])
goid, wall, t0, t1 = walls[len(walls) // 2]
print(goid, wall,
      t0.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"),
      t1.astimezone(timezone.utc).isoformat().replace("+00:00", "Z"))
PY
)

PAD="$(bar_b_pad_ms "$WALL_NS")"
OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT
bar_b_analyze "$TRACE" "$GT" "$TOKEN" "$GOID" "$T_FROM" "$T_TO" "$PAD" "$OUT"

python3 - "$OUT" "$WALL_NS" "$TOKEN" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    j = json.load(f)
wall = int(sys.argv[2])
token = sys.argv[3]
pw = int(j.get("path_weight_ns", 0))
pe = len(j.get("critical_path") or [])
rg = int(j.get("request_goid_count", 0))
se = int(j.get("scoped_edge_count", 0))
wall_j = int(j.get("epoch_wall_ns", 0)) or wall
occ = int(j.get("handler_occupancy_ns", 0))
residual = int(j.get("residual_ns", max(0, wall_j - pw)))
ratio = pw / wall_j if wall_j else 0
pct_res = (100.0 * residual / wall_j) if wall_j else 0.0
print(f"probe token={token} path_weight={pw} wall={wall_j} ratio={ratio:.2f}")
print(f"  residual_ns={residual} ({pct_res:.0f}% of wall) handler_occupancy_ns={occ}")
print(f"  path_edges={pe} scoped_edges={se} request_goids={rg}")
if pe == 0:
    print("  WARN: empty critical_path")
if rg > 16:
    print("  WARN: request_goids > maxEpochMembership (16)")
PY
