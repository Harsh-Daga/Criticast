# Overhead benchmark report

- **Date:** 2026-05-31
- **Host:** `prod0-telephony-voipmonitor-primary` — kernel `6.1.0-40-cloud-amd64`, Go 1.22.10
- **Commit:** `873a8c4` (bench complete)
- **Target:** `bin/httpgo` — `wrk -t4 -c100 -d30s` → `http://127.0.0.1:8080/`
- **Artifacts:** `/tmp/p0a-bench.log`, `/tmp/p0a-{full,min-block,sampled}-{base,treat}-{1..5}.txt`

## Verdict

**PASS (full mode)** on throughput vs charter §B.5 (&lt;5% regression). **min-block** and **sampled** also under 5% on median req/s.

| Mode | Median baseline rps | Median treatment rps | Throughput Δ | ringbuf drops |
|------|---------------------|----------------------|--------------|---------------|
| full (1µs, 1/1) | 12,551 | 12,438 | **−0.90%** | 0 |
| min-block (50µs, 1/1) | 12,559 | 12,428 | **−1.04%** | 0 |
| sampled (1µs, 1/100) | 12,571 | 12,525 | **−0.37%** | 0 |

P99 latency regression was **not** extracted from wrk text output (avg latency ~7.9–8.0 ms baseline vs ~7.9–8.0 ms treatment — no obvious tail blow-up). Re-run with `wrk -L` or latency log if strict P99 gate is required.

**Ship recommendation:** production path may use **full** or **sampled** collection at this load; **min-block** reduces userspace volume (~18% fewer ringbuf events vs full) with similar req/s impact.

## bpftrace spike — Linux server (2026-05-31T20:38Z)

| Metric | Value |
|--------|-------|
| Peak `@wakes`/s | **~12,200** |
| `@wakes` &gt; 1M/s? | **no** |
| `wrk` (spike script) | **~12,529 req/s** |
| Log | `results/phase0/spike-20260531T203847Z.log` |

## Bench detail (median of 5 runs)

### full

| Run | Baseline rps | Treatment rps |
|-----|--------------|---------------|
| 1 | 12,568 | 12,461 |
| 2 | 12,537 | 12,438 |
| 3 | 12,551 | 12,436 |
| 4 | 12,564 | 12,463 |
| 5 | 12,540 | 12,437 |

BPF treatment (run 1): ~675k events/30s, 0 drops, `sampled_out=0`.

### min-block (50µs)

| Run | Baseline rps | Treatment rps |
|-----|--------------|---------------|
| 1–5 medians | 12,559 | 12,428 |

BPF treatment: ~552k events/30s (fewer blocks after short filter).

### sampled (1/100)

| Run | Baseline rps | Treatment rps |
|-----|--------------|---------------|
| 1–5 medians | 12,571 | 12,525 |

BPF treatment: ~6.8k events/30s, `sampled_out` ~676k (sampling works).

## Environment

| Check | Result |
|-------|--------|
| `./scripts/verify.sh` | PASS |
| `casgstatus` smoke | PASS (goid_off=152) |
| `bench-p0a.sh` | completed (`873a8c4+`) |

## Notes

- `criticast: recorder stop timed out after 3s` after each treatment — ringbuf drained after wrk; **0 chan_drops**, events accounted for. Consider longer drain timeout in Phase 1.
- Recorder attaches **sched only** on bench (no `--go-binary` in `bench-p0a.sh`) — overhead table is L2 sched path.

## Charter gate (§B.5)

| Criterion | Result |
|-----------|--------|
| Throughput Δ full | **−0.90%** → PASS (&lt;5%) |
| Throughput Δ sampled | **−0.37%** → PASS |
| P99 Δ | not measured from saved wrk files |
| Wake rate | PASS (~12k/s ≪ 1M/s) |

## Related

Attribution results: [p0b-attribution.md](p0b-attribution.md). Engineering backlog: [docs/ROADMAP.md](../../docs/ROADMAP.md).
