# Agent guidance — `criticast`

Instructions for humans and LLMs working in this repository. **`CHARTER.md` is the source of truth** for product and system design; this file translates it into day-to-day engineering rules.

**Docs:** [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) (run/regress), [docs/ROADMAP.md](docs/ROADMAP.md) (backlog), [results/](results/README.md) (benchmarks).

## Before you write code

1. **Read `CHARTER.md`** (at least Part 0, the layer you touch, and Part H for validation methodology).
2. **Read `docs/ROADMAP.md`** for what is implemented vs planned — do not build export/k8s/symbolize unless the task targets those milestones.
3. **Prefer the smallest change** that satisfies the task. Do not refactor unrelated code or add speculative abstractions.

## North star

**Single-host, per-request critical path from scheduler events** — which wait (lock, channel, I/O, run queue) dominated latency for *this* request, with stacks and confidence.

We are **not** building: a distributed tracer, an APM replacement, a protocol parser, or a general-purpose dashboard. We **complement** OTel/Grafana/Pyroscope via pprof and OTLP-Profiles export.

## Non-negotiable design rules (from charter)

### Wait-for vs request identity

| Concept | Rule |
|--------|------|
| **Wait-for edge** | `wakee → waker` at `sched_waking`, weighted by blocked time. Almost always correct; preserve it. |
| **Request identity (cookie)** | **Lineage-first**: Tier-1 socket anchor + `parentGoid` spawn tree. **Never** blindly copy the waker's cookie on every wakeup. |
| **Mutex / sema / conn pool** | **Do not propagate** request identity. Record contention; keep the waiter's own lineage cookie. |
| **Ambiguity** | Label `request-ambiguous` with confidence — **never** a confident wrong answer. |

### Kernel hot path (L2)

- Filter **before** `ringbuf_reserve`: `targeted()` → `min_block_ns` → sampling → then stacks.
- **`prev_state == 0`** after preempt = RUNNABLE (run-queue latency), **not** a block. Getting this wrong corrupts all metrics.
- Prefer **`tp_btf/sched_switch`** and **`tp_btf/sched_waking`**; fall back only when attach tier requires it.
- **Never stall the probe**: ring overflow → drop + count; never block the kernel from userspace backpressure.
- **Single clock domain in-kernel**: `bpf_ktime_get_ns` for ordering; userspace wall clock for display only.
- Stack walks are expensive: gate on `min_block` + sampling; capture subject stack at block site (syscall/gopark), waker stack at `sched_waking`.
- **Go: entry uprobes only** — no uretprobes on Go binaries (movable stacks).

### Tech stack (do not swap without charter amendment)

| Layer | Choice |
|-------|--------|
| Kernel BPF | C + libbpf + CO-RE + BTF |
| Userspace | Go + cilium/ebpf + bpf2go |
| Min kernel | 5.8+ (ringbuf, task storage), BTF required |
| Licenses | Apache-2.0 userspace · GPLv2 BPF objects |

Do **not** implement kernel BPF in Rust/Aya for production paths (charter: BTF relocation maturity).

### Validation thresholds (charter §B.5, Part H)

| Check | Question | On failure |
|-------|----------|------------|
| Overhead | Within budget on real load? | Sampled-only or stop |
| Attribution | Per-mechanism precision? | Degrade Tier-2; label ambiguous |

See [results/](results/README.md) and [docs/ROADMAP.md](docs/ROADMAP.md).

## Repository layout (target)

Keep layers separable:

```
bpf/           # GPLv2: maps, probes, struct event (L2 contract)
cmd/           # criticast CLI entrypoints
internal/
  loader/      # CO-RE attach, probe tiers (L1)
  agent/       # ringbuf drain, targets, config
  symbolize/   # stack_id → frames, build-id cache
  attribution/ # lineage, cookies, confidence (L3)
  analyzer/    # segments, wait-for graph, critical path (L4)
  export/      # pprof, OTLP-Profiles, .criticast trace file
testdata/      # httpgo + adversarial server, golden traces
docs/          # operator docs (not duplicate of charter)
```

Match existing layout if the tree already differs; do not fight established paths without reason.

## Code quality standards

### General

- **Production-grade**: explicit error handling, no silent drops without metrics, no panics on user input paths.
- **Readable**: names match charter terms (`wait_class`, `cookie`, `task_id`, `EV_BLOCK_END`).
- **Minimal scope**: no drive-by refactors, no "while we're here" features.
- **Comments**: only for non-obvious invariants (e.g. preempt bit-test, verifier limits) — not narrating obvious code.
- **No secrets** in repo: no API keys, no `.env` commits.

### Go (userspace)

- Use `errors` wrapping with context; return errors up, handle at `main`/CLI boundary.
- Context propagation for shutdown (`signal.NotifyContext`).
- Bounded channels for ringbuf → workers; document capacity vs drop policy.
- Table-driven tests for analyzer and attribution; golden files for trace → critical path.
- `go test ./...` must pass; run `go vet` / staticcheck if configured.
- Keep `internal/` packages acyclic: loader → agent → analyzer; export at the edge.

### C / BPF (kernel)

- CO-RE only (`BPF_CORE_READ`, vmlinux.h); no hardcoded struct layouts for `task_struct`.
- Respect verifier: bounded loops, limited stack, no large stack arrays.
- Fixed **`struct event`** layout — any change bumps version in trace header and bpf2go types together.
- Map updates in probes: prefer task storage field writes over ringbuf for refinement probes.
- Every drop path increments a **stat** (per-CPU array), not only silent return.

### Wire formats & API stability

- L2→L4 contract is **`struct event`** (see CHARTER §B.2, Appendix P). Pointer-free, fixed size.
- Breaking event layout requires version bump + migration note in commit/PR.
- CLI flags should mirror charter terms: `--min-block`, `--sample`, `--runtime go|none|tokio`.

## Testing expectations

| Area | Expectation |
|------|-------------|
| Analyzer | Unit tests: segments, cascade, SCC, longest-path; invariant `path_weight ≈ wall_clock` |
| Attribution | Adversarial fixture: per-mechanism precision; do not regress published baselines |
| BPF | CI kernel matrix (when infra exists); at minimum compile bpf object in CI |
| Go offsets | `offsets.json` + DWARF tests per Go version — never hardcode `0x98`-style goid offsets in source |

Add tests when fixing bugs; do not add tests that only assert mocks or trivial getters.

## Security & privacy

- Read-only observation; no target memory writes, no payload inspection.
- Document required capabilities: `CAP_BPF`, `CAP_PERFMON` — not `privileged` in docs/examples.
- Target scoping by tgid/cgroup; do not cross tenant boundaries in cookies or export.

## PR / change checklist (LLM self-check)

- [ ] Aligns with `CHARTER.md` and `docs/ROADMAP.md`?
- [ ] Wait-for edges preserved; cookies not naively forward-propagated?
- [ ] Hot path: filter before reserve; drops counted?
- [ ] Licenses respected (GPLv2 bpf/ vs Apache internal/)?
- [ ] Tests or spike script for non-trivial logic?
- [ ] No scope creep into distributed tracing / full APM?

## When uncertain

1. Re-read the relevant charter part (A–E for probes/analyzer, C for attribution, H for PoC).
2. Choose **degradation** (Tier-0/1, ambiguous label) over a heuristic that asserts certainty.
3. Ask the user rather than inventing kernel/runtime facts — cite `CHARTER.md` or kernel source if you do.

## Related Cursor rules

File-specific rules live in `.cursor/rules/`:

- `criticast-core.mdc` — always applies
- `bpf-collector.mdc` — BPF/C probes
- `go-userspace.mdc` — Go agent and CLI
- `attribution-analyzer.mdc` — L3/L4 logic

## Documentation map

See [docs/README.md](docs/README.md) for the full index.
