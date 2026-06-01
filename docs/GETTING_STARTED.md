# Getting started

criticast runs on **Linux with BTF**. Use a bare-metal or VM host for authoritative benchmarks; optional Docker is fine for compile-and-smoke only (wake rates differ from production metal).

## Prerequisites

```bash
# Debian/Ubuntu (once per machine)
sudo ./scripts/debian-setup.sh
# or: SKIP_APT=1 ./scripts/install-go.sh  when apt is broken

export PATH="/usr/local/go/bin:$PATH"
```

Packages: `clang`, `llvm`, `lld`, `libelf-dev`, `libbpf-dev`, `bpftool`, `bpftrace`, `wrk`, `curl`.

Check environment:

```bash
./scripts/check-linux-env.sh
# or: criticast env
```

## Build

```bash
cd /path/to/criticast
make bpf go workloads    # bpf/collector.bpf.o + bin/criticast, httpgo, p0b-server
./scripts/verify.sh      # go test, vet, BPF object symbol check
```

`make bpf` requires `/sys/kernel/btf/vmlinux`. The BPF object is a **single translation unit** (`collector.c` includes `go_probe.c`) — no `ld.lld` BPF link step.

## Scripts reference

| Script | Purpose |
|--------|---------|
| `verify.sh` | Build + unit tests + BPF compile gate (every PR) |
| `demo-p1.sh` | Phase 1 E2E: httpgo + wrk + record + analyze + pprof |
| `sched-smoke.sh` | bpftrace sched_switch count for `httpgo` |
| `ci-lint.sh` | golangci-lint (same toolchain as module) |
| `validate-bar-b.sh` | P2 gates: offline fixture + mechanism eval + optional `bar-b-scoped-live.sh` |
| `bar-b-scoped-live.sh` | Bar B literal: one token on live p0b trace vs GT handler wall time |
| `bar-b-scoped-batch.sh` | Bar B batch: tokens A/B/C pass rate |
| `linux-validate-p2.sh` | Full P2 validation script (build, test, Bar B, fixtures) |
| `check-linux-env.sh` | Tool and BTF preflight |
| `debian-setup.sh` / `install-go.sh` | Host setup |
| `spike.sh` | bpftrace wake-rate check (`make spike`) |
| `bench-p0a.sh` | Overhead benchmark: baseline vs `record` (~30–45 min) |
| `casgstatus-smoke.sh` | Short Go uprobe + record smoke |
| `run-p0b.sh` | Adversarial server + ground-truth log |
| `load-p0b-interleaved.sh` | Interleaved A/B/C load |
| `record-p0b.sh` | Record trace + GT log |
| `test-bpf-object.sh` | CI: BPF symbols |
| `criticast-spike.bt` | bpftrace program (used by `spike.sh`) |

## Record

Attach scheduler probes to a target TGID:

```bash
sudo ./bin/criticast record --pid <tgid> --dur 30s \
  --min-block 1us \
  --sample 1 \
  --bpf-object bpf/collector.bpf.o \
  --out /tmp/trace.criticast
```

Traces use **format v2** (magic `CRTC`): header, stacks section, events, footer. `analyze` also reads **v1 JSONL** traces from earlier validation runs.

Go targets — add uprobes for goroutine IDs. **Generate load while recording** (idle targets often produce empty traces):

```bash
./scripts/demo-p1.sh   # recommended: starts bin/httpgo, wrk load, checks bpf emitted > 0
```

Manual equivalent:

```bash
make bpf go workloads
# Use bin/httpgo — pgrep httpgo may match an unrelated process on shared hosts.
./bin/httpgo &; sleep 1; PID=$(pgrep -nx httpgo)
wrk -t2 -c20 -d30s http://127.0.0.1:8080/ &
sudo ./bin/criticast record --pid "$PID" --dur 30s \
  --bpf-object bpf/collector.bpf.o \
  --go-binary "$(pwd)/bin/httpgo" --go-version go1.22.0 \
  --out /tmp/trace.criticast
wait
```

Output includes userspace and BPF drop counters. Ringbuf drops must stay at zero under normal load.

## Analyze

Critical-path analysis (Tier-0/1): wait-for graph, SCC collapse, longest blocked path, dominant waits.

```bash
./bin/criticast analyze /tmp/trace.criticast
./bin/criticast analyze /tmp/trace.criticast --request 0xabc --format json
./bin/criticast analyze /tmp/trace.criticast --top 20 --min-confidence 80
./bin/criticast analyze testdata/traces/golden_chain.jsonl   # offline fixture
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--request` | (none) | Scope by cookie (`0x…`), tid, goid (`goid=N`), or token (`token=A` + `--gt-log`) |
| `--gt-log` | (none) | Ground-truth log for `token=` scope on live p0b traces |
| `--top` | `10` | Dominant waits when not scoped |
| `--format` | `text` | `text` or `json` |
| `--min-confidence` | `0` | Hide sub-threshold edges from critical path (still listed as ambiguous) |
| `--spurious-wake-us` | `10` | False-wakeup filter threshold (charter E.4) |

Channel waits without `aux`/elem appear as **ambiguous** — not high-confidence Tier-2.

## Export (pprof)

```bash
./bin/criticast export /tmp/trace.criticast --pprof /tmp/criticast.pb.gz
go tool pprof -top /tmp/criticast.pb.gz
```

`--otlp` is reserved for P3 (exits with code 2). Use `--pprof` for Grafana/Pyroscope-compatible profiles.

## Phase 1 demo script

```bash
./scripts/demo-p1.sh
```

On macOS the script prints analyzer-only commands (no BPF attach).

Environment variables:

| Variable | Default | Meaning |
|----------|---------|---------|
| `GO_UPROBES` | `1` | Set `0` for sched-only record (no `casgstatus`) |
| `GO_VERSION` | `go1.22.0` | Key in `bpf/offsets.json` for goid offset |
| `DUR` | `10s` | Record and wrk duration |

Diagnostics if record shows `emitted=0`:

```bash
./scripts/sched-smoke.sh          # bpftrace: sched activity for comm httpgo
sudo ./bin/criticast probe-stats --pid "$(pgrep -nx httpgo)" --dur 5s --bpf-object bpf/collector.bpf.o
```

## Phase 1 validation sign-off

- P1 plumbing: **[P1_COMPLETION.md](P1_COMPLETION.md)**
- Phase map + ship policy: **[PHASES.md](PHASES.md)**
- **Active implementation:** branch `phase2/tier2-product` — see [ROADMAP.md](ROADMAP.md) Phase 2 and [PHASES.md](PHASES.md)

Published smoke numbers (Linux 6.1 cloud, 122k events, 0 ringbuf drops): **[results/p1-smoke.md](../results/p1-smoke.md)**.

## Evaluate attribution

Ground-truth lines use the prefix `CRITICAST_GT` (JSON: token, site, goid, optional `extra` for channel elem id).

**Three terminals** for the full interleaved regression:

| Terminal | Command |
|----------|---------|
| A | `./scripts/run-p0b.sh` |
| C | `CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh` |
| B | `sudo -E env PATH="$PATH" ./scripts/record-p0b.sh` |
| B | `OUT=/tmp/p0b-trace.criticast DUR=30s ./scripts/record-p0b.sh` |
| B | `./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.criticast --mode all` |

Single-terminal variant: background `run-p0b.sh` and load; see [results/p2-validation.md](../results/p2-validation.md).

Modes: `e1-lineage`, `e2-sudog`, `e3-suppress`, `e4-naive`, or `all`. See [ROADMAP.md](ROADMAP.md) for how to interpret E1 vs E2 on channel handoff.

## Overhead benchmark

```bash
make spike
sudo -E env PATH="$PATH" ./scripts/casgstatus-smoke.sh

stdbuf -oL -eL sudo -E env PATH="$PATH" ./scripts/bench-p0a.sh 2>&1 | tee /tmp/p0a-bench.log
```

`bench-p0a.sh` drives `httpgo` at `http://127.0.0.1:8080/`. Update [results/phase0/p0a-overhead.md](../results/phase0/p0a-overhead.md) from `/tmp/p0a-*-*.txt` when re-running.

Charter overhead target: **&lt;5%** throughput regression at representative load; see published report for medians.

## Docker (dev smoke only)

```bash
docker compose -f docker/compose.yml build
docker compose -f docker/compose.yml run --rm dev
```

Inside the container: `make test`, `make workloads`, optional `make spike`. Do not treat container wake rates as production sign-off.

## CI parity on your machine

```bash
export PATH="/usr/local/go/bin:$PATH"
./scripts/ci-go.sh
./scripts/ci-lint.sh          # not: standalone golangci-lint binary from an old release
govulncheck ./...            # optional; upgrade Go if stdlib CVEs reported

# Linux with tools already installed (check-linux-env OK):
SKIP_APT=1 sudo env PATH="/usr/local/bin:/usr/local/go/bin:$PATH" ./scripts/ci-linux-bpf.sh
```

If `apt-get update` fails (e.g. Lacework `forky` 404 repos on Debian), either fix `/etc/apt/sources.list*` or use `SKIP_APT=1` when `bpftrace`, `clang`, `bpftool`, and `wrk` are already present.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `golangci-lint`: `undefined: profile` or `go1.22 … lower than … 1.24` | Run `./scripts/ci-lint.sh` (not bare `go install` + `golangci-lint run`). Installs the linter with `GOTOOLCHAIN=go1.24.0`. |
| `record`: all BPF stats zero / empty trace | Wrong `httpgo` on host (`pgrep httpgo` may not be `bin/httpgo`); run `./scripts/demo-p1.sh` or generate load with `wrk` during `record` |
| `go tool pprof`: unrecognized profile format | Empty trace export; fix recording first. Use `go tool pprof` from Go 1.22+ |
| `apt-get update` fails / Lacework 404 | `SKIP_APT=1 sudo ./scripts/ci-linux-bpf.sh` after `check-linux-env.sh` passes |
| `govulncheck` reports Go stdlib CVEs | Upgrade Go to **1.24.6+** (or latest 1.24.x patch); not a criticast code bug |
| `invalid program type Tracing, expected RawTracepoint` | Use `link.AttachTracing` for `tp_btf/*` (already in tree) |
| BPF link / `ld.lld` fails | Single TU build: `collector.c` includes `go_probe.c` |
| `parse error` on `--dur 5` | Use `5s` or plain seconds |
| Bench hangs after `ensure_httpgo: ready` | Ensure current tree (httpgo log not on pipe) |
| Trace join `labeled=0` | Re-record with trace header `wall_base_utc` + `ktime_base_ns` |
| Bogus `task_id` / goid | Rebuild BPF; goid sanity filter in `casgstatus` |
| `eval` hangs at end | Analyzer cycle guard in `CriticalPathKeys` |
| `p0b-server` not running for record | Start `./scripts/run-p0b.sh` first |

## What is not in the tree yet

See [ROADMAP.md](ROADMAP.md): ELF symbolization from `/proc` maps, BPF `sudog.elem` capture, OTLP-Profiles default export, live TUI, Kubernetes DaemonSet.
