#!/usr/bin/env bash
# Full P2 validation on Linux: build, verify, Bar B, fixture analyze.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-validate-p2: requires Linux" >&2
  exit 1
fi

cd "$ROOT"
export PATH="/usr/local/go/bin:${PATH:-$PATH}"

echo "========== 1. Build =========="
make bpf go workloads
make test-bpf

echo "========== 2. verify.sh (go + BPF gate) =========="
./scripts/verify.sh

echo "========== 3. validate-bar-b.sh (offline + live B1 if /tmp artifacts) =========="
./scripts/validate-bar-b.sh

echo "========== 4. Offline analyze fixtures =========="
./bin/criticast analyze testdata/traces/golden_chain.jsonl --top 5
./bin/criticast analyze testdata/traces/bar_b_scoped.jsonl --request 0x1234 --format json | head -50

echo ""
echo "linux-validate-p2: done"
echo "  Live p0b record: see results/p2-validation.md"
echo "  Optional overhead: ./scripts/bench-p0a.sh"
