#!/usr/bin/env bash
# Dev gate: compile + unit tests. Does NOT run overhead or attribution benchmarks.
#
# Checks:
#   1. Linux tools present (BTF, bpftrace, clang, bpftool, …)
#   2. go test + go vet (all packages + workloads)
#   3. bin/criticast, httpgo, p0b-server build
#   4. bpf/collector.bpf.o links (sched + go uprobe in one object)
#
# Benchmarks (run separately on Linux under load — docs/GETTING_STARTED.md):
#   - scripts/spike.sh, bench-p0a.sh → results/phase0/p0a-overhead.md
#   - run-p0b.sh + record-p0b.sh + eval → results/phase0/p0b-attribution.md
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="/usr/local/go/bin:${PATH:-}"

exec "$(dirname "$0")/ci-verify.sh"
