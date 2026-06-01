# Phase 1 — what was validated vs what was not

Read this before treating green CI or `demo-p1: OK` as proof of the Criticast thesis.

**Authority:** [CHARTER.md](../CHARTER.md) Part I defines Phase 1 as **Tier-0/1 + record/analyze/pprof**, not “request causal attribution proven on production traces.”

---

## Two different bars (do not merge them)

| Bar | Question it answers | Status (2026-06) |
|-----|---------------------|------------------|
| **A. Plumbing** | Can we capture sched wait-for events, store v2 trace, run analyze, export pprof without drops? | **Passed** on Linux 6.1 cloud ([results/p1-smoke.md](../results/p1-smoke.md)) |
| **B. Thesis** | For a **scoped request**, does the critical path sum ≈ that request’s wall time and decompose into **meaningful** waits (not idle/park noise, not all `WC_UNKNOWN`)? | **Not tested** by `demo-p1` or default `analyze` |

Bar A is necessary. Bar A is **not sufficient** for the product claim in CHARTER §0 (“which wait dominated **this request**”). That is Bar B.

Commodity baseline for Bar A: scheduler wake graphs (bcc `offwaketime` / `wakeuptime`, bpftrace sched hooks). Criticast’s differentiator is **async-task causal attribution** on top — lineage, Tier-2 elem, resource suppression, request scope — validated in **Phase 0** on ground truth ([results/phase0/p0b-attribution.md](../results/phase0/p0b-attribution.md)), **not** re-proven on the P1 live `httpgo` demo trace.

---

## What P1 is scoped as (charter Part I)

Per [CHARTER.md](../CHARTER.md) Part I, Phase 1 ships **Tier-0/1** product: record + analyze + pprof export. **Not** Tier-2 chan default.

| In scope for P1 | Explicitly out of P1 |
|-----------------|----------------------|
| End-to-end data path | Request-scoped proof on live wrk trace |
| Trace v2 + footer stats | sudog.elem in BPF (`aux` on live chan) |
| Wait-for graph + longest path | Real wait-class refinement (gopark/syscall) |
| E3: no confident Tier-2 chan without `aux` | Full confidence model (entropy, waiters) |
| Degradation labels / ambiguous | Idle/park exclusion from path candidates |
| Golden + CI | pprof with function names (ELF symbolize) |

**Answer:** P1 is the **plumbing + Tier-0/1 shell** milestone. The **attribution thesis** milestone is **not** cleared by `demo-p1.sh` alone.

If the project goal is “prove the idea on one slow request,” that is a **narrow product validation** (Bar B below) — still open.

---

## What plumbing validation showed (Bar A)

Host: Debian cloud, kernel `6.1.0-40-cloud-amd64`. Workload: `bin/httpgo` + `wrk`.

| Signal | Run with uprobes | Run sched-only |
|--------|------------------|----------------|
| `emitted` = `received` | 122114 | 134445 |
| `ringbuf_drops` / `chan_drops` | 0 / 0 | 0 / 0 |
| `switch_seen` / `target_prev` | 1.15M / 72k | 1.14M / 73k |
| `sched-smoke` (bpftrace) | 43k+ hits / 5s | OK |

Conclusion: **transport and scheduler capture are solid.** Sign off Bar A.

---

## What the live demo did *not* show (Bar B)

Honest reading of the same traces ([results/p1-smoke.md](../results/p1-smoke.md)):

### 1. Tier-2 / mechanism layer not exercised

- Every analyzed edge: **`WC_UNKNOWN`** at **`conf=60`**.
- P0’s scientific win (chan-work-handoff **~1.0** with sudog elem on GT replay) does **not** appear — BPF still does not populate `sudog.elem` in `aux` on live paths ([ROADMAP.md](ROADMAP.md)).
- E3 rules (mutex/pool/chan-without-elem) are implemented but **inactive** when class is unknown and `aux=0`.

### 2. `conf=60` is not a measured score

In `internal/attribution/trace_attrib.go`, **60** is the fixed fallback when `cookie==0` and class is unknown — not evidence or entropy. With default `--min-confidence 0`, the threshold **filters nothing**. CLI no longer implies a selective threshold; each edge still prints its label.

### 3. Dominant waits ≠ request waits

Top buckets include **`task_id=1 → task_id=1`** (large self-loop), **M↔idle (4251200)** , raw **tid=** runtime parking — scheduler churn without request tokens (no A/B/C like adversarial GT). **~65%** of blocked time in one unattributed self-loop is a graph artifact, not a bottleneck.

### 4. Critical path ≈ longest idle gap, not request bottleneck

- Unscoped longest-path picks the **maximum blocked-ns chain** over all tasks → often **parked M / netpoll**, not “slow request.”
- Path weight **150ms vs 6.8s** between runs that only differ by `GO_UPROBES` → headline metric is **not stable** for the same server/load; uprobe layer changes graph identity enough to swing path selection 45×.
- **Text vs JSON** on the same trace could differ across invocations before deterministic tie-break in `LongestPath` (fixed: sorted adjacency + tie keys).

### 5. Stacks and pprof

- **`stack_fail` ~15%** at record time → permanent blind spot for waker stacks, not deferred symbolize.
- pprof may be a valid gzip with **100% `unknown`** frames — useful for **durations**, not actionable without symbolize.

---

## Bar B — thesis validation (not done; concrete acceptance test)

When we claim Criticast measures **this request**, one trace must show:

1. **Scope:** `--request <cookie|tid>` for a known-slow request (or injected token), not “all requests.”
2. **Exclude:** idle/park/runq-only edges from path **candidates** (or scope so they cannot dominate).
3. **Decompose:** at least one edge with a **real** wait class (or Tier-2 mech + `aux`) — not all `WC_UNKNOWN`.
4. **Sum:** critical-path weight **≈** that request’s measured wall-clock latency (order-of-magnitude, charter E invariant).
5. **Compare:** optional Jaccard vs GT critical path on adversarial fixture (≥0.7 before marketing “the” critical path).

Until that passes on a live or GT-linked trace, status is:

> **Skeleton works on real metal; the part that is not offwaketime has not run yet.**

---

## Definition of done — split checklist

### Plumbing (P1 engineering) — done except merge

- [x] `verify.sh` / CI
- [x] `record` → v2 trace → `analyze` → `export --pprof`
- [x] Zero ringbuf drops under demo load
- [x] E3 code + tests (chan without `aux` not high-confidence)
- [ ] PR merge / optional `v0.1.0` tag

### Thesis (product) — not done on live demo

- [ ] Request-scoped critical path with weight ≈ wall clock
- [ ] Non–`WC_UNKNOWN` edges on that path (BPF refinement or Tier-1 spans)
- [ ] Dominant waits free of idle/sentinel garbage at top
- [ ] Stable path under go-uprobe on/off for same scoped request
- [ ] Actionable pprof (symbols) or explicit “duration-only” UX

---

## Code quality (proportionate claim)

**Fair for v0.1 plumbing:** layer split, BPF filter order, drop stats, lineage-safe analyze rules, tests, CI, operator scripts.

**Not fair to claim “production-grade causal profiler” yet:** no request-scoped path policy, placeholder confidence, no idle filter, partial stacks, no ELF symbolize, single-threaded analyze at scale.

See [ROADMAP.md](ROADMAP.md) for P2 work: `sudog.elem`, wait-class probes, symbolize, cgroup, path policy.

---

## Commands (repeat validation)

```bash
# Bar A only
./scripts/verify.sh && ./scripts/demo-p1.sh

# Bar B (when implemented) — example shape, not yet passing on httpgo demo
# ./bin/criticast analyze /tmp/trace.criticast --request 0x<cookie> --min-confidence 70
```

Phase 0 attribution matrix (mechanism logic, not live chan elem):

```bash
./scripts/record-p0b.sh
./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.jsonl --mode all
```
