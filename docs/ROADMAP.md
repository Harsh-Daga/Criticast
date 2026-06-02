# Roadmap

This page records **what works today**, **what we learned from early validation**, and **what to build next**. The charter ([CHARTER.md](../CHARTER.md) Part I) remains the long-range product plan; this doc is the practical engineering backlog.

## Current capabilities

| Area | Status |
|------|--------|
| BPF collector | `sched_switch` / `sched_waking`, task storage, ringbuf, per-CPU stats, targeted TGID |
| Go uprobes | `casgstatus` → `task_id`; `gopark` → `wait_class`, `sudog.elem`/`aux`, subject stack |
| Userspace | Ringbuf drain, trace v2 (`CRTC`), CLI `record` / `analyze` / `export` / `eval` / `env` |
| Symbolize | Trace stacks + ELF symtab; `/proc/maps` modules + build-id in trace v2 |
| Export | `criticast export --pprof` with symbolic frames when binary has symbols |
| Attribution | E1–E4 eval; production confidence model (C.3.6); Tier-2 gated on `aux` |
| Analyzer | Path policy, SCC, longest path, parallel-edge merge, scoped `--request` |
| Regression | Golden traces, `scripts/validate-bar-b.sh`, p0b/httpgo workloads |

Published numbers: [results/README.md](../results/README.md). P2 validation checklist: [results/p2-validation.md](../results/p2-validation.md).

## Validated learnings (follow these in future work)

### Overhead

- **Full mode** at ~12.5k req/s (httpgo, wrk c100): **~0.93%** throughput, **~1%** latency, **0** drops (prod0, 5 runs) — existential uprobe risk **retired for this event density** ([p0a-overhead.md](../results/phase0/p0a-overhead.md)).
- **min-block** is **worse** than full here (1.31% + latency tail) — not recommended until investigated.
- **sampled** last bench run **invalid** (broken server / harness fatigue) — isolated re-run required.
- Overhead tracks **event rate**, not req/s; p0b ~2× httpgo events/s — not delta-measured.
- Re-run `scripts/bench-p0a.sh` after BPF hot-path changes.

### Attribution

| Mechanism | Lineage only (E1) | Sudog elem (E2, GT) | Trace-joined E2 (live BPF) |
|-----------|-------------------|---------------------|----------------------------|
| spawn-lineage | 1.0 | 1.0 | — |
| conn-pool, mutex | 1.0 | 1.0 | — |
| chan-work-handoff | ~0.55 | **1.0** (GT replay) | **0.995–1.0** trace-joined ([p2-validation](../results/p2-validation.md)) — **mechanism gate**, not Bar B literal |

**Product rule:** Tier-2 chan only when `event.aux` (sudog.elem) is set; else `request-ambiguous`.

## Phase 2 — status (`phase2/tier2-product`)

**Mechanism gate validated** (prod0): trace-joined chan **1.000**. **Bar B literal:** first pass (token C); A/B failed ~67× path/wall (pool-window longest-path overcount) — **handler-rooted path + window clip shipped**; re-run `./scripts/bar-b-scoped-batch.sh`. Scorecard: [results/STATUS.md](../results/STATUS.md). **Not public GA** — [PHASES.md](PHASES.md).

| P2 item | Status |
|---------|--------|
| BPF `sudog.elem` → `aux` | Shipped (`runtime.gopark`) |
| BPF `wait_class` from gopark reason | Shipped (`go_waitreason.h`) |
| BPF subject stack at block | Shipped (gopark `stack_id`) |
| BPF mutex lock → `futex_uaddr` / `aux` | Shipped (best-effort lock pointer) |
| Tier-1 accept/recv anchor | **Deferred** (P3 stretch; Bar B uses GT cookie / `--request`) |
| Path candidate policy | Shipped (`pathpolicy.go`) |
| Confidence model C.3.6 | Shipped |
| Parallel-edge merge (E.2 baseline) | Shipped (`cascade.go`) |
| ELF symbolize + proc maps | Shipped (Linux; modules section in trace) |
| sudog.elem LRU | Shipped (`sudog_elem_seen`, 30s default TTL) |
| Scoped subgraph filter | Shipped (`FilterScopedSubgraph`) |
| `validate-bar-b.sh` | Shipped (offline B2/B3; live B1 when artifacts exist) |
| Golden traces | `golden_chain.jsonl`, `bar_b_scoped.jsonl` |

**Bar B (thesis):** **Demonstrated, not reliable** — see [p2-validation.md](../results/p2-validation.md). Remaining P2: statistical batch gate, labeling coverage. P3: netpoll, tail workload, Tier-1 anchor, min-block/sampled benches.

| P2 backlog (Bar B) | Status |
|--------------------|--------|
| Temporal-monotonic longest path | Shipped (`LongestPathTemporal`) |
| Scoped path from sudog/GT attributed edges | Shipped (`FilterScopedToken`) |
| Request epoch path (Bar B literal) | Shipped (`request_epoch.go`, `request_path.go`) |
| Path weight invariant enforced in analyze | Shipped |
| `bar_b_parallel_pool.jsonl` regression | Shipped |
| `bar-b-scoped-batch.sh` ≥80% / ≥50 epochs | Shipped (`bar-b-sample-handlers.py`) — **run on prod0** |
| Join: sudog label before worker recv | Shipped (re-measure `labeled/` — **P3 metric**) |
| Mutex join-survival | **Deferred P3** (1 edge in trace join today) |
| Live labeling without GT | P3 |
| netpoll + slow-request workload | P3 |

## Phase 1 — status

**Plumbing (Bar A): validated** ([results/p1-smoke.md](../results/p1-smoke.md)). **Mechanism attribution on p0b live trace:** validated with P2 ([p0b-attribution](../results/phase0/p0b-attribution.md)). **Bar B literal** and **httpgo realistic traffic:** open — [P1_COMPLETION.md](P1_COMPLETION.md).

See **[P1_COMPLETION.md](P1_COMPLETION.md)**.

## Next milestones (P3+)

- k8s DaemonSet, cgroup targeting, OTLP-Profiles default export
- `criticast top` TUI, Tokio/JVM (P4)
- sudog TTL / generation map, DWARF-first offsets CI
- Full wPerf cascade when multi-child fan-out needs redistribution
- Sharded analyze (only if traces &gt;500k events measured)

## Regression expectations

1. `go test ./...`
2. `./scripts/verify.sh` (Linux)
3. `./scripts/validate-bar-b.sh` (Linux, for P2 merges)
4. Re-run p0b `eval --mode all` if L3/BPF touched
5. Re-run `bench-p0a.sh` if BPF hot path touched

## Non-goals (unchanged)

Distributed tracing, L7 payload parsing, metrics TSDB, Go uretprobes, blocking kernel on userspace backpressure, naive waker-cookie propagation at shared resources.
