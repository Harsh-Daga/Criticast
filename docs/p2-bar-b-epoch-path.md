# P2 Bar B: Request Epoch critical path

**Authority:** [CHARTER.md](../CHARTER.md) Part E.3–E.5, Part H.2 (p0b). **Scope:** literal Bar B with GT (`--gt-log`, `--scope-handler-goid`, `--scope-from` / `--scope-to`).

## Unit of analysis

A **Request Epoch** is one GT `handler-entry` → `handler-exit` for `(token, handler_goid)`. All ktime bounds, membership, and `path_weight` are relative to that wall-clock span (with a small BPF padding window for edge discovery only).

## Membership (small set)

1. **Pinned handler only:** calibrate BPF `task_id` from GT sites on `handler_goid` during entry→exit (not every concurrent handler for the token).
2. **Wakeup bridge:** BPF `task_id`s that were **waker** on `EV_BLOCK_END` where **wakee == handler** during strict epoch ktime (direct only; no transitive closure through shared workers).
3. Do **not** use token-wide GT windows, cookie scans, or chan send-window expansion (those flood concurrent requests).

Record with `--emit-running` only when investigating RUNNING occupancy; Bar B `path_weight` is wait-edge sum only (default record omits RUNNING events).

Reject epochs with more than `maxEpochMembership` (16) calibrated tasks.

## Edges

- Every wait where **wakee == handler** `task_id` (includes worker → handler result delivery).
- Waits between two membership `task_id`s.
- Forward reachability from handler seed (no bidirectional pool closure).

## Path and path_weight

1. Discover edges in **padded** ktime; clip edge weights with padded overlap when strict clip is zero (GT/BPF skew).
2. Temporal longest path (`LongestPathTemporal`) on SCC-collapsed graph from handler seed.
3. Fallbacks: dominant handler wait → heaviest labeled wait (`path_edges ≥ 1`).
4. **`path_weight`** = sum of `blocked_ns` on path edges from the DP — **never** `epoch.WallNs`, never `wall − blocked_union`, never back-fill from GT wall. If `path_weight ≪ wall` after measured RUNNING segments exist, the capture or scope is incomplete (honest fail or wait-bound-only gate).

## Measured RUNNING (P2)

BPF emits `EV_TASK_STATE` on `sched_switch` when a targeted task leaves CPU: `running_ns = now − last_switch_in_ns` (charter E.1). Userspace builds `Running` segments in `segment.go`. RUNNING is used for occupancy diagnostics (`measuredHandlerOccupancyNs`, `bar-b-probe-epoch.sh`), **not** to synthesize `path_weight` from wall.

## P2 vs P3

| P2 (this doc) | P3 |
|---------------|-----|
| GT handler epoch required for Bar B literal | Cookie / Tier-1 anchor without GT |
| Mechanism eval (chan handoff at send) | Mutex join-survival, netpoll |
| 50-sample batch + falsification test | Live propagation |

## Falsification (required for honest batch gate)

See [p2-bar-b-falsification.md](p2-bar-b-falsification.md): slow the worker for token A only; A’s critical path must name that worker and explain added wall on the **observed wait edge**, not on inferred running gap.
