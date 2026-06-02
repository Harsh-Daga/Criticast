# Bar B falsification test (P2 gate supplement)

The 50-sample batch checks `path_weight` within ±30% of GT wall. That alone cannot distinguish a correct causal path from a tautology if any term couples to wall. This experiment separates **“the number matches”** from **“the tool is right.”**

## Setup (p0b)

1. Record a baseline trace + GT log (`p0b-full-validate.sh`) **after** `make bpf` (trace must include `EV_TASK_STATE`).
2. Run falsification script (records baseline + slowed worker for token A):

```bash
# default: P0B_WORKER_SLOW_TOKEN=A P0B_WORKER_SLOW_NS=8000000 P0B_SLOW_WORKER_ID=0
./scripts/bar-b-falsification-run.sh
```

Worker slowdown applies only on worker `P0B_SLOW_WORKER_ID` (default `0`) for items with that token, so B/C are not slowed on every dequeue.

Or manually: `P0B_WORKER_SLOW_TOKEN=A P0B_WORKER_SLOW_NS=8000000 P0B_SLOW_WORKER_ID=0 ./scripts/run-p0b.sh` then re-record.

## Predictions (if Bar B is honest)

| Check | Token A | Token B/C |
|-------|---------|-----------|
| Handler wall | Increases by ≈ Δ | Unchanged |
| Dominant path edge | Points at **slowed worker** wake → handler | Does not name A’s slowed worker |
| `path_weight` increase | Explained by **blocked_ns on that wait edge** | No smear into handler “running gap” |
| Mechanism label on path | `WC_CHAN` or handoff meta on the **wakeup** edge | — |

## Fail signals (tautology or wrong bridge)

- `path_weight ≈ wall` but dominant edge is idle or unrelated.
- Added latency appears only as inflated occupancy with **no** heavier worker→handler edge.
- Token B/C paths pick up A’s slowed worker (scope leak).

## Script hook

After implementation in the server, run:

```bash
./scripts/bar-b-scoped-batch.sh   # baseline pass rate
# deploy A-only slowdown, re-record, then batch again
```

Document results in [results/p2-validation.md](../results/p2-validation.md).
