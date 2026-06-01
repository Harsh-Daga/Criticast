# Phase 2 validation report

**Branch:** `phase2/tier2-product` · **Host:** prod0 (2026-06-02)

Read [STATUS.md](STATUS.md) for the one-page scorecard. **Do not conflate Bar A, mechanism gate, and Bar B literal.**

---

## Three gates

| Gate | Question | Status |
|------|----------|--------|
| **Bar A** | Kernel → trace → analyze → export without drops? | **PASS** |
| **Mechanism** | Trace-joined causal edges typed correctly? | **PASS** (chan **1.000**) |
| **Bar B literal** | Scoped live request: path ≈ wall, labeled non-idle path? | **Open** — honest gate (no wall fill); prod0 batch + residual report + falsification pending |

---

## Environment

| Item | Value |
|------|--------|
| Kernel | `6.1.0-40-cloud-amd64` |
| Go | 1.24.0 |
| Load | p0b interleaved A/B/C, `CONN=8`, 30s record |
| Trace | `/tmp/p0b-trace.criticast` |
| GT | `/tmp/p0b-gt.log` |

---

## Bar A — plumbing

| Check | Result |
|-------|--------|
| `make bpf` / `go test` / `verify.sh` | PASS |
| Live record | ~1.5M consumed, **0** ringbuf drops |

---

## Mechanism gate

Trace join (representative prod0 capture):

| Metric | Value |
|--------|--------|
| block_ends | ~748k |
| with_goid | ~99.3% |
| labeled (TokenAt only, pre-sudog join) | **~4.5%** (34k) — investigate vs prior ~15% |
| trace-joined chan | **1.000** |
| conn-pool | 1.0 |
| spawn-lineage | 1.0 |
| mutex | 1 edge in join sample |
| netpoll / broadcast | 0 / unimplemented |

**Interpretation:** Sudog/channel handoff on live BPF is validated. Low `labeled` fraction limits how many block_ends feed a scoped path; join now credits **sudog.elem** at block time before worker GT recv ([`join.go`](../internal/attribution/join.go)) — re-run `eval` to measure uplift.

---

## B2-synthetic

| Fixture | Result |
|---------|--------|
| `bar_b_scoped.jsonl` | PASS (by construction) |
| `golden_chain.jsonl` | Analyzer regression PASS |

Proves path arithmetic and policy on hand-built traces — **not** live Bar B.

---

## B2-live — scoped request on real capture

### Commands

```bash
./scripts/bar-b-scoped-live.sh C    # single token
./scripts/bar-b-scoped-batch.sh     # A B C + pass rate
```

Uses handler entry→exit window, `--scope-handler-goid`, calibrated trace `task_id`s, ktime overlap filter.

### Prod0 results (pre path-bounding fix)

| Token | GT wall (median) | path_weight | path/wall | WC_CHAN | Band ±30% |
|-------|------------------|-------------|-----------|---------|-----------|
| **C** | ~14.5 ms | ~12.7 ms | ~0.87 | yes | **PASS** |
| **A** | ~14.5 ms | ~970 ms | ~67 | yes* | FAIL |
| **B** | ~14.5 ms | ~971 ms | ~67 | yes* | FAIL |

\*“labeled non-idle on path” passed on A/B but **misleading** — ~600 scoped edges chained (~1.6 ms each) from shared worker pool + wide scope pad, not one causal chain.

### Path fixes (2026-06-02)

| Fix | Why |
|-----|-----|
| **`LongestPathTemporal`** (`path_temporal.go`) | Old longest path summed **concurrent** waits as serial (~600×1.6ms). Temporal relax requires non-overlapping time order. |
| **`FilterScopedToken`** (`scope_attributed.go`) | Token scope (eval / fixtures); Bar B literal uses **request epoch** (`request_epoch.go`). |
| **`PathWeightInvariantOK` hard fail** (`analyze.go`) | `path_weight ≤ handler_window + slack` or analyze errors. |
| Clip + parallel max + tighter pad | Still useful; not sufficient alone. |
| `bar_b_parallel_pool.jsonl` + unit tests | Regression for 970ms class. |
| `bar-b-scoped-batch.sh` | **P2 gate:** ≥80% of ≥50 tokens (`BAR_B_MIN_*`). |

### P2 done checklist (fill on prod0)

**Precondition:** `make bpf` + re-record (`p0b-full-validate.sh`). Prior traces lack `EV_TASK_STATE`; prior pass rates may have been tautological (wall-filled `path_weight`).

| Line | Criterion | Prod0 result |
|------|-----------|--------------|
| **(a)** | `bar-b-scoped-batch.sh` ≥80% + **residual_ns** median/p90 in notes | _pending_ |
| **(b)** | `eval --mode all`: chan ≥0.90 | _re-run_ |
| **(c)** | `bar-b-falsification-run.sh`: A slows, B/C stable, A dominant edge = worker | _pending_ |
| **(d)** | `bench-p0a.sh` with `running=` stat; ringbuf_drops=0 | _pending_ (0.93% is **stale**) |
| **(e)** | Unit tests (`go test ./...`) | CI |

Commands: `./scripts/p0b-post-record-checklist.sh` then `./scripts/bar-b-falsification-run.sh`.

### Fill after re-run

| Field | A | B | C |
|-------|---|---|---|
| path_weight_ns | | | |
| path/wall | | | |
| Pass? | | | |

**Bar B product gate:** not **1/3 anecdotal** — target **≥80%** pass on `bar-b-scoped-batch.sh` with tightened pad after bounding fix.

---

## Overhead (httpgo, separate from p0b)

See [phase0/p0a-overhead.md](phase0/p0a-overhead.md):

- **full:** −0.93%, ~1% latency, 0 drops — **recommended**
- **min-block:** −1.31% + tail — **not recommended** until investigated
- **sampled:** **INVALID** last run — isolated re-run required

Overhead **≠** Bar B. Statement is about **httpgo event density**, not all workloads.

---

## Conclusion (precise wording)

| Claim | Status |
|-------|--------|
| P2 BPF + uprobes + analyzer shipped | **Yes** |
| Mechanism gate on live p0b | **Yes** |
| Bar B reachable on real capture | **Yes** (token C) |
| Bar B reliable / product gate | **No** — fix bounding, re-run batch, raise labeling |
| Thesis on real traffic | **Open** (Bar B + tail + netpoll) |
| Overhead existential risk (full, httpgo density) | **Retired** with caveats |
