#!/usr/bin/env bash
# casgstatus uprobe smoke (Linux). Verifies attach does not crash target.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/common.sh
source "$ROOT/scripts/lib/common.sh"
criticast_path

cd "$ROOT"
ensure_bin "$ROOT"

PID="$(ensure_httpgo "$ROOT")"
export PID
GO_BINARY="${GO_BINARY:-/proc/${PID}/exe}"
GO_VERSION="${GO_VERSION:-go1.22.0}"
DUR="${DUR:-5s}"

ensure_bpf "$ROOT"

echo "Attaching casgstatus on PID=$PID exe=$GO_BINARY dur=$DUR"
criticast_sudo ./bin/criticast go-smoke --pid "$PID" --go-binary "$GO_BINARY" \
  --go-version "$GO_VERSION" --dur "$DUR" \
  --bpf-object bpf/collector.bpf.o
echo "casgstatus smoke: OK"
