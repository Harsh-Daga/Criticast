# Architecture

Full design: [CHARTER.md](../CHARTER.md) Parts A–F. This page is a navigation map.

## Layers

```text
L5  CLI / export     record | analyze | top | export (pprof, OTLP-Profiles) — export planned
L4  Analyzer         wait-for graph → longest path per request (SCC/cascade planned)
L3  Attribution      lineage, sudog elem, tiers, confidence
L2  Kernel BPF       sched_switch / sched_waking + refinement → ringbuf
L1  Loader / agent   CO-RE load, drain, symbolize (symbolize planned)
L0  Linux kernel     scheduler, BTF, task storage
```

## Data flow

1. **Kernel** — On `sched_switch`, record preempt vs block; on `sched_waking`, close block with waker tid and optional stacks.
2. **Ring buffer** — Fixed 80-byte `struct event` (no pointers); filter before `ringbuf_reserve`.
3. **Userspace** — Drain to bounded channel; optional JSONL trace with ktime/wall anchor for GT join.
4. **Attribution** — Replay ground truth; score E1–E4 per mechanism; trace-join for end-to-end check.
5. **Analyzer** — Group edges by request token; longest-weight path for critical-path keys.
6. **Export** (planned) — pprof, OTLP-Profiles.

## Invariants

| Invariant | Why |
|-----------|-----|
| `bpf_ktime_get_ns` orders events | Cross-CPU consistency |
| Filter before `ringbuf_reserve` | Hot-path overhead |
| Drop + count on overflow | Never stall probes |
| `prev_state == 0` → RUNNABLE | Preempt is not a block |
| Wait-for edge ≠ cookie copy | Shared pools/mutexes (charter §0.3) |
| GPLv2 `bpf/` vs Apache Go | License boundary |

## Repository layout

```text
bpf/                    # L2 — GPLv2
cmd/criticast/          # CLI
internal/
  loader/               # CO-RE attach, probe tiers
  agent/                # ringbuf drain
  symbolize/            # planned
  attribution/          # lineage, experiments E1–E4
  analyzer/             # graph, critical path
  export/               # planned
  trace/                # JSONL format
scripts/                # verify, benchmarks
testdata/
  p0a/httpgo/           # HTTP overhead workload
  p0b/server/           # adversarial topology + CRITICAST_GT
docs/
results/                # committed benchmark reports
```

Coding rules per layer: [AGENTS.md](../AGENTS.md).
