# criticast

**Per-request critical path from Linux scheduler events — which wait dominated *this* request, with stacks and confidence.**

criticast complements OpenTelemetry, Grafana, and Pyroscope. It does not replace distributed tracing or APM. It answers: *for one slow request on one host, which block (lock, channel, I/O, run queue) carried the time?*

## Features

- **Wait-for graph** from `sched_switch` / `sched_waking` (BTF tracepoints, CO-RE BPF)
- **Lineage-first attribution** — request identity via spawn tree; no blind waker-cookie copy at shared resources
- **Go Tier-2** — `runtime.gopark` → `wait_class` + `sudog.elem` in `event.aux` when present
- **Analyze + pprof** — scoped `--request`, path policy, confidence (C.3.6), ELF symbolize (Linux)
- **Ground-truth eval** — adversarial p0b workload + `criticast eval` precision matrix

## Requirements

| Requirement | Notes |
|-------------|--------|
| Linux **5.8+** | BTF at `/sys/kernel/btf/vmlinux`, ringbuf, task storage |
| **Capabilities** | `CAP_BPF` + `CAP_PERFMON` |
| **Toolchain** | `clang`, `llvm`, `bpftool`, Go **1.22+** (CI uses **1.24.x**) |
| **macOS** | Build Go, run `analyze`/`export` on committed fixtures; BPF requires Linux |

## Quick start

```bash
git clone https://github.com/Harsh-Daga/Criticast.git && cd Criticast
export PATH="/usr/local/go/bin:$PATH"

make bpf go workloads
./scripts/verify.sh

# Linux: record → analyze → pprof
./scripts/demo-p1.sh

# Linux: attribution gate (p0b)
./scripts/run-p0b.sh >>/tmp/p0b-gt.log 2>&1 &
CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh &
sleep 2 && OUT=/tmp/p0b-trace.criticast DUR=30s ./scripts/record-p0b.sh
./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.criticast --mode all
```

**macOS / offline:**

```bash
make go
./bin/criticast analyze testdata/traces/golden_chain.jsonl
./scripts/validate-bar-b.sh   # Linux only for full gate
```

Full guide: **[docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)** · Validation: **[results/p2-validation.md](results/p2-validation.md)**

## Status (2026-06)

| Milestone | Status |
|-----------|--------|
| P0 benchmarks | [results/phase0/](results/phase0/) |
| P1 plumbing (Bar A) | Done — [results/p1-smoke.md](results/p1-smoke.md) |
| P2 mechanisms (p0b live trace) | **Validated** — trace-joined chan ≥0.99 |
| Bar B literal (scoped path≈wall on live capture) | **Open** — `./scripts/bar-b-scoped-live.sh` |
| Public GA | **Not yet** — [docs/PHASES.md](docs/PHASES.md) |

`demo-p1` = Bar A on `httpgo`. **Mechanism gate** = p0b `eval` on live BPF. **Bar B** = scoped `analyze` explaining one request’s latency — not the same check.

## Limitations

| Topic | Current behavior |
|-------|------------------|
| Deployment | `--pid` / local scripts; k8s DaemonSet is P3 |
| Scale | Single-process analyze; sharding not implemented |
| Stacks | ~15% `stack_fail` possible under load; symbolize needs binary/maps in trace |
| Generic httpgo | Unscoped analyze often `WC_UNKNOWN`-heavy — use `--request` + GT workload for proof |
| Overhead | Re-run `bench-p0a.sh` after BPF changes — [results/phase0/p0a-overhead.md](results/phase0/p0a-overhead.md) |

## Documentation

| Document | Description |
|----------|-------------|
| [CHARTER.md](CHARTER.md) | Design authority |
| [docs/PHASES.md](docs/PHASES.md) | Phase map and ship policy |
| [docs/P1_COMPLETION.md](docs/P1_COMPLETION.md) | Bar A vs Bar B |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Capabilities and backlog |
| [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) | Build, scripts, benchmarks |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | PR workflow and licenses |
| [AGENTS.md](AGENTS.md) | Contributor and agent rules |
| [results/](results/README.md) | Published benchmark reports |

## CLI

```text
criticast env
criticast record --pid <tgid> --dur 30s [--out trace.criticast] [--go-binary path]
criticast analyze <trace> [--request 0x…|tid] [--format text|json]
criticast export <trace> --pprof out.pb.gz [--request 0x…|tid]
criticast eval --gt-log <log> [--trace trace.criticast] [--mode all]
criticast go-smoke --pid <tgid>   # uprobe attach smoke test
criticast probe-stats --pid <tgid> --dur 5s
```

## Layout

```text
bpf/              # GPLv2 collector (sched + go uprobes)
cmd/criticast/    # CLI
internal/         # loader, agent, attribution, analyzer, trace, symbolize, export
scripts/          # verify, validate-bar-b, demo, benchmarks
testdata/         # httpgo, p0b-server, golden traces
docs/             # operator docs
results/          # committed benchmark reports
```

## License

- **Userspace (Go, docs, scripts):** Apache-2.0
- **BPF (`bpf/`):** GPL-2.0-only
