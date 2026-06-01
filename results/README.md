# Benchmark reports

Committed measurement reports from Linux validation runs. Re-run procedures: [docs/GETTING_STARTED.md](../docs/GETTING_STARTED.md).

| Report | What it measures |
|--------|------------------|
| [STATUS.md](STATUS.md) | **One-page scorecard** (Bar A / mechanism / Bar B / overhead) |
| [phase0/p0a-overhead.md](phase0/p0a-overhead.md) | Probe overhead — full/min-block/sampled (2026-06-02 prod0) |
| [phase0/p0b-attribution.md](phase0/p0b-attribution.md) | Per-mechanism attribution (E1–E4, trace-joined) |
| [p1-smoke.md](p1-smoke.md) | Phase 1 plumbing (Bar A) on Linux 6.1 cloud |
| [p2-validation.md](p2-validation.md) | Phase 2 validation detail |

## Summary (prod0, 2026-06-02)

| Measurement | Result |
|-------------|--------|
| P2 trace-joined chan | **1.000** |
| Bar B literal (live) | **1/3** (C pass; A/B ~67× — bounding fix shipped, re-validate) |
| Overhead full @ 12.5k rps | **−0.93%**, 0 drops — [p0a-overhead](phase0/p0a-overhead.md) |
| min-block | −1.31% + tail — not recommended |
| sampled | **invalid run** — re-run isolated |
| Live record | ~1.5M events, **0** ringbuf drops |
| P1 E2E (`demo-p1`) | Bar A only |

Interpretation: [docs/ROADMAP.md](../docs/ROADMAP.md) · Phases: [docs/PHASES.md](../docs/PHASES.md) · P1 sign-off: [docs/P1_COMPLETION.md](../docs/P1_COMPLETION.md).

Local artifacts (not committed): `results/phase0/spike-*.log`, `/tmp/p0a-*.txt`, `/tmp/p0b-*`.
