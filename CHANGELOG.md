# Changelog

## v0.2.0 (unreleased) — Phase 2 Tier-2 + cleanup

### Added

- Go `gopark` uprobe: `wait_class`, `sudog.elem` → `event.aux`, subject stack at park.
- Path policy, parallel-edge merge, ELF symbolize, trace v2 modules, `validate-bar-b.sh`.
- [results/p2-validation.md](results/p2-validation.md) — prod0 Bar B pass (trace-joined chan ≥0.90).

### Changed

- Removed obsolete `docs/P2_IMPLEMENTATION_PROMPT.md`.
- Docs: separate **mechanism gate** vs **Bar B literal**; retract “Bar B ✓ on p0b” without live scoped path≈wall.
- `scripts/bar-b-scoped-live.sh` — scoped `analyze` on live trace vs GT handler wall time.
- `validate-bar-b.sh`: B1=mechanism eval; B2-live=bar-b-scoped-live; no `verify.sh` recursion.
- Default p0b trace: `/tmp/p0b-trace.criticast`.

## v0.1.0 (unreleased) — Phase 1 shippable core

### Added

- Trace format **v2** (`magic: CRTC`): header, stacks section, events, footer stats; v1 JSONL backward compatible.
- `criticast analyze` — Tier-0/1 segments, wait-for graph, SCC collapse, longest-path critical wait, dominant-wait ranking, text/JSON output.
- `criticast export --pprof` — gzip pprof profiles (`critical_wait` samples; single gzip layer for `go tool pprof`).
- `criticast probe-stats` — sched BPF counters without ringbuf (diagnostics).
- `internal/symbolize` — `Resolver` + trace stack_id → frames (PC placeholders for stripped binaries).
- Production attribution bridge (E3): mutex/pool/chan-without-elem → ambiguous, never high-confidence Tier-2 chan without `aux`.
- Golden trace fixture `testdata/traces/golden_chain.jsonl` + analyzer/CLI tests.
- GitHub Actions CI: `ci-go.sh`, `ci-lint.sh`, `ci-linux-bpf.sh`, `verify.sh`.
- Scripts: `demo-p1.sh` (wrk + record gate), `sched-smoke.sh`, hardened `ensure_httpgo`, `SKIP_APT` for broken apt hosts.
- BPF debug stats: `switch_seen`, `target_prev` (prove programs run vs targeting).
- Docs: [docs/P1_COMPLETION.md](docs/P1_COMPLETION.md), [results/p1-smoke.md](results/p1-smoke.md).

### Fixed

- `record` timer exit now prints BPF/userspace summary (`drainRecorder` regression).
- Ringbuf reader closes on context cancel (no spurious 3s drain hang).
- pprof export: removed double-gzip (`profile.Write` is already compressed).
- `ensure_httpgo` verifies exe path and listening port (avoids wrong `httpgo` on shared hosts).

### Validated

- Linux 6.1 cloud: 122k+ events, 0 ringbuf/chan drops, analyze + pprof ([results/p1-smoke.md](results/p1-smoke.md)).
- Phase 0 overhead and attribution baselines unchanged ([results/phase0/](results/phase0/)).

### Notes

- OTLP-Profiles export, live TUI, and k8s DaemonSet remain **P3** (`--otlp` exits 2 with message).
- BPF `sudog.elem` capture remains **P2**; live traces show `WC_UNKNOWN` at Tier-0/1 confidence until refinement probes ship.
- pprof function names require ELF symbolization (planned; frames may show `unknown` in v0.1).
