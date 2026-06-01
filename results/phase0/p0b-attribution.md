# Attribution benchmark report

- **Date:** 2026-06-02 (P2 trace-joined re-verify); 2026-06-01 (E2 GT harness); initial 2026-05-31
- **Host:** `prod0-telephony-voipmonitor-primary` (Debian, kernel `6.1.0-40-cloud-amd64`, BTF OK)
- **Go:** 1.24.0 (`/usr/local/go`) on P2 runs; 1.22.10 on earlier runs
- **Load:** interleaved A/B/C — `CONN=8 THREADS=1 DURATION=30s` × 3 (`./scripts/load-p0b-interleaved.sh`)
- **Artifacts:** `/tmp/p0b-gt.log`, `/tmp/p0b-trace.criticast` (v2 trace)

## Verdict (2026-06-02, Phase 2)

**PASS — mechanism gate** (trace-joined `chan-work-handoff` **0.995–1.000**, threshold ≥0.90). BPF emits `sudog.elem` via `gopark` → `event.aux`.

**Bar B literal** (scoped live `path≈wall`) is **open** — see [../p2-validation.md](../p2-validation.md#b2-live--scoped-request-on-real-capture-open) and `./scripts/bar-b-scoped-live.sh`.

| Gate | Threshold | E1 (lineage) | E2 (GT sudog) | **Trace-joined (live BPF)** |
|------|-----------|--------------|---------------|-----------------------------|
| spawn-lineage | ≥90% | 1.000 | 1.000 | **1.000** |
| chan-work-handoff | ≥90% | ~0.55 | 1.000 | **0.995–1.000** |
| conn-pool | ≥90% | 1.000 | 1.000 | **1.000** |
| mutex (GT-only) | ≥90% | 1.000 | 1.000 | trace-join sample often 0 edges |
| ringbuf drops | 0 | — | — | **0** |

**Product rule (unchanged):** Tier-2 chan confidence only when `event.aux` (sudog.elem) is present; else `request-ambiguous`. Do not ship lineage-only as confident chan attribution.

Full P2 report: [../p2-validation.md](../p2-validation.md).

---

## P2 live run — record + eval (2026-06-02)

**Record:** `emitted=1412508`, `blocks=697589`, `stack_fail=218268` (~15%), `ringbuf_drops=0`.

**Eval (`--mode all`) — trace-joined:**

| Mechanism | Precision | Recall | Edges |
|-----------|-----------|--------|-------|
| spawn-lineage | 1.000 | 1.000 | 92407 |
| chan-work-handoff | **0.995** | 0.995 | 13509 |
| conn-pool | 1.000 | 1.000 | 231 |

**Smaller prior capture (same host):** trace-joined chan **1.000** (1741 edges).

---

## Historical — pre-BPF sudog (2026-06-01)

Before `runtime.gopark` + `last_sudog_elem` in `bpf/collector.c`, trace-joined chan was **~0.783** (waker heuristic only). GT-only E2 was already **1.000**, proving attribution logic, not BPF transport.

| Mode | chan-work-handoff | Notes |
|------|-------------------|-------|
| e1-lineage | 0.548 | GT replay only |
| e2-sudog | 1.000 | elem in GT `extra` |
| trace-joined (pre-P2 BPF) | 0.783 | `aux` not populated |

That gap is **closed** as of P2 validation above.

---

## E2 experiment (2026-06-01, GT harness)

Fixture emits per-item elem id at `worker-pool-send` / `worker-recv` / `worker-done`.

`criticast eval --gt-log /tmp/p0b-gt.log --mode all` (GT-only):

| Mode | chan-work-handoff | spawn | pool | mutex |
|------|-------------------|-------|------|-------|
| e1-lineage | 0.548 | 1.000 | 1.000 | 1.000 |
| **e2-sudog** | **1.000** | 1.000 | 1.000 | 1.000 |
| e3-suppress | 0.548 | 1.000 | 1.000 | 1.000 |
| e4-naive | 0.000 | 0.000 | 0.000 | 0.000 |

---

## Environment

| Check | Result |
|-------|--------|
| `./scripts/verify.sh` | PASS |
| `make bpf` (`up_gopark`, sched) | PASS |
| `validate-bar-b.sh` | PASS (2026-06-02) |
| Trace clock join | PASS |
| E2 elem in GT + BPF `aux` | PASS (P2) |

---

## Ship recommendation

- **P2 merge:** Supported on Linux with published numbers above.
- **Public GA:** Not yet — [PHASES.md](../../docs/PHASES.md) requires P3+ (k8s, OTLP, operations).
- **Chan on production apps:** Use Tier-2 only when `aux` nonzero; label ambiguous otherwise.
- **Next:** P3 operations, `bench-p0a.sh` post-P2, reduce `stack_fail`, Tier-1 socket anchor.

---

## Commands (reproduce)

```bash
export PATH="/usr/local/go/bin:$PATH"
cd /path/to/criticast
make bpf go workloads

./scripts/run-p0b.sh >>/tmp/p0b-gt.log 2>&1 &
sleep 2 && curl -sf http://127.0.0.1:8080/health
CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh &
sleep 2
OUT=/tmp/p0b-trace.criticast DUR=30s ./scripts/record-p0b.sh

./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.criticast --mode all 2>&1 | tee /tmp/p0b-eval.log
grep -A8 '^=== trace-joined ===' /tmp/p0b-eval.log
```
