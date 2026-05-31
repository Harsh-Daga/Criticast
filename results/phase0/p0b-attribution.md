# Attribution benchmark report

- **Date:** 2026-06-01 (E2 re-run); initial 2026-05-31
- **Host:** `prod0-telephony-voipmonitor-primary` (Debian, kernel `6.1.0-40-cloud-amd64`, BTF OK)
- **Go:** 1.22.10 (`/usr/local/go`)
- **Load:** interleaved A/B/C — `CONN=8 THREADS=1 DURATION=30s` × 3 (`./scripts/load-p0b-interleaved.sh`)
- **Artifacts:** `/tmp/p0b-gt.log`, `/tmp/p0b-trace.jsonl`, `/tmp/p0b-eval-e2.txt`

## Verdict

**PASS with tiering** — not a logic bug; the original **0.557 was E1 (lineage-only)**, which the charter already treats as insufficient for shared worker pools. **E2 (sudog-element matching) reaches 1.000** on the same interleaved A/B/C load once send/recv share an elem id.

| Gate | Threshold | E1 (lineage) | E2 (sudog elem) |
|------|-----------|--------------|-----------------|
| spawn-lineage | ≥90% | **PASS** 1.000 | **PASS** 1.000 |
| chan-work-handoff | ≥90% | **FAIL** 0.548 | **PASS** 1.000 |
| conn-pool / mutex | ≥90% | **PASS** 1.000 | **PASS** 1.000 |
| broadcast / netpoll | in fixture | N/A | N/A |
| E4 naive baseline | worse than E1 | **PASS** (0.000) | — |

**Product default:** Tier-0/1 (lineage) everywhere; Tier-2 chan only when `sudog.elem` (or equivalent) is observed — otherwise `request-ambiguous`. Do not ship lineage-only as confident chan attribution.

**End-to-end caveat:** GT-only E2 passes; trace-joined E2 is **chan 0.783** because `bpf/collector.c` does not yet write `last_sudog_elem`/`futex_uaddr` — `e->aux` is always 0. See [Trace join](#trace-join--e2-end-to-end-2026-06-01) and [docs/ROADMAP.md](../../docs/ROADMAP.md) (BPF sudog capture).

## E2 experiment (2026-06-01)

Fixture emits per-item elem id at `worker-pool-send` / `worker-recv` / `worker-done`; harness passes `parseElem(se.Aux)` into `RunExperiment`.

GT log sample:

```text
worker-pool-send ... "extra":"2"
worker-done        ... "extra":"4591"
```

`criticast eval --gt-log /tmp/p0b-gt.log --mode all` (GT-only gate metric):

| Mode | chan-work-handoff | spawn | pool | mutex | edges (chan) |
|------|-------------------|-------|------|-------|--------------|
| e1-lineage | 0.548 | 1.000 | 1.000 | 1.000 | 13773 |
| **e2-sudog** | **1.000** | 1.000 | 1.000 | 1.000 | 13773 |
| e3-suppress | 0.548 | 1.000 | 1.000 | 1.000 | 13773 |
| e4-naive | 0.000 | 0.000 | 0.000 | 0.000 | 13773 |

Record (30s, interleaved load): `emitted=53232`, `ringbuf_drops=0`, `chan_drops=0`.

## Prior run (2026-05-31, pre-E2 harness)

E2 was identical to E1 because `RunExperiment` passed `sudogElem=0`. Chan **0.557** on that run reflected **untested** sudog path, not a proven ceiling.

## Environment

| Check | Result |
|-------|--------|
| `./scripts/verify.sh` | PASS |
| `make bpf` (single TU) | PASS |
| `link.AttachTracing` for `tp_btf/*` | PASS |
| `casgstatus` + `offsets.json` go1.22.0 | PASS |
| Trace clock join | PASS |
| p0b map race (`cacheVal` under lock) | fixed |
| E2 elem in GT + `NoteSudogElem` on send | PASS (2026-06-01) |

## Trace join — E2 end-to-end (2026-06-01)

`./bin/criticast eval --gt-log /tmp/p0b-gt.log --trace /tmp/p0b-trace.jsonl --mode e2-sudog`

`trace join: block_ends=26530 with_goid=24925 labeled=17058 clock_corr=true`

| Mechanism | Precision | N edges |
|-----------|-----------|---------|
| spawn-lineage | 1.000 | 12701 |
| chan-work-handoff | **0.783** | 4260 |
| conn-pool | 1.000 | 97 |
| mutex | — | 0 |

**Why chan is 0.783 here but 1.000 GT-only:** the BPF path carries no sudog elem yet.
`bpf/collector.c` sets `e->aux = futex_uaddr ? : last_sudog_elem`, but **neither field is
ever written** in `bpf/` — so `ev.Aux == 0`, `te.SudogElem == 0`, and E2 falls back to the
waker-token heuristic (`engine.go` E2 branch). 0.783 = waker fallback alone, **not** elem
matching. This is the gate's known **Phase 1 task**, not a logic defect.

So:
- GT-only E2 1.0 → attribution **logic** correct given an elem key.
- Trace-joined E2 0.783 → **BPF does not yet emit `sudog.elem`**; production chan stays
  ambiguous until that probe lands.

## Ship recommendation

- Ship **wait-for graph + Tier-0/1** (scheduler edges, lineage spawn/pool/mutex).
- Chan attribution: **logic validated** (GT-only 1.0); **not yet** production-ready end-to-end
  (trace-joined 0.783) until BPF emits `sudog.elem`. Until then, label chan **`request-ambiguous`**.
- **Next (chan Tier-2 end-to-end):** see [docs/ROADMAP.md](../../docs/ROADMAP.md) — BPF `last_sudog_elem`, sudog TTL, fixture coverage.

## Commands (reproduce)

```bash
export PATH="/usr/local/go/bin:$PATH"
cd /path/to/criticast
make workloads
# Terminal A
./scripts/run-p0b.sh
# Terminal C (during record)
CONN=8 THREADS=1 DURATION=30s ./scripts/load-p0b-interleaved.sh
# Terminal B
sudo -E env PATH="$PATH" ./scripts/record-p0b.sh
./bin/criticast eval --gt-log /tmp/p0b-gt.log --mode all 2>&1 | tee /tmp/p0b-eval-e2.txt
grep -E '^(===|chan-work)' /tmp/p0b-eval-e2.txt
```
