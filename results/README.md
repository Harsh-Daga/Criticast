# Benchmark reports

Committed measurement reports from Linux validation runs. Re-run procedures: [docs/GETTING_STARTED.md](../docs/GETTING_STARTED.md).

| Report | What it measures |
|--------|------------------|
| [phase0/p0a-overhead.md](phase0/p0a-overhead.md) | Probe overhead (throughput, wake rate, ringbuf drops) |
| [phase0/p0b-attribution.md](phase0/p0b-attribution.md) | Per-mechanism attribution precision (E1–E4) |
| [p1-smoke.md](p1-smoke.md) | Phase 1 plumbing (Bar A) on Linux 6.1 cloud |
| [p2-validation.md](p2-validation.md) | Phase 2 thesis gate template (Bar B) — fill when P2 done |

## Summary (reference host, 2026-06)

| Measurement | Result |
|-------------|--------|
| Overhead (full mode) | ~−0.9% median throughput; 0 ringbuf drops |
| spawn / pool / mutex (E1) | 1.0 precision |
| chan handoff (E1 lineage) | ~0.55 — expected for shared worker pool |
| chan handoff (E2 sudog, GT replay) | 1.0 — logic validated with elem id |
| chan handoff (E2, trace-joined) | ~0.78 — pending BPF `sudog.elem` capture |

| P1 E2E (demo-p1, 2026-06) | 122k events, 0 ringbuf drops, valid pprof on cloud kernel 6.1 |

Interpretation and backlog: [docs/ROADMAP.md](../docs/ROADMAP.md) · Phase 1 sign-off: [docs/P1_COMPLETION.md](../docs/P1_COMPLETION.md).

Local artifacts (not committed): `results/phase0/spike-*.log`, `/tmp/p0a-*.txt`, `/tmp/p0b-*`.
