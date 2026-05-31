#!/usr/bin/env bash
# Start adversarial server on :8080 (stops httpgo if running).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
criticast_path

PORT="${PORT:-8080}"
LOG="${LOG:-/tmp/p0b-gt.log}"
OTEL_TRACE_FILE="${OTEL_TRACE_FILE:-/dev/null}"

cd "$ROOT"
ensure_bin "$ROOT"
stop_httpgo

echo "Starting p0b-server on :$PORT (logs -> $LOG)"
export PORT OTEL_TRACE_FILE
exec ./bin/p0b-server 2>&1 | tee "$LOG"
