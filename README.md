# criticast

**Per-request critical path from Linux scheduler events — which wait dominated *this* request, with stacks and confidence.**

criticast complements OpenTelemetry, Grafana, and Pyroscope. It does not replace distributed tracing or APM. It answers: *for one slow request on one host, which block (lock, channel, I/O, run queue) carried the time?*

## Features

- **Wait-for graph** from `sched_switch` / `sched_waking` (BTF tracepoints, CO-RE BPF)
- **Lineage-first attribution** — request identity via spawn tree and anchors, not blind waker-cookie copy
- **Tiered confidence** — ambiguous when identity is unknown; never a confident wrong label
- **Go runtime** — `runtime.casgstatus` uprobe for goroutine IDs (`offsets.json`, no uretprobes)
- **Offline evaluation** — ground-truth adversarial workload + `criticast eval` precision matrix
- **Trace capture** — JSONL with monotonic/wall clock correlation for join with app logs

## Requirements

| Requirement | Notes |
|-------------|--------|
| Linux **5.8+** | BTF at `/sys/kernel/btf/vmlinux`, ringbuf, task storage |
| **Capabilities** | `CAP_BPF` + `CAP_PERFMON` (not full root in docs/examples) |
| **Toolchain** | `clang`, `llvm`, `bpftool`, `go` 1.21+ |
| **Host** | Production eBPF requires Linux; macOS can build Go only |

## Quick start

```bash
git clone https://github.com/your-org/criticast.git && cd criticast
export PATH="/usr/local/go/bin:$PATH"   # if needed

make bpf go workloads
./scripts/verify.sh
```

Record a target process (example: HTTP workload on :8080):

```bash
make spike                    # optional: bpftrace wake-rate sanity check
./bin/httpgo &                # or your own service
sudo ./bin/criticast record --pid $(pgrep -nx httpgo) --dur 30s \
  --bpf-object bpf/collector.bpf.o \
  --go-binary "/proc/$(pgrep -nx httpgo)/exe" --go-version go1.22.0 \
  --out /tmp/trace.jsonl
```

Evaluate attribution against ground truth:

```bash
./scripts/run-p0b.sh          # terminal A — adversarial server + GT log
# terminal C: CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh
sudo -E env PATH="$PATH" ./scripts/record-p0b.sh
./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.jsonl --mode all
```

See **[docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)** for setup, scripts, benchmarks, and troubleshooting.

## Documentation

| Document | Description |
|----------|-------------|
| [CHARTER.md](CHARTER.md) | Product and system design (source of truth) |
| [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) | Build, run, validate, regress |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Layers, data flow, invariants |
| [docs/ROADMAP.md](docs/ROADMAP.md) | What works today and what to build next |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | PR workflow, licenses, tests |
| [AGENTS.md](AGENTS.md) | Engineering rules for contributors and LLMs |
| [results/](results/README.md) | Published benchmark and attribution reports |

## CLI

```text
criticast env
criticast record --pid <tgid> --dur 30s [--min-block 1us|50us] [--sample N] [--out trace.jsonl]
criticast eval --gt-log <log> [--trace trace.jsonl] [--mode e1-lineage|e2-sudog|all]
criticast go-smoke --pid <tgid> [--go-binary /proc/PID/exe] [--go-version go1.22.0]
```

## Repository layout

```text
bpf/              # GPLv2 kernel collector + go_probe.c
cmd/criticast/    # CLI
internal/         # loader, agent, attribution, analyzer, trace, …
scripts/          # verify, benchmarks, workloads
testdata/         # httpgo + adversarial server for regression
docs/             # operator documentation
results/          # benchmark reports (committed)
```

## License

- **Userspace (Go, docs, scripts):** Apache-2.0
- **BPF objects (`bpf/`):** GPL-2.0-only
