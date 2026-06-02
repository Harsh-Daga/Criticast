# Project phases and release policy

**Authority:** [CHARTER.md](../CHARTER.md) Part I · [ROADMAP.md](ROADMAP.md) · [P1_COMPLETION.md](P1_COMPLETION.md)

---

## Release policy (project decision)

| Policy | Detail |
|--------|--------|
| **Public ship** | Only when the **full charter** through agreed scope (minimum **P0 + P1 + P2 + P3**) is complete and re-validated. |
| **No “P1 GA”** | P1 is an **internal milestone** (Tier-0/1 plumbing). Do not market as finished causal profiler. |
| **Charter wording** | Part I still says P1 “plant the flag” — this repo **defers** that until charter-complete ship. |

---

## Phase map

| Phase | Charter | Engineering status | Validation bar |
|-------|---------|-------------------|----------------|
| **P0** | Overhead + attribution gates | **Done** | [results/phase0/](results/phase0/) |
| **P1** | Tier-0/1 record/analyze/pprof | **Bar A done** (plumbing) | [P1_COMPLETION.md](P1_COMPLETION.md), [results/p1-smoke.md](../results/p1-smoke.md) |
| **P1 gap** | — | Bar B literal (partial) | Superseded by request-epoch pipeline — see [STATUS.md](../results/STATUS.md) |
| **P2** | Go task-level + Tier-2 + GT mechanism gate | **Mechanism done**; Bar B **code complete** | Mechanism ✓; **≥50-epoch batch on prod0** closes P2 |
| **P3** | k8s, OTLP, TUI, netpoll, labeling at scale | Planned | After Bar B statistical gate |
| **P4** | Tokio, Tier-3, JVM, io_uring | Research | Post-P3 |

**Active branch:** `phase2/tier2-product` (all P2 + Bar B work).

**Frozen reference:** `phase1/shippable-core` or `main` after P1 merge — plumbing baseline only.

---

## Two validation bars (do not merge)

| Bar | Question | Phase |
|-----|----------|-------|
| **A — Plumbing** | Does data flow kernel → trace → analyze → export without drops? | P1 ✓ |
| **B — Thesis** | For **scoped requests** on **live capture**, does critical path ≈ wall time with **labeled, non-idle** dominant waits? | **Open** — request epoch shipped; **≥80% of ≥50** GT epochs on prod0 ([STATUS.md](../results/STATUS.md)) |

Commodity scheduler graphs pass Bar A only (bcc offwaketime class). Criticast’s thesis is Bar B.

---

## Implementation briefs

| Doc | Use when |
|-----|----------|
| [P1_COMPLETION.md](P1_COMPLETION.md) | What P1 proved vs did not (Bar A / Bar B) |
| [ROADMAP.md](ROADMAP.md) | Backlog, P2 registry, Bar B thresholds |
| [CHARTER.md](../CHARTER.md) | Design authority (Parts C, E, H for P2) |

---

## Merge order (recommended)

1. Merge P1 plumbing to `main` (if not already).
2. Branch `phase2/tier2-product` from `main`.
3. Land P2 in reviewable PRs; `./scripts/verify.sh` + Bar B + P0 regressions each slice.
4. After P2: branch `phase3/operations` for P3.
5. Single public **v1.0.0** (or similar) when charter ship checklist passes.
