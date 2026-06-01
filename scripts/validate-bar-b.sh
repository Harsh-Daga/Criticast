#!/usr/bin/env bash
# Bar B thesis gate: offline fixtures + optional live p0b eval. See results/p2-validation.md.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "validate-bar-b: requires Linux (BPF + uprobes)" >&2
  exit 1
fi

cd "$ROOT"
ensure_bin "$ROOT"
ensure_bpf "$ROOT"

FAIL=0
warn() { echo "validate-bar-b: WARN: $*" >&2; }
fail() { echo "validate-bar-b: FAIL: $*" >&2; FAIL=1; }

echo "=== B3 — offline determinism (golden trace) ==="
go test ./internal/analyzer -run 'TestAnalyzeDeterministic|TestAnalyzeBarBScoped' -count=1

echo "=== B3 — analyze text/json path stability ==="
TRACE="$ROOT/testdata/traces/bar_b_scoped.jsonl"
OUT1="$(mktemp)"
OUT2="$(mktemp)"
trap 'rm -f "$OUT1" "$OUT2"' EXIT
./bin/criticast analyze "$TRACE" --request 0x1234 --format json >"$OUT1"
./bin/criticast analyze "$TRACE" --request 0x1234 --format json >"$OUT2"
if ! diff -q "$OUT1" "$OUT2" >/dev/null; then
  fail "analyze json not deterministic"
fi

echo "=== B2-synthetic — scoped fixture (offline, by construction) ==="
if ! ./bin/criticast analyze "$TRACE" --request 0x1234 --format json | grep -q 'WC_CHAN'; then
  fail "scoped critical path missing WC_CHAN on bar_b_scoped fixture"
fi

PPROF="$(mktemp --suffix=.pb.gz)"
if ./bin/criticast export "$TRACE" --request 0x1234 --pprof "$PPROF" 2>/dev/null; then
  if command -v go >/dev/null; then
    if go tool pprof -top "$PPROF" 2>/dev/null | grep -q 'unknown'; then
      warn "pprof has unknown frames (fixture has no target_binary in trace)"
    fi
  fi
else
  warn "export pprof skipped"
fi

echo "=== B1 — mechanism gate: p0b eval trace-joined (when artifacts present) ==="
GT="${GT_LOG:-/tmp/p0b-gt.log}"
TR="${P0B_TRACE:-/tmp/p0b-trace.criticast}"
if [[ -f "$GT" && -f "$TR" ]]; then
  EVAL_OUT="$(mktemp)"
  ./bin/criticast eval --gt-log "$GT" --trace "$TR" --mode all | tee "$EVAL_OUT"
  if grep -q 'trace-joined' "$EVAL_OUT"; then
    CHAN="$(awk '
      /^=== trace-joined ===$/ { in_tj=1; next }
      in_tj && /^=== / { exit }
      in_tj && $1 == "chan-work-handoff" { print $2; exit }
    ' "$EVAL_OUT" || true)"
    if [[ -n "$CHAN" ]]; then
      awk -v c="$CHAN" 'BEGIN { if (c+0 < 0.90) exit 1 }' || fail "trace-joined chan precision ${CHAN} < 0.90"
    else
      fail "trace-joined section missing chan-work-handoff row"
    fi
  fi
  if grep -q '"bpf_ringbuf_drops":[^0]' "$TR" 2>/dev/null; then
    fail "ringbuf drops in trace footer"
  fi
else
  warn "skip B1 (set GT_LOG and P0B_TRACE; see results/p2-validation.md)"
fi

echo "=== B2-live — scoped request on real capture (Bar B literal) ==="
if [[ -f "$GT" && -f "$TR" ]]; then
  if ! ./scripts/bar-b-scoped-live.sh "${BAR_B_TOKEN:-A}"; then
    fail "Bar B literal: bar-b-scoped-live.sh (path≈wall on live trace)"
  fi
else
  warn "skip B2-live (need p0b trace + GT log)"
fi

echo "=== unit tests ==="
if [[ -n "${CRITICAST_BPF_GATE:-}" ]]; then
  go test ./testdata/p0a/httpgo ./testdata/p0b/server
else
  ./scripts/ci-go.sh
fi

if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "validate-bar-b: OK"
