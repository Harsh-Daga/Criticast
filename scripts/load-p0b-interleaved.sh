#!/usr/bin/env bash
# Interleaved load A/B/C for attribution regression (adversarial server).
# Prerequisite: p0b-server on :8080 (./scripts/run-p0b.sh in another terminal).
set -euo pipefail

PORT="${PORT:-8080}"
URL_BASE="${URL_BASE:-http://127.0.0.1:${PORT}/work?id=}"
DURATION="${DURATION:-60s}"
THREADS="${THREADS:-2}"
CONN="${CONN:-32}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }
need wrk

if ! curl -sf "http://127.0.0.1:${PORT}/health" >/dev/null; then
  echo "error: nothing on :${PORT}/health — run ./scripts/run-p0b.sh first (and stop httpgo)" >&2
  exit 1
fi

for id in A B C; do
  echo "wrk id=${id} -> ${URL_BASE}${id}"
  wrk -t"${THREADS}" -c"${CONN}" -d"${DURATION}" "${URL_BASE}${id}" &
done
wait
echo "interleaved load finished"
