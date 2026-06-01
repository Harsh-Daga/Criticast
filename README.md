# criticast

**Per-request critical path from Linux scheduler events — which wait dominated *this* request, with stacks and confidence.**

criticast complements OpenTelemetry, Grafana, and Pyroscope. It does not replace distributed tracing or APM. It answers: *for one slow request on one host, which block (lock, channel, I/O, run queue) carried the time?*

## Features

- **Wait-for graph** from `sched_switch` / `sched_waking` (BTF tracepoints, CO-RE BPF)
- **Lineage-first attribution** — request identity via spawn tree and anchors, not blind waker-cookie copy
- **Tier-0/1 default** — scheduler wait-for + optional cookie; **Tier-2 chan** only when `aux`/elem is present (never confident without it)
- **Analyze + pprof export** — critical-path text/JSON and `go tool pprof` profiles
- **Go runtime** — `runtime.casgstatus` uprobe for goroutine IDs (`offsets.json`, no uretprobes)
- **Offline evaluation** — ground-truth adversarial workload + `criticast eval` precision matrix

## Requirements

| Requirement | Notes |
|-------------|--------|
| Linux **5.8+** | BTF at `/sys/kernel/btf/vmlinux`, ringbuf, task storage |
| **Capabilities** | `CAP_BPF` + `CAP_PERFMON` (not full root in docs/examples) |
| **Toolchain** | `clang`, `llvm`, `bpftool`, `go` 1.22+ (export uses `toolchain go1.24` when needed) |
| **Host** | Production eBPF requires Linux; macOS can build Go and run analyze/export on traces |

## 5-minute demo (Phase 1)

```bash
git clone https://github.com/your-org/criticast.git && cd criticast
export PATH="/usr/local/go/bin:$PATH"

make bpf go workloads
./scripts/verify.sh

# Linux: full record → analyze → pprof
./scripts/demo-p1.sh

# Or step by step:
./bin/httpgo &
sudo ./bin/criticast record --pid $(pgrep -nx httpgo) --dur 10s \
  --bpf-object bpf/collector.bpf.o \
  --go-binary "/proc/$(pgrep -nx httpgo)/exe" --go-version go1.22.0 \
  --out /tmp/trace.criticast

./bin/criticast analyze /tmp/trace.criticast --top 10
./bin/criticast export /tmp/trace.criticast --pprof /tmp/criticast.pb.gz
go tool pprof -top /tmp/criticast.pb.gz
```

**macOS / no BPF:** analyze a fixture trace:

```bash
make go
./bin/criticast analyze testdata/traces/golden_chain.jsonl
./bin/criticast export testdata/traces/golden_chain.jsonl --pprof /tmp/demo.pb.gz
```

Attribution regression (validation baseline):

```bash
./scripts/run-p0b.sh
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
criticast record --pid <tgid> --dur 30s [--min-block 1us|50us] [--sample N] [--out trace.criticast]
criticast analyze <trace> [--request 0x…|tid] [--top N] [--format text|json]
criticast export <trace> --pprof out.pb.gz [--request 0x…|tid]
criticast eval --gt-log <log> [--trace trace] [--mode e1-lineage|all]
criticast go-smoke --pid <tgid> [--go-binary /proc/PID/exe] [--go-version go1.22.0]
```

## Repository layout

```text
bpf/              # GPLv2 kernel collector + go_probe.c
cmd/criticast/    # CLI
internal/         # loader, agent, attribution, analyzer, trace, symbolize, export
scripts/          # verify, demo-p1, benchmarks, workloads
testdata/         # httpgo, adversarial server, golden traces
docs/             # operator documentation
results/          # benchmark reports (committed)
```

## License

- **Userspace (Go, docs, scripts):** Apache-2.0
- **BPF objects (`bpf/`):** GPL-2.0-only
