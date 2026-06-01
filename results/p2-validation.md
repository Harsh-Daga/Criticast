# Phase 2 validation report (template)

**Status:** Not started. Fill this when `scripts/validate-bar-b.sh` passes on a reference host.

## Prerequisites

- P1 plumbing merged to `main`
- Branch `phase2/tier2-product` complete
- [docs/ROADMAP.md](../docs/ROADMAP.md) Phase 2 definition of done · [docs/P1_COMPLETION.md](../docs/P1_COMPLETION.md) Bar B bar

## Environment

| Item | Value |
|------|--------|
| Host | |
| Kernel | |
| Go | |
| Commit | |

## B1 — p0b GT attribution (`eval --mode all`)

| Mode | Precision | Notes |
|------|-----------|-------|
| spawn-lineage | | |
| conn-pool | | |
| mutex | | |
| chan-work-handoff (trace-joined) | | target ≥0.90 |

## B2 — Scoped live request

| Check | Result |
|-------|--------|
| `--request` critical path non-empty | |
| ≥1 non-`WC_UNKNOWN` on path | |
| path_weight vs wall clock (±30%) | |
| No `task_id=1→1` dominant | |
| pprof named frames | |

## B3 — Stability

| Check | Result |
|-------|--------|
| analyze text/json same path | |
| scoped path weight GO_UPROBES 0 vs 1 within 2× | |

## Overhead (post-BPF)

Re-run [phase0/p0a-overhead.md](phase0/p0a-overhead.md) addendum if hot path changed.

## Conclusion

- [ ] Bar B passed — thesis validated on this host
- [ ] P2 complete — ready for P3 (not public GA until charter ship policy)
