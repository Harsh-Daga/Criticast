# Overhead benchmark report

- **Date:** 2026-06-02 (full re-validation on prod0; prior row 2026-05-31 retained below)
- **Host:** `prod0-telephony-voipmonitor-primary` — kernel `6.1.0-40-cloud-amd64`, Go 1.22.10 / httpgo
- **Target:** `bin/httpgo` — `wrk -t4 -c100 -d30s` → `http://127.0.0.1:8080/` @ ~12.5k req/s
- **Artifacts:** `/tmp/p0a-bench.log`, `/tmp/p0a-{full,min-block,sampled}-{base,treat}-{1..5}.txt`

## Stale after EV_TASK_STATE (2026-06)

BPF now emits `EV_TASK_STATE` on every targeted CPU exit (~`target_prev` rate). **Re-run `scripts/bench-p0a.sh`** before treating full-mode overhead as banked; check `running=` in bpf stats and ringbuf_drops at 12.5k req/s.

## Verdict (2026-06-02)

| Mode | Status | Median throughput Δ | Notes |
|------|--------|---------------------|--------|
| **full** (1µs, 1/1) | **PASS — recommended** | **−0.93%** (12,576 → 12,459 rps) | Latency +~1% (7.89→7.97 ms); **0** ringbuf/chan drops (5/5 runs) |
| **min-block** (50µs, 1/1) | **PASS charter (&lt;5%) but not recommended** | **−1.31%** (12,581 → 12,416 rps) | Worse than full despite ~35% fewer block events; **latency tail** (treatment max 33–41 ms vs ~20 ms full); investigate before recommending |
| **sampled** (1µs, 1/100) | **INVALID this run** | ~~−0.37%~~ | Treatment run 1: ~90% non-2xx, 75k write errors, harness `Terminated` — **not** sampled overhead; re-run isolated (fresh server, baseline-then-treatment only) |

**Bar A / P0 gate:** full-mode uprobe + sched path overhead at **httpgo event density** (~23k BPF events/s, ~135k ctx switches/s, ~11.5k blocks/s) is **under 1%** with zero drops. That retires the early “existential” overhead risk for this workload class.

**Not claimed here:**

- **Bar B / thesis** — overhead ≠ scoped path ≈ wall time ([p2-validation.md](../p2-validation.md)).
- **Universal ceiling** — overhead tracks **event rate**, not req/s. p0b emits ~2× events/s vs httpgo at similar load; channel-heavy services not delta-measured on this bench.
- **min-block** as default — currently **dominated by full** on both throughput and tail.

## full mode detail (5 runs, 2026-06-02)

| Metric | Baseline (median) | Treatment (median) |
|--------|-------------------|---------------------|
| req/s | ~12,576 | ~12,459 |
| avg latency | ~7.89 ms | ~7.97 ms |
| ringbuf/chan drops | — | **0** (all runs) |

## min-block mode detail

- Filter cut blocks ~346k → ~226k (`short_filt` ~123k) but **higher** throughput loss than full.
- Run 5: preempts ~101k vs ~69k full — extra hot-path branch cost suspected.
- **Action:** investigate filter placement / cost before recommending min-block ([ROADMAP.md](../../docs/ROADMAP.md)).

## sampled mode — invalid run (do not use for ship decisions)

| Run | Symptom |
|-----|---------|
| 1 treatment | 37,420 rps, 3.99 ms latency, **1,009,289** non-2xx/3xx / 1,125,238 total, 75,021 write errors |
| 2 baseline | `Terminated` |

Likely **harness fatigue** (single httpgo PID reused across 11+ × 30s runs, port/TIME_WAIT exhaustion) ± possible sampled-path instability — **cannot separate** without isolated re-run.

**Required follow-up:** fresh server, fresh ports, baseline-then-treatment only, no prior attach/detach hammering.

## Prior bench (2026-05-31, commit `873a8c4`)

Earlier medians (sched-only recorder in `bench-p0a.sh`, no gopark uprobes):

| Mode | Median Δ |
|------|----------|
| full | −0.90% |
| min-block | −1.04% |
| sampled | −0.37% |

Post-P2 BPF hot path should be re-benchmarked after gopark uprobes land in `bench-p0a.sh`; 2026-06-02 numbers above include full Go probe path on prod0.

## bpftrace spike (2026-05-31)

| Metric | Value |
|--------|-------|
| Peak `@wakes`/s | ~12,200 |
| `wrk` | ~12,529 req/s |

## Charter gate (§B.5)

| Criterion | Result |
|-----------|--------|
| Throughput Δ full | **−0.93%** → PASS (&lt;5%) |
| Throughput Δ min-block | −1.31% → PASS charter, **not recommended** |
| Throughput Δ sampled | **invalid run** — re-run |
| P99 Δ | tail regression on min-block; full ~flat on avg |

## Related

- P2 validation (Bar B, mechanism): [p2-validation.md](../p2-validation.md)
- Consolidated status: [STATUS.md](../STATUS.md)
- Backlog: [docs/ROADMAP.md](../../docs/ROADMAP.md)
