#!/usr/bin/env bash
# CI Linux BPF gate: deps + compile BPF object + symbol check.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/usr/local/bin:/usr/local/go/bin:${PATH:-}"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  exec sudo env \
    PATH="$PATH" \
    SKIP_APT="${SKIP_APT:-0}" \
    BPFFTOOL_CACHE_DIR="${BPFFTOOL_CACHE_DIR:-}" \
    BPFFTOOL_VERSION="${BPFFTOOL_VERSION:-v7.5.0}" \
    "$0" "$@"
fi

./scripts/ci-install-deps.sh

export PATH="/usr/local/bin:/usr/local/go/bin:${PATH:-}"
export BPFTOL=/usr/local/bin/bpftool
export CRITICAST_BPF_GATE=1

echo "=== preflight ==="
./scripts/check-linux-env.sh

echo "=== make test-bpf ==="
make test-bpf

echo "=== workloads (sanity) ==="
make go workloads

echo "=== validate-bar-b (offline) ==="
./scripts/validate-bar-b.sh

echo "ci-linux-bpf: OK"
