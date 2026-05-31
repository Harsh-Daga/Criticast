#!/usr/bin/env bash
# Shared helpers for criticast scripts (source, do not execute).
criticast_root() {
  cd "$(dirname "${BASH_SOURCE[1]}")/../.." && pwd
}

criticast_path() {
  export PATH="/usr/local/go/bin:${PATH:-}"
}

criticast_sudo() {
  criticast_path
  sudo -E env "PATH=$PATH" "$@"
}

ensure_bpf() {
  local root="$1"
  if [[ ! -f "$root/bpf/collector.bpf.o" ]]; then
    (cd "$root" && make bpf)
  fi
}

ensure_bin() {
  local root="$1"
  if [[ ! -x "$root/bin/criticast" ]]; then
    (cd "$root" && criticast_path && make go workloads)
  fi
}

stop_httpgo() {
  pkill -x httpgo 2>/dev/null || pkill -f '/bin/httpgo' 2>/dev/null || true
  sleep 1
}

# resolve_target_pid returns TGID for process name (or $PID if already set).
resolve_target_pid() {
  local name="${1:-httpgo}"
  if [[ -n "${PID:-}" ]]; then
    echo "$PID"
    return 0
  fi
  pgrep -nx "$name" 2>/dev/null || pgrep -f "/bin/${name}" 2>/dev/null | tail -1 || true
}

# ensure_httpgo starts bin/httpgo on :PORT if not running; prints PID to stdout.
ensure_httpgo() {
  local root="$1"
  local port="${PORT:-8080}"
  local pid
  pid="$(resolve_target_pid httpgo)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    echo "$pid"
    return 0
  fi
  stop_httpgo
  pkill -x p0b-server 2>/dev/null || true
  sleep 1
  if [[ ! -x "$root/bin/httpgo" ]]; then
    (cd "$root" && criticast_path && make workloads)
  fi
  echo "ensure_httpgo: starting on :${port}" >&2
  # Must not inherit stdout/stderr when called inside $(...) or the subshell never closes.
  PORT="$port" "$root/bin/httpgo" >>/tmp/httpgo.log 2>&1 &
  pid=$!
  sleep 0.5
  for _ in $(seq 1 50); do
    if kill -0 "$pid" 2>/dev/null && curl -sf --max-time 2 "http://127.0.0.1:${port}/" >/dev/null 2>&1; then
      echo "ensure_httpgo: ready pid=$pid" >&2
      echo "$pid"
      return 0
    fi
    sleep 0.2
  done
  echo "error: httpgo did not become ready on :${port} (pid=$pid)" >&2
  return 1
}
