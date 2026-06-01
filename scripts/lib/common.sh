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

need() {
  local cmd
  for cmd in "$@"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "need: missing command: $cmd" >&2
      exit 1
    fi
  done
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

# pid_listens_tcp returns 0 if PID has a LISTEN socket on PORT (best-effort).
pid_listens_tcp() {
  local pid="$1" port="$2"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp 2>/dev/null | grep -E ":${port} " | grep -q "pid=${pid}," && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk '{print $2}' | grep -qx "$pid" && return 0
  fi
  return 1
}

# pid_exe_matches returns 0 if /proc/pid/exe resolves to expected (readlink -f).
pid_exe_matches() {
  local pid="$1" expected="$2"
  local exe
  exe="$(readlink -f "/proc/${pid}/exe" 2>/dev/null)" || return 1
  [[ "$exe" == "$expected" ]]
}

# ensure_httpgo starts bin/httpgo on :PORT if not running; prints PID to stdout.
ensure_httpgo() {
  local root="$1"
  local port="${PORT:-8080}"
  local expected_exe pid url
  expected_exe="$(readlink -f "$root/bin/httpgo")"
  url="http://127.0.0.1:${port}/"
  pid="$(resolve_target_pid httpgo)"

  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null \
    && pid_exe_matches "$pid" "$expected_exe" \
    && pid_listens_tcp "$pid" "$port" \
    && curl -sf --max-time 2 "$url" >/dev/null 2>&1; then
    echo "ensure_httpgo: reusing pid=$pid exe=$expected_exe :${port}" >&2
    echo "$pid"
    return 0
  fi

  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    echo "ensure_httpgo: stale httpgo pid=$pid (wrong exe, port, or not serving) — restarting" >&2
  fi

  stop_httpgo
  pkill -x p0b-server 2>/dev/null || true
  sleep 1
  if [[ ! -x "$root/bin/httpgo" ]]; then
    (cd "$root" && criticast_path && make workloads)
  fi
  echo "ensure_httpgo: starting on :${port}" >&2
  PORT="$port" "$root/bin/httpgo" >>/tmp/httpgo.log 2>&1 &
  pid=$!
  sleep 0.5
  for _ in $(seq 1 50); do
    if kill -0 "$pid" 2>/dev/null \
      && pid_listens_tcp "$pid" "$port" \
      && curl -sf --max-time 2 "$url" >/dev/null 2>&1; then
      echo "ensure_httpgo: ready pid=$pid" >&2
      echo "$pid"
      return 0
    fi
    sleep 0.2
  done
  echo "error: httpgo did not become ready on :${port} (pid=$pid)" >&2
  tail -20 /tmp/httpgo.log 2>/dev/null >&2 || true
  return 1
}
