#!/usr/bin/env bash
# Debian/Ubuntu dependencies + Go toolchain for criticast.
#
# If apt fails (broken third-party repos), tools may already be installed:
#   SKIP_APT=1 ./scripts/debian-setup.sh
# Or install Go only:
#   ./scripts/install-go.sh
set -euo pipefail

GO_VER="${GO_VER:-1.22.10}"
ARCH=$(dpkg --print-architecture 2>/dev/null || uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

echo "criticast debian-setup (Go ${GO_VER}, arch ${ARCH})"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Re-run with sudo for apt and /usr/local/go install." >&2
  exit 1
fi

install_go() {
  echo "Installing Go ${GO_VER} to /usr/local/go ..."
  rm -rf /usr/local/go
  curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xz
  echo 'export PATH="/usr/local/go/bin:$PATH"' >/etc/profile.d/criticast-go.sh
  export PATH="/usr/local/go/bin:$PATH"
  echo "Go: $(go version)"
  go list -f . runtime >/dev/null
}

need_cmd() { command -v "$1" >/dev/null 2>&1; }

install_apt_deps() {
  local pkgs=(
    bpftrace clang llvm lld libelf-dev libbpf-dev
    linux-tools-common linux-tools-generic
    make git curl ca-certificates wrk
  )
  echo "Running apt-get update ..."
  if ! apt-get update; then
    echo "WARN: apt-get update failed (broken repo? e.g. lacework 'forky')." >&2
    echo "      Fix /etc/apt/sources.list* or use: SKIP_APT=1 $0" >&2
    return 1
  fi
  apt-get install -y --no-install-recommends "${pkgs[@]}"
  local kver tool
  kver="$(uname -r)"
  if apt-cache show "linux-tools-${kver}" >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends "linux-tools-${kver}" || true
  fi
  if [[ -x "/usr/lib/linux-tools/${kver}/bpftool" ]]; then
    tool="/usr/lib/linux-tools/${kver}/bpftool"
  else
    tool=$(find /usr/lib/linux-tools -name bpftool -type f 2>/dev/null | head -1)
  fi
  if [[ -n "$tool" ]]; then
    ln -sf "$tool" /usr/local/bin/bpftool
  fi
}

if [[ "${SKIP_APT:-0}" == "1" ]]; then
  echo "SKIP_APT=1 — skipping apt (install Go only)."
  for c in bpftrace clang llvm-strip bpftool go wrk curl; do
    if need_cmd "$c"; then
      printf '  have %s\n' "$c"
    else
      printf '  missing %s (install manually or fix apt)\n' "$c"
    fi
  done
else
  if ! install_apt_deps; then
    echo ""
    echo "Continuing with Go install only (you passed check-linux-env earlier? deps may already exist)."
  fi
fi

install_go

echo ""
echo "Done. Run:"
echo '  export PATH="/usr/local/go/bin:$PATH"'
echo "  cd $(pwd) && ./scripts/check-linux-env.sh && make test bpf go workloads"
