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
stop_p0b_server

export PORT OTEL_TRACE_FILE
if [[ "${DETACH:-0}" == 1 ]]; then
  echo "Starting p0b-server on :$PORT (detach, logs -> $LOG)"
  nohup ./bin/p0b-server >>"$LOG" 2>&1 &
  sleep 1
  if ! curl -sf "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
    echo "error: p0b-server did not become healthy on :$PORT" >&2
    exit 1
  fi
  echo "p0b-server pid=$(pgrep -nx p0b-server) ok"
  exit 0
fi

echo "Starting p0b-server on :$PORT (foreground, logs -> $LOG)"
exec ./bin/p0b-server 2>&1 | tee -a "$LOG"
