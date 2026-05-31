#!/usr/bin/env bash
# Linux / container preflight for criticast (BTF, tools).
set -euo pipefail

fail=0
check() {
  if eval "$2" >/dev/null 2>&1; then
    printf '  OK  %s\n' "$1"
  else
    printf '  FAIL %s\n' "$1"
    fail=1
  fi
}

echo "criticast environment check"
echo "kernel: $(uname -r)"
echo ""

check "BTF vmlinux" "test -f /sys/kernel/btf/vmlinux"
check "bpftrace" "command -v bpftrace"
check "clang" "command -v clang"
check "llvm-strip" "command -v llvm-strip || command -v llvm-strip-18"
check "bpftool" "command -v bpftool"
if command -v go >/dev/null 2>&1; then
  go_ver=$(go version | awk '{print $3}')
  case "$go_ver" in
    go1.22.*|go1.23.*|go1.24.*)
      printf '  OK  go (%s)\n' "$go_ver"
      ;;
    *)
      printf '  FAIL go (%s) — need Go 1.22+ from https://go.dev/dl/\n' "$go_ver"
      fail=1
      go_ver=""
      ;;
  esac
  if [[ -n "${go_ver:-}" ]]; then
    if ! go list -f . runtime >/dev/null 2>&1; then
      printf '  FAIL go stdlib (mixed GOROOT? run: ./scripts/debian-setup.sh)\n'
      fail=1
    fi
  fi
else
  printf '  FAIL go\n'
  fail=1
fi
check "wrk" "command -v wrk"
check "curl" "command -v curl"

if [[ -r /sys/kernel/btf/vmlinux ]]; then
  echo ""
  echo "BTF size: $(stat -c%s /sys/kernel/btf/vmlinux 2>/dev/null || stat -f%z /sys/kernel/btf/vmlinux)"
fi

if [[ $fail -ne 0 ]]; then
  echo ""
  echo "Install missing tools or use: docker compose -f docker/compose.yml run --rm dev"
  exit 1
fi
echo ""
echo "Environment ready."
