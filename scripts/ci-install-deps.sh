#!/usr/bin/env bash
# CI dependency install for GitHub Actions (Ubuntu 22.04/24.04, Debian, Azure kernels).
#
# Azure runners often have kernel 6.17.x while apt only ships linux-tools for older kernels.
# When kernel-matched packages are missing, build bpftool from libbpf/bpftool (cached via
# BPFFTOOL_CACHE_DIR when set by Actions).
#
# On hosts with broken apt repos (e.g. Lacework 404), pre-install tools via debian-setup.sh
# then: SKIP_APT=1 sudo ./scripts/ci-linux-bpf.sh
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
BPFFTOOL_VERSION="${BPFFTOOL_VERSION:-v7.5.0}"
BPFFTOOL_CACHE_DIR="${BPFFTOOL_CACHE_DIR:-}"

need_cmd() { command -v "$1" >/dev/null 2>&1; }

deps_present() {
  need_cmd bpftrace && need_cmd clang && need_cmd make && need_cmd git && need_cmd curl && need_cmd wrk &&
    { need_cmd llvm-strip || need_cmd llvm-strip-18 || need_cmd llvm-strip-19; } &&
    { need_cmd pkg-config || true; }
}

install_apt_deps() {
  if ! apt-get update; then
    echo "ci-install-deps: apt-get update failed (broken repo mirror?)" >&2
    echo "  Fix apt sources or run: SKIP_APT=1 sudo $0" >&2
    return 1
  fi
  apt-get install -y --no-install-recommends \
    bpftrace clang llvm lld libelf-dev libbpf-dev \
    linux-tools-common linux-headers-generic \
    make git curl ca-certificates wrk \
    build-essential pkg-config libcap-dev libssl-dev
}

if [[ "${SKIP_APT:-0}" == "1" ]]; then
  echo "ci-install-deps: SKIP_APT=1 — using existing system packages"
elif deps_present; then
  echo "ci-install-deps: required tools already installed — skipping apt"
else
  install_apt_deps
fi

KVER="$(uname -r)"
echo "ci-install-deps: kernel=${KVER}"

if [[ "${SKIP_APT:-0}" != "1" ]]; then
  for pkg in "linux-tools-${KVER}" "linux-cloud-tools-${KVER}"; do
    echo "ci-install-deps: trying ${pkg} ..."
    if apt-get install -y --no-install-recommends "${pkg}" 2>/dev/null; then
      echo "ci-install-deps: installed ${pkg}"
    else
      echo "ci-install-deps: package ${pkg} not available"
    fi
  done
fi

find_bpftool() {
  local t
  for t in \
    "/usr/lib/linux-tools/${KVER}/bpftool" \
    "/usr/lib/linux-cloud-tools/${KVER}/bpftool" \
    /usr/sbin/bpftool; do
    if [[ -x "$t" ]] && "$t" btf dump file /sys/kernel/btf/vmlinux format c >/dev/null 2>&1; then
      echo "$t"
      return 0
    fi
  done
  for root in /usr/lib/linux-tools /usr/lib/linux-cloud-tools; do
    t="$(find "$root" -path "*/${KVER}/bpftool" -type f 2>/dev/null | head -1)"
    if [[ -n "$t" && -x "$t" ]]; then
      echo "$t"
      return 0
    fi
  done
  return 1
}

install_bpftool_from_source() {
  local dir cached="${BPFFTOOL_CACHE_DIR}/bpftool"
  if [[ -n "$BPFFTOOL_CACHE_DIR" && -x "$cached" ]]; then
    install -m 0755 "$cached" /usr/local/bin/bpftool
    echo "ci-install-deps: bpftool from cache $cached"
    return 0
  fi

  need_cmd git || { echo "ci-install-deps: git required to build bpftool" >&2; exit 1; }
  dir="$(mktemp -d /tmp/bpftool-src.XXXXXX)"
  echo "ci-install-deps: cloning bpftool ${BPFFTOOL_VERSION} ..."
  git clone --depth 1 --recursive --branch "${BPFFTOOL_VERSION}" \
    https://github.com/libbpf/bpftool.git "$dir"
  make -C "$dir/src" -j"$(nproc)"
  install -m 0755 "$dir/src/bpftool" /usr/local/bin/bpftool

  if [[ -n "$BPFFTOOL_CACHE_DIR" ]]; then
    mkdir -p "$BPFFTOOL_CACHE_DIR"
    install -m 0755 "$dir/src/bpftool" "$cached"
    echo "ci-install-deps: cached bpftool at $cached"
  fi
  rm -rf "$dir"
}

if tool="$(find_bpftool)"; then
  ln -sf "$tool" /usr/local/bin/bpftool
  echo "ci-install-deps: bpftool -> $tool"
else
  echo "ci-install-deps: no working distro bpftool for ${KVER}; using source/cache"
  install_bpftool_from_source
fi

if [[ ! -x /usr/local/bin/bpftool ]]; then
  echo "ci-install-deps: bpftool install failed" >&2
  exit 1
fi

if [[ ! -f /sys/kernel/btf/vmlinux ]]; then
  echo "ci-install-deps: no /sys/kernel/btf/vmlinux" >&2
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    exit 1
  fi
  echo "ci-install-deps: WARN skip BTF smoke (local non-Linux/BTF host)"
  exit 0
fi

/usr/local/bin/bpftool btf dump file /sys/kernel/btf/vmlinux format c >/dev/null
echo "ci-install-deps: bpftool btf dump OK"
