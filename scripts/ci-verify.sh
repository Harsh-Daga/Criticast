#!/usr/bin/env bash
# Local/legacy full gate: Go + Linux BPF (same as CI jobs combined).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="/usr/local/bin:/usr/local/go/bin:${PATH:-}"

./scripts/ci-go.sh

if [[ "$(uname -s)" == "Linux" ]]; then
  if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    exec sudo env PATH="$PATH" BPFFTOOL_VERSION="${BPFFTOOL_VERSION:-v7.5.0}" \
      "$ROOT/scripts/ci-linux-bpf.sh"
  fi
  ./scripts/ci-linux-bpf.sh
else
  echo "ci-verify: skip BPF (not Linux)"
fi

echo "ci-verify: OK"
