# Changelog

## v0.1.0 (unreleased) — Phase 1 shippable core

### Added

- Trace format **v2** (`magic: CRTC`): header, stacks section, events, footer stats; v1 JSONL backward compatible.
- `criticast analyze` — Tier-0/1 segments, wait-for graph, SCC collapse, longest-path critical wait, dominant-wait ranking, text/JSON output.
- `criticast export --pprof` — gzip pprof profiles (`critical_wait` samples, waker stacks when present).
- `internal/symbolize` — `Resolver` + trace stack_id → frames (PC placeholders for stripped binaries).
- Production attribution bridge (E3): mutex/pool/chan-without-elem → ambiguous, never high-confidence Tier-2 chan without `aux`.
- Golden trace fixture `testdata/traces/golden_chain.jsonl` + analyzer/CLI tests.
- GitHub Actions CI (`scripts/verify.sh` on Linux).

### Notes

- OTLP-Profiles export, live TUI, and k8s DaemonSet remain **P3** ( `--otlp` exits 2 with message).
- BPF `sudog.elem` capture remains **P2**; do not expect end-to-end chan Tier-2 in production traces yet.
