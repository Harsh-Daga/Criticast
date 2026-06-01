# Phase 1 — what was validated vs what was not

Read this before treating green CI or `demo-p1: OK` as proof of the Criticast thesis.

**Authority:** [CHARTER.md](../CHARTER.md) Part I defines Phase 1 as **Tier-0/1 + record/analyze/pprof**, not “request causal attribution proven on production traces.”

---

## Two different bars (do not merge them)

| Bar | Question it answers | Status (2026-06) |
|-----|---------------------|------------------|
| **A. Plumbing** | Can we capture sched wait-for events, store v2 trace, run analyze, export pprof without drops? | **Passed** on Linux 6.1 cloud ([results/p1-smoke.md](../results/p1-smoke.md)) |
| **B. Thesis** | For a **scoped request** on a **live capture**, does critical path ≈ that request’s wall time with **labeled, non-idle** dominant waits? | **Open** — mechanism eval ✓; synthetic fixture ✓; live scoped analyze **not ticked** ([p2-validation](../results/p2-validation.md)) |

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
- P0’s scientific win on **httpgo demo** still does **not** appear — unscoped analyze remains `WC_UNKNOWN`-heavy. **p0b live trace** now populates `aux` (P2); see [p2-validation](../results/p2-validation.md).
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

## Bar B — thesis validation

### Mechanism gate on p0b live trace — **done with P2** (2026-06-02)

On `prod0-telephony-voipmonitor-primary`, interleaved A/B/C + `record-p0b.sh`:

- **trace-joined** `chan-work-handoff` **0.995–1.000** (≥0.90 mechanism gate)
- `labeled/block_ends` ≈ **15%** at join — most blocking not GT-labeled for scoped path
- `ringbuf_drops=0`
- Details: [results/p2-validation.md](../results/p2-validation.md)

This validates **sudog/channel handoff on real BPF data**. It does **not** tick the Bar B sentence (scoped `analyze`, path≈wall on that request).

### Bar B literal on live p0b — **open**

Missing artifact: `./scripts/bar-b-scoped-live.sh A` on `/tmp/p0b-trace.criticast` (scope by handler **goid** from GT; BPF cookies are usually 0).

Also open: **tail workload** (injected slow path) so “critical path explains the slow request” is testable; uniform ~14.7ms traffic only tests arithmetic.

### Generic `httpgo` demo — **not done**

`demo-p1.sh` still shows process-wide `WC_UNKNOWN` dominance and ambiguous path weight. Until request-scoped analyze on a non-GT workload passes items below, do not claim Bar B for arbitrary services.

When we claim Criticast measures **this request** on an arbitrary app, one trace must show:

1. **Scope:** `--request <cookie|tid>` for a known-slow request, not “all requests.”
2. **Exclude:** idle/park/runq-only edges from path **candidates**.
3. **Decompose:** real wait class or Tier-2 mech + `aux` on the path.
4. **Sum:** path weight **≈** request wall clock (charter E invariant).
5. **Compare:** optional Jaccard vs GT (≥0.7) where GT exists.

---

## Definition of done — split checklist

### Plumbing (P1 engineering) — done except merge

- [x] `verify.sh` / CI
- [x] `record` → v2 trace → `analyze` → `export --pprof`
- [x] Zero ringbuf drops under demo load
- [x] E3 code + tests (chan without `aux` not high-confidence)
- [ ] PR merge / optional `v0.1.0` tag

### Thesis (product) — mechanism done; Bar B literal open

- [x] Mechanism gate (p0b trace-joined chan ≥0.90) — P2
- [x] BPF `sudog.elem` → `aux` on live trace
- [x] Offline scoped fixture (`bar_b_scoped.jsonl`) — synthetic only
- [ ] **Bar B literal:** `bar-b-scoped-live.sh` path≈wall on live trace
- [ ] p0b tail / slow-request injection
- [ ] Request-scoped path on **generic** httpgo with weight ≈ wall clock
- [ ] Non–`WC_UNKNOWN` dominant waits on unscoped production-shaped trace
- [ ] Stable scoped path under go-uprobe on/off (httpgo)
- [ ] Actionable pprof on live record without manual `--go-binary` (UX)

---

## Code quality (proportionate claim)

**Fair after P2 (internal / merge-ready):** charter layer split; BPF filter-before-reserve; drop stats; lineage-first L3; path policy + cascade; ELF symbolize (Linux); Go gopark Tier-2; golden tests + `validate-bar-b`; CI gates; operator scripts.

**Not fair to claim “large-scale OSS production GA” yet:**

| Area | Status |
|------|--------|
| k8s / OTLP / TUI | P3 |
| Sharded analyze (1M+ events) | Not built |
| Post-P2 overhead proof | Re-run `bench-p0a.sh` |
| `stack_fail` ~15% | Known gap |
| Security audit / fuzz / SBOM | Not done |
| Tier-1 socket cookie anchor | P3 stretch |

See [ROADMAP.md](ROADMAP.md) P3+ and [results/p2-validation.md](../results/p2-validation.md) § Production readiness.

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
./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.criticast --mode all
```
