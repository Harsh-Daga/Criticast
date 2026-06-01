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

Go targets — add uprobes for goroutine IDs:

```bash
sudo ./bin/criticast record --pid $(pgrep -nx httpgo) --dur 30s \
  --bpf-object bpf/collector.bpf.o \
  --go-binary "/proc/$(pgrep -nx httpgo)/exe" \
  --go-version go1.22.0 \
  --out /tmp/trace.criticast
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
| `--request` | (none) | Scope by cookie (`0x…`) or tid (decimal) |
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

## Evaluate attribution

Ground-truth lines use the prefix `CRITICAST_GT` (JSON: token, site, goid, optional `extra` for channel elem id).

**Three terminals** for the full interleaved regression:

| Terminal | Command |
|----------|---------|
| A | `./scripts/run-p0b.sh` |
| C | `CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh` |
| B | `sudo -E env PATH="$PATH" ./scripts/record-p0b.sh` |
| B | `./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.jsonl --mode all` |

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

## Troubleshooting

| Symptom | Fix |
|---------|-----|
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
