# Roadmap

This page records **what works today**, **what we learned from early validation**, and **what to build next**. The charter ([CHARTER.md](../CHARTER.md) Part I) remains the long-range product plan; this doc is the practical engineering backlog.

## Current capabilities

| Area | Status |
|------|--------|
| BPF collector | `sched_switch` / `sched_waking`, task storage, ringbuf, per-CPU stats, targeted TGID |
| Go uprobes | `runtime.casgstatus` → `tid_to_task` / `task_id` on events |
| Userspace | Ringbuf drain, trace v2 (`CRTC`), CLI `record` / `analyze` / `export` / `eval` |
| Symbolize | Trace stack_id → frames (PC placeholders; ELF maps planned) |
| Export | `criticast export --pprof` (gzip profile, `critical_wait` samples) |
| Attribution | E1 lineage, E2 sudog-elem (offline GT), E3 resource suppress in analyze, E4 naive baseline |
| Analyzer | Segments, SCC, longest path, false-wakeup filter, dominant-wait aggregation |
| Regression | `httpgo` workload, adversarial server + interleaved load, scripts under `scripts/` |

Published numbers: [results/README.md](../results/README.md).

## Validated learnings (follow these in future work)

### Overhead

- At ~12k req/s (Go HTTP, wrk), **full** and **sampled** modes stayed under **~1%** median throughput loss with **zero ringbuf drops** on a 6.1 kernel.
- bpftrace spike peak **~12k wakes/s** — well below the 1M/s danger zone.
- Filter order matters: target → block vs preempt → `min_block` → sample → stack.

### Attribution

| Mechanism | Lineage only (E1) | Sudog elem (E2, GT) | Notes |
|-----------|-------------------|---------------------|--------|
| spawn-lineage | 1.0 | 1.0 | `parentGoid` / spawn sites |
| conn-pool, mutex | 1.0 | 1.0 | Waiter's own lineage; do not inherit waker cookie |
| chan-work-handoff | ~0.55 | **1.0** (GT replay) | Shared worker pool breaks per-goid cookie |
| chan (trace-joined E2) | — | **~0.78** | BPF does not emit `sudog.elem` yet (`aux` always 0) |

**Product rule (from validation):**

- Ship **Tier-0/1** (scheduler wait-for + lineage) as the default.
- Use **Tier-2** (channel/work handoff) only when `sudog.elem` (or equivalent) is present in the event stream.
- Otherwise label **`request-ambiguous`** with confidence — never a confident wrong token.

**E2 offline vs production:** GT logs carry a per-item elem id at send/recv; attribution logic is validated. End-to-end trace join still needs kernel-side elem capture.

### BPF gaps found in validation

`bpf/collector.c` reads `last_sudog_elem` / `futex_uaddr` into `event.aux` but **does not write them yet**. Trace-joined E2 therefore falls back to waker-token heuristics (~78% chan precision). Closing this gap is the top BPF attribution task.

## Phase 1 execution

Full agent/engineer brief: **[P1_IMPLEMENTATION_PROMPT.md](P1_IMPLEMENTATION_PROMPT.md)** (branch `phase1/shippable-core`).

## Next milestones

Ordered by dependency. Each should land with tests and an update to benchmark reports when behavior changes.

### 1. Kernel refinement (L2)

- [ ] Populate `last_sudog_elem` on Go channel block path (gopark / wait reason)
- [ ] Optional `futex_uaddr` for mutex waits
- [ ] TTL or generation on sudog map (pointer reuse after free)
- [ ] Subject stack at block site (not only waker stack on `EV_BLOCK_END`)
- [ ] `wait_class` refinement beyond `classify_prev`

### 2. Attribution & join (L3)

- [ ] Wire trace `aux` → E2 in production path; re-run trace-joined eval (target chan ≥90%)
- [ ] `parentGoid` from DWARF or runtime probe (reduce reliance on `offsets.json` only)
- [ ] Cookie TTL enforcement in userspace replay
- [ ] Broadcast / netpoll sites in adversarial fixture + matrix rows

### 3. Symbolization & export (L1 / L5)

- [x] `internal/symbolize` — Resolver + trace STACKS (P1)
- [ ] ELF `/proc` symbolization + build-id cache
- [x] `internal/export` — pprof (P1)
- [ ] OTLP-Profiles default export (P3)
- [x] CLI: `analyze`, `export --pprof` (P1); `top` TUI (P3)

### 4. Analyzer (L4)

- [x] SCC + longest path + dominant waits (P1)
- [ ] Full wPerf cascade redistribution (E.2)
- [x] Confidence + ambiguous buckets in analyze (P1)
- [x] Golden trace tests (P1)

### 5. Operations

- [ ] CI: kernel matrix compile, attribution regression thresholds in `go test`
- [ ] Container image rename / polish (`criticast-dev` vs legacy tag)
- [ ] Operator docs for cgroup targeting, capabilities, multi-tenant scoping

## Regression expectations

When touching BPF or attribution:

1. `./scripts/verify.sh`
2. Re-run overhead bench if hot path changed ([GETTING_STARTED.md](GETTING_STARTED.md))
3. Re-run adversarial `eval --mode all` if L3 changed
4. Update [results/phase0/](../results/phase0/) reports with date, commit, and tables

Do not regress: spawn/pool/mutex precision; overhead &lt;5% at charter load; zero ringbuf drops on benchmark runs.

## Non-goals (unchanged)

Distributed tracing, L7 payload parsing, metrics TSDB, Go uretprobes, blocking kernel on userspace backpressure, naive waker-cookie propagation at shared resources.
