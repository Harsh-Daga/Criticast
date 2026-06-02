#!/usr/bin/env bash
# Validate bpf/collector.bpf.o after make bpf (Linux only).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OBJ="$ROOT/bpf/collector.bpf.o"

if [[ ! -f "$OBJ" ]]; then
  echo "test-bpf-object: missing $OBJ (run: make bpf)" >&2
  exit 1
fi

if ! command -v llvm-objdump >/dev/null 2>&1; then
  echo "test-bpf-object: llvm-objdump not found" >&2
  exit 1
fi

dump="$(llvm-objdump -t "$OBJ" 2>/dev/null || true)"
for sym in handle_switch handle_waking up_casgstatus up_gopark; do
  if ! grep -q "$sym" <<<"$dump"; then
    echo "test-bpf-object: symbol $sym not in $OBJ" >&2
    exit 1
  fi
done

# EMachine 247 = BPF
if command -v readelf >/dev/null 2>&1; then
  if ! readelf -h "$OBJ" 2>/dev/null | grep -q "Machine:.*BPF"; then
    echo "test-bpf-object: not a BPF ELF object" >&2
    readelf -h "$OBJ" >&2 || true
    exit 1
  fi
fi

echo "test-bpf-object: OK ($OBJ has sched + go uprobe symbols)"
