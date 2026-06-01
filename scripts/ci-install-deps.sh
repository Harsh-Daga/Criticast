#!/usr/bin/env bash
# CI-only dependency install (Ubuntu 22.04/24.04 GitHub Actions).
#
# On Noble/Azure runners, bpftool is not an apt package name; it ships in
# linux-tools-$KVER. /usr/sbin/bpftool is a wrapper that exits non-zero when
# the kernel-specific tools package is missing — always symlink the matched binary.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends \
  bpftrace clang llvm lld libelf-dev libbpf-dev \
  linux-tools-common linux-headers-generic \
  make git curl ca-certificates wrk

KVER="$(uname -r)"

# Kernel-matched tools (GitHub ubuntu-latest often needs linux-tools-*-azure).
if apt-cache show "linux-tools-${KVER}" >/dev/null 2>&1; then
  apt-get install -y --no-install-recommends "linux-tools-${KVER}"
fi
for meta in linux-tools-azure linux-tools-generic linux-cloud-tools-azure; do
  if apt-cache show "$meta" >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends "$meta" || true
  fi
done

tool=""
if [[ -x "/usr/lib/linux-tools/${KVER}/bpftool" ]]; then
  tool="/usr/lib/linux-tools/${KVER}/bpftool"
else
  tool="$(find /usr/lib/linux-tools -path "*/${KVER}/bpftool" -type f 2>/dev/null | head -1)"
fi
if [[ -z "$tool" ]]; then
  tool="$(find /usr/lib/linux-tools -name bpftool -type f 2>/dev/null | head -1)"
fi
if [[ -z "$tool" || ! -x "$tool" ]]; then
  echo "ci-install-deps: no bpftool under /usr/lib/linux-tools (kernel ${KVER})" >&2
  exit 1
fi

ln -sf "$tool" /usr/local/bin/bpftool
echo "ci-install-deps: bpftool -> $tool"

# What make bpf needs — not `bpftool version` (wrapper errors on KVER mismatch).
if [[ ! -f /sys/kernel/btf/vmlinux ]]; then
  echo "ci-install-deps: missing /sys/kernel/btf/vmlinux" >&2
  exit 1
fi
/usr/local/bin/bpftool btf dump file /sys/kernel/btf/vmlinux format c >/dev/null
echo "ci-install-deps: bpftool btf dump OK (kernel ${KVER})"
