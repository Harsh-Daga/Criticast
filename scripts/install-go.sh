#!/usr/bin/env bash
# Install a clean Go toolchain to /usr/local/go (no apt). Run as root.
set -euo pipefail

GO_VER="${GO_VER:-1.22.10}"
ARCH="${ARCH:-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo $0" >&2
  exit 1
fi

echo "Installing Go ${GO_VER} (linux/${ARCH}) to /usr/local/go ..."
rm -rf /usr/local/go
curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xz
echo 'export PATH="/usr/local/go/bin:$PATH"' >/etc/profile.d/criticast-go.sh

export PATH="/usr/local/go/bin:$PATH"
echo "Go: $(go version)"
go list -f . runtime >/dev/null
echo "OK — use: export PATH=/usr/local/go/bin:\$PATH"
