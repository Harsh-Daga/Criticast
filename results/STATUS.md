# criticast validation status

**Updated:** 2026-06-02 · **Branch:** `phase2/tier2-product`

One-page scorecard. Detail: [p2-validation.md](p2-validation.md).

---

## Three gates

| Gate | Status |
|------|--------|
| **Bar A** + overhead (full @ httpgo density) | **Done** — [p0a-overhead](phase0/p0a-overhead.md) |
| **Mechanism** (trace-joined sudog chan) | **Done** — ≥0.99 on prod0 |
| **Bar B literal** | **Re-validate on prod0** — request epoch path (`request_epoch.go`, `request_path.go`) |

---

## Bar B: request epoch pipeline

When `--scope-handler-goid` is set (literal Bar B), analyze uses one path:

1. **`buildRequestEpoch`** — strict GT entry→exit + padded bpf window; membership from handler span + chan handoff only (max 16 tasks).
2. **`filterEpochEdges`** — handler wakee waits + edges between membership tasks; forward reach from handler.
3. **`computeEpochCriticalPath`** — temporal longest path + dominant-wait fallbacks; **`path_weight`** = sum of observed edge `blocked_ns` only (no GT wall fill).
4. **Membership** — handler wakee block-ends add waker workers (wait-for graph); not send-window chan handoff alone.
5. **RUNNING** — BPF `EV_TASK_STATE` on switch-out; segments in analyzer (diagnostics, not wall back-fill).
6. **`PathWeightInvariantOK`** — hard fail if path_weight > wall + 2ms slack (upper bound only).

**Prod0:** `./scripts/bar-b-scoped-batch.sh` samples **≥50** handler epochs from GT (`bar-b-sample-handlers.py`); target **≥80%** pass. Smoke: `bar-b-scoped-live.sh A|B|C`.

Design: [docs/p2-bar-b-epoch-path.md](../docs/p2-bar-b-epoch-path.md).

---

## P2 vs P3

| P2 (validation gate uses GT) | P3 (product / no GT) |
|------------------------------|----------------------|
| Bar B batch pass rate (50-sample) | Live token propagation on BPF |
| Request epoch temporal path | Tier-1 accept/recv anchor |
| Mechanism eval (chan) | Mutex join-survival |
| `labeled/block_ends` re-measure | Live BPF propagation (no GT) |

---

## Mechanism / mutex

| Mechanism | Trace-joined |
|-----------|----------------|
| chan-work-handoff (sudog) | **≥0.99** |
| spawn, conn-pool | 1.0 |
| **mutex** | join-survival **deferred to P3** |
| netpoll, broadcast | unimplemented (P3) |

---

## Overhead (not Bar B)

| Mode | Status |
|------|--------|
| full | **−0.93%**, recommended |
| min-block | PASS charter but not recommended vs full |

---

## Next prod0 commands

```bash
git pull && make bpf go workloads && go test ./...
./scripts/p0b-full-validate.sh
./scripts/bar-b-scoped-live.sh A
./scripts/bar-b-scoped-batch.sh
./scripts/validate-bar-b.sh
```
