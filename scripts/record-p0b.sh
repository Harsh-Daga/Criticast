#!/usr/bin/env bash
# Record BPF trace from running p0b-server (start ./scripts/run-p0b.sh first).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
criticast_path

DUR="${DUR:-30s}"
OUT="${OUT:-/tmp/p0b-trace.jsonl}"
GO_VER="${GO_VER:-go1.22.0}"

cd "$ROOT"
ensure_bin "$ROOT"
ensure_bpf "$ROOT"

PID="$(pgrep -nx p0b-server 2>/dev/null || pgrep -f '/bin/p0b-server' | tail -1 || true)"
if [[ -z "${PID}" ]]; then
  echo "error: p0b-server is not running — start it in another terminal:" >&2
  echo "  ./scripts/run-p0b.sh" >&2
  exit 1
fi

if ! curl -sf "http://127.0.0.1:${PORT:-8080}/health" >/dev/null 2>&1; then
  echo "warning: :${PORT:-8080}/health did not respond (continuing with PID=${PID})" >&2
fi

echo "recording PID=${PID} dur=${DUR} -> ${OUT}"
criticast_sudo "$ROOT/bin/criticast" record \
  --pid "$PID" \
  --dur "$DUR" \
  --bpf-object "$ROOT/bpf/collector.bpf.o" \
  --go-binary "/proc/${PID}/exe" \
  --go-version "$GO_VER" \
  --out "$OUT"
