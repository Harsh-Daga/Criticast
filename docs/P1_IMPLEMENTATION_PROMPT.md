# Phase 1 implementation prompt (agent / engineer spec)

Use this document as the **single execution brief** for Phase 1. Copy the [Agent prompt](#agent-prompt-copy-paste-block) section into a new Cursor/Claude session after checking out the branch.

**Authority:** [CHARTER.md](../CHARTER.md) > [AGENTS.md](../AGENTS.md) > [docs/ROADMAP.md](ROADMAP.md) > this prompt.

---

## Git workflow (mandatory)

```bash
git fetch origin
git checkout main && git pull
git checkout -b phase1/shippable-core
# All P1 work lands on this branch only. No direct commits to main.
```

- Branch name: **`phase1/shippable-core`** (or `phase1/analyze-export` if you split PRs later).
- PRs: small, reviewable slices (e.g. trace v2 → symbolize → analyze → export → CI).
- Every PR: `./scripts/verify.sh` green; Linux CI job compiles BPF when touched.
- Do not amend charter scope into P2/P3 (k8s DaemonSet, OTLP default, live TUI, Tokio, full Tier-2 chan default).

---

## Phase 1 mission (charter Part I)

**Duration target:** 4–6 weeks of engineering.

**Deliverable:** First **shippable** open-source release — “plant the flag”:

| In scope (P1) | Out of scope (defer) |
|---------------|----------------------|
| Tier-0/1 default (scheduler wait-for + thread/tid; lineage when available) | Tier-2 chan as **default** high-confidence |
| `criticast record` (harden + stable flags) | `criticast top` live TUI (P3) |
| **`criticast analyze`** on trace files | k8s DaemonSet (P3) |
| Critical-path **text** + simple **flamechart** aggregation | OTLP-Profiles **default** export (stub/flag OK) |
| **`criticast export --pprof`** (stable default) | Full gopark/sudog BPF (P2) |
| **`internal/symbolize`** (minimal: stack_id → frames) | Distributed tracing / L7 parsing |
| Trace format v2 toward charter Appendix P | JVM / Tokio (P4) |
| CI: build + test + BPF compile on Linux | Full kernel 5.8–6.12 matrix (stretch) |
| README demo: nginx **or** Go svc + screenshot path | |

**Success statement (Part M):** A user can record 10s on a busy service, run `analyze`, see the dominant wait for a request, and open **`pprof`** in standard tools — **zero app code changes**, in under five minutes.

---

## Charter decision registry (read CHARTER.md — this table tracks every decision)

**Rule:** `CHARTER.md` is the source of truth. This registry maps charter sections to P1 work so nothing is “forgotten” — items marked **DEFER** are intentional, not omissions.

### Part 0 — Thesis and degradation ladder (§0.3–§0.5)

| Charter decision | P1 action |
|------------------|-----------|
| v2 was wrong: **never** blind waker-cookie propagation at shared resources | **IN** — default UX + attribution in analyze |
| Tier-2 failure = feature downgrade, not project death | **IN** — ship Tier-0/1; ambiguous labels |
| Degradation ladder: full vision → lineage-only → Tier-1 → Tier-0 → sampled-only | **IN** — P1 lands on **Tier-0/1** rung; document in README |
| Overhead unproven until measured | **DONE** (validation); re-smoke on BPF changes |
| §0.4 honest reminders: spinlocks/busy-wait blind spot; not a full APM replacement | **DOC** — README limitations + Part K #2 |

### Part A — Physical model

| Charter decision | P1 action |
|------------------|-----------|
| BLOCKED vs RUNNABLE vs RUNNING segments | **IN** — `internal/analyzer` segments (E.1) |
| `prev_state == 0` → preempt → RUNNABLE, not BLOCKED | **IN** — preserve BPF + analyzer; tests |
| Wait-for edge at `sched_waking`; waker = current task | **IN** — already in collector |
| Prefer `sched_waking` over `sched_wakeup` | **IN** — already attached |
| RUNQ as first-class wait class (`WC_RUNQ`, synthetic scheduler node) | **IN** — emit `EV_RUNQ`; graph edges to scheduler node |
| Thread-level edges sufficient for bottlenecks; per-request = our addition | **IN** — analyze scopes per cookie/tid when available |

### Part B — Kernel collector

| Charter decision | P1 action |
|------------------|-----------|
| Fixed `struct event` 80 bytes; version bump if changed | **IN** — trace header `struct_event_version` |
| Filter: targeted → block vs preempt → min_block → sample → stack | **IN** — do not regress |
| Cookie **read** in kernel, **written** by L3/refinement only | **IN** — no cookie writes in sched hot path |
| Ringbuf drop + stat; never block probe | **IN** — do not regress |
| B.4 refinement probes (futex, epoll, accept, spawn, io_uring, …) | **PARTIAL** — P1: document + stub hooks OK; **full attach DEFER** unless needed for Tier-1 demo |
| Tier-1: `accept`/`recv` open span, `send`/`write` close | **PARTIAL** — design in trace `SPANS` section; BPF syscall probes **stretch** or **DEFER** with Tier-0-only demo |
| B.5 overhead budget &lt;5% / &lt;10% P99 | **IN** — maintain; re-benchmark if BPF changes |
| Overhead levers: min_block, sample, stack gate, tp_btf | **IN** — already; document in operator docs |
| Cookie-seeded sampling for per-request consistency | **PARTIAL** — implement if `--sample` &gt;1; else document as P2 polish |

### Part C — Attribution tiers

| Charter decision | P1 action |
|------------------|-----------|
| **C.1 Tier-0** — per-process causal wait chains, no request id | **IN** — `analyze` without `--request`; top waits |
| **C.2 Tier-1** — accept/recv anchor, span open/close, cookie on tid | **PARTIAL** — analyze uses tid + optional cookie; full span BPF **stretch** |
| **C.3 Tier-2** — lineage-first; spawn tree; sudog.elem for work-handoff | **PARTIAL** — userspace lineage when GT/cookie known; **confident Tier-2 DEFER** (P2 BPF) |
| C.3.4 resource suppression (mutex/pool — no waker cookie) | **IN** — E3 behavior in analyze confidence |
| C.3.5 false-wakeup suppression (ε) | **IN** — analyzer E.4 |
| C.3.6 confidence = f(mech, entropy, waiters, timing) | **PARTIAL** — P1: mech + ambiguous flags; **full entropy detector DEFER** (stub `w_entropy=1` or simple placeholder + test hook) |
| C.4 Tier-3 cross-process / OTel seed | **DEFER** P3+ (optional ingest stub forbidden as default) |
| C.5 runtime resolvers: `none` (tid), `go` (casgstatus) | **IN** — `--runtime go|none`; degrade to tid if offsets fail (Q.2) |
| `runtime.newproc1` / `EV_SPAWN` for lineage | **PARTIAL** — use existing goid map; **EV_SPAWN in trace DEFER** unless BPF time |

### Part D — Userspace agent

| Charter decision | P1 action |
|------------------|-----------|
| Ringbuf drain, bounded channel, drop accounting | **IN** — harden `record` footer in trace |
| Symbolization pipeline | **IN** — `internal/symbolize` |
| Sharded parallel analyze (cookie mod N) | **PARTIAL** — single-threaded analyze OK for P1; **sharding DEFER** unless traces &gt;1M events force it |

### Part E — Analyzer algorithm

| Charter decision | P1 action |
|------------------|-----------|
| E.1 Segment reconstruction | **IN** |
| E.2 Cascade weight redistribution (wPerf) | **IN** — implement or document equivalence in longest-path weighting; prefer **IN** for fidelity |
| E.3 SCC collapse per request | **IN** |
| E.3 longest weighted path; node=on-CPU, edge=blocked | **IN** |
| E.4 False-wakeup ε (5–20µs tunable) | **IN** — flag `--spurious-wake-us` |
| E.5 DAG longest path after SCC | **IN** |
| E.6 Aggregate flamechart across requests | **IN** — `--top N` |
| Invariant: `path_weight ≈ wall_clock` | **IN** — golden test |
| Ambiguous edges below confidence threshold shown separately | **IN** — text + JSON output sections |
| Jaccard vs GT critical path ≥0.7 to market “the critical path” | **PARTIAL** — measure on adversarial trace in CI if GT available; else market **“dominant waits”** until proven |

### Part F / G — Layering and stack

| Charter decision | P1 action |
|------------------|-----------|
| Layer separation L0–L5 | **IN** — respect package boundaries |
| C + libbpf + CO-RE + BTF; Go cilium/ebpf | **IN** — no Rust kernel path |
| Min kernel 5.8+, BTF required | **IN** — document |

### Part H — Validation methodology (completed)

| Charter decision | P1 action |
|------------------|-----------|
| P0-A overhead gates | **DONE** — do not regress |
| P0-B per-mechanism matrix | **DONE** — keep `criticast eval` + CI |
| Ship decision table (lineage ≥90%, work-handoff tiering) | **IN** — product follows **lineage default + ambiguous chan** |

### Part I — Roadmap phase boundaries

| Charter decision | P1 action |
|------------------|-----------|
| P1 = Tier-0/1 + record/analyze + flamechart + pprof | **IN** — this branch |
| P2 = Go task Tier-2 + sudog/gopark BPF | **DEFER** |
| P3 = sampled agent, k8s, OTLP, TUI | **DEFER** |
| Each phase: CI green, overhead re-measured, docs, release | **IN** — Sprint 4 |

### Part J — Deployment and CLI

| Charter decision | P1 action |
|------------------|-----------|
| J.1 CLI: record, analyze, top, export | **IN** record/analyze/export; **top DEFER** |
| `--pid`, `--dur`, `--min-block`, `--sample`, `-o trace` | **IN** |
| `--runtime auto\|go\|none` | **IN** (`auto` = detect Go binary) |
| `--cgroup`, `--k8s-pod` | **DEFER** P3 (cgroup **stretch**) |
| analyze: `--request`, `--top`, `--min-confidence`, `--format` | **IN** |
| export: `--pprof` default; `--otlp` flagged | **IN** pprof; **otlp stub DEFER** |
| CAP_BPF + CAP_PERFMON, not privileged | **IN** — all docs/examples |
| BTF required; BTFhub fallback documented | **IN** — operator note |
| Complement APM; no payload inspection | **IN** — README |

### Part K — Edge cases (handle or document in P1)

| # | Edge case | P1 action |
|---|-----------|-----------|
| 1 | Preempt vs block | **IN** |
| 2 | Busy-wait / spinlocks blind spot | **DOC** — known limitation |
| 3 | io_uring blind spots | **DOC**; probes **DEFER** |
| 4 | Broadcast / thundering herd | **PARTIAL** — confidence `w_waiters`; full TTL **DEFER** |
| 5 | No Go uretprobes | **IN** |
| 6 | goid offset drift | **IN** — offsets.json + DWARF path design; CI test |
| 7 | task_id vs tid (M:N) | **IN** — prefer task_id when go present |
| 8 | Tokio id reuse | **DEFER** P4 |
| 9 | Ring overflow | **IN** — drops counted |
| 10 | Stripped binaries | **IN** — symbolize degrades to PC |
| 11 | PID namespaces / containers | **DOC**; build-id cache **PARTIAL** |
| 12 | Short-lived threads | **DEFER** |
| 13 | Clock domains | **IN** — ktime order; wall for display |
| 14 | GC pauses | **PARTIAL** — `WC_GC` when gopark reason **DEFER** |
| 15 | D-state vs S-state | **PARTIAL** — classify_prev disk vs unknown |
| 16 | Shared-resource false attribution | **IN** — core product rule |
| 17 | chan vs conn-pool | **IN** — ambiguous without elem |

### Part L — Risks

| Risk | P1 mitigation |
|------|----------------|
| Overhead unacceptable | Re-run bench if BPF touched |
| Tier-2 too noisy | Tier-0/1 default; never confident chan without elem |
| Scope creep to distributed tracing | **OUT** — charter discipline in PR review |

### Part M — Success / ROI

| Charter decision | P1 action |
|------------------|-----------|
| P0-A met; lineage shippable; Jaccard ≥0.7 or market “dominant waits” | **IN** — honest README wording |
| HN-grade demo nginx + Go | **STRETCH** — at least one polished Go demo required |
| &lt;5 min root-cause story | **IN** — README walkthrough |

### Appendix P — Wire formats

| Charter decision | P1 action |
|------------------|-----------|
| Trace: magic CRTC, sections STACKS, BUILDIDS, EVENTS, SPAWNS, SPANS, footer stats | **IN** — v2 header; STACKS required; SPAWNS/SPANS **PARTIAL** |
| pprof default export | **IN** |
| OTLP Profiles | **DEFER** P3 |
| JSON debug export of DAG | **IN** — `analyze --format json` |

### Appendix Q — Operational artifacts

| Charter decision | P1 action |
|------------------|-----------|
| bpftrace spike script | **KEEP** — `make spike` |
| offsets.json schema; DWARF-first | **IN** — maintain; refuse goid → thread-level |
| k8s DaemonSet YAML | **DEFER** P3 — do not half-ship |

### Appendix R — Testing and security

| Charter decision | P1 action |
|------------------|-----------|
| Attribution regression fixture (adversarial svc) | **IN** — keep eval + CI test thresholds |
| Threat model: read-only, no payload | **IN** |
| R.2 userspace resource budget (CPU/mem targets) | **DOC** — note targets; enforce bounded channels in record |
| CI kernel matrix compile | **PARTIAL** — one Linux job minimum; full matrix **stretch** |

### Appendices N & O — Reference only

| Charter section | P1 action |
|-----------------|-----------|
| Appendix N — verified kernel facts & sources | **READ** — cite when touching BPF; do not duplicate in code comments |
| Appendix O — glossary (`cookie`, `wait_class`, `EV_*`, …) | **IN** — use charter names in CLI, trace, and code |

### Mandatory reading list for implementers

Before writing code, read **in full**:

1. CHARTER **§0.3** (v2 correction) and **§0.5** (degradation ladder)  
2. **Part A** (physical model)  
3. **Part B** §B.1–B.3 + B.5 (skip implementing all B.4 probes unless stretch)  
4. **Part C** §C.1–C.3.6 (implement C.1–C.2 + suppression + confidence basics)  
5. **Part E** (entire analyzer algorithm)  
6. **Part J** (CLI + security)  
7. **Part K** (skim all 17 — document or implement)  
8. **Appendix P** (trace + export)  
9. **Appendix O** (glossary — naming consistency)  
10. AGENTS.md + docs/ROADMAP.md (validation numbers)  
11. Skim **Appendix N** when changing probes; **Appendix R** for security/CI

---

## Non-negotiable learnings from validation (Phase 0)

These are **requirements**, not suggestions. Violating them reintroduces the v2 attribution bug or invalidates overhead claims.

### 1. Wait-for vs request identity

| Rule | Implementation |
|------|----------------|
| **Wait-for edges** | Always preserve `wakee → waker` from `sched_waking`, weighted by `blocked_ns`. |
| **Request cookie / token** | **Lineage-first** (spawn tree, Tier-1 anchors). **Never** copy waker cookie on every wakeup. |
| **Mutex / conn pool / shared workers** | Do **not** propagate waker identity. Record contention; keep waiter's lineage or mark ambiguous. |
| **Channel work handoff** | Default **ambiguous** in P1 unless `event.aux` carries a validated sudog/elem key (P2). Do not ship confident chan labels from lineage alone. |
| **Uncertainty** | `request-ambiguous` + `confidence` field on every attributed edge — never a confident wrong answer. |

**Validated numbers (do not regress in CI fixtures):**

- spawn-lineage, conn-pool, mutex: **≥0.99** precision on GT replay (was 1.0).
- chan-work-handoff E1 lineage only: **~0.55** — expected; do not “fix” by naive cookie forward.
- chan E2 with elem id in GT: **1.0** — logic exists in `internal/attribution`; P1 does not require BPF sudog yet.
- Overhead: **&lt;5%** throughput regression at reference load; **0** ringbuf drops on benchmark scripts.

### 2. Kernel hot path (do not regress)

- Filter order: `targeted()` → preempt vs block (`prev_state == 0` → RUNNABLE, **not** BLOCKED) → `min_block` → sample → stack.
- **Never** block on ringbuf backpressure: drop + increment stat.
- Single time domain in-kernel: `bpf_ktime_get_ns`; wall clock for display/join only.
- Go: **entry uprobes only** — no uretprobes on Go binaries.
- `link.AttachTracing` for `tp_btf/*` (not RawTracepoint).

### 3. BPF gaps (P1 may prepare, P2 completes)

- `last_sudog_elem` / `futex_uaddr` are read into `event.aux` but **not written** in BPF today.
- P1 **must not** claim end-to-end Tier-2 chan accuracy in user-facing docs.
- Optional P1 prep: stub map hooks or TODO with tests skipped — **no half-shipped confident chan**.

### 4. Product tiering for P1 UX

- **Default analyze output:** Tier-0/1 — per-thread/tid wait-for chains, optional cookie when lineage exists.
- **Label clearly:** “ambiguous (shared resource)” for pool/mutex/chan without elem.
- **Export:** pprof values = `blocked_ns` on critical-path edges; locations = symbolized waker stack when available.

---

## Architecture targets (implement incrementally)

```
L5  CLI          record (harden) | analyze (NEW) | export (NEW)
L4  Analyzer     segments → wait-for graph → SCC → longest path → aggregate flamechart
L3  Attribution  integrate confidence into analyze path; Tier-0/1 default
L2  BPF          minor hardening only unless needed for P1; no large new probes
L1  symbolize    stack_id → []frame (build-id cache skeleton)
```

Keep packages **acyclic:** `loader` → `agent` → `trace` → `symbolize` → `attribution` → `analyzer` → `export` → `cmd/criticast`.

---

## Trace format v2 (charter Appendix P)

Upgrade from current JSONL-only capture to a structured on-disk format (can be JSONL sections initially if binary is too much for P1, but header **must** include):

```text
Header  { magic="CRTC", version, endianness, ktime_base_ns, wall_base_utc,
          tgid, min_block_ns, sample_mod, struct_event_version, bpf_stats_summary }
Section EVENTS   — ordered by ts_ns (existing event.Event, versioned)
Section STACKS   — stack_id → pcs (deduped); populate as symbolize input
Section FOOTER   — userspace + bpf drop counters
```

- Bump `internal/trace` version; migration note in commit if breaking.
- `record --out trace.criticast` (or keep `.jsonl` with magic line — prefer charter name).
- Backward compat: `analyze` reads v1 JSONL traces from P0 runs.

---

## CLI specification (implement)

### `criticast record` (harden existing)

```
criticast record --pid <tgid> [--dur 30s] [--min-block 1us|50us] [--sample N]
                 [--out trace.criticast] [--runtime go|none]
                 [--go-binary PATH] [--go-version go1.22.0]
                 [--bpf-object PATH]
```

- Require `--pid` or add `--cgroup` (P1 stretch: cgroup tgids from `/proc` — document if deferred).
- On exit: print summary (existing); exit non-zero on attach failure.
- Write trace v2 header + events + footer stats.

### `criticast analyze` (NEW — core P1)

```
criticast analyze <trace> [--request <cookie|tid>] [--top N]
                  [--min-confidence 0-100] [--format text|json]
```

**Behavior:**

1. Load trace + symbolize stacks (lazy OK).
2. Build segments per `task_id` (or `tid` if task_id missing — Tier-0 fallback).
3. Build wait-for graph; apply false-wakeup filter (ε configurable, default 10µs, charter E.4).
4. If `--request` set: compute **one** critical path (SCC collapse + DAG longest path, charter E.3/E.5).
5. If no `--request`: aggregate top edges across all cookies/tids (flamechart ranking).
6. **Text output** example:

```text
Request: req-abc (cookie=0x...)
Wall: 412ms  |  Path weight: 408ms  |  Ambiguous: 22ms (5%)

CRITICAL PATH (confidence ≥ 80):
  188ms  WC_NET      tid 42 ← tid 17   waker: epoll_wait  [stack...]
  142ms  WC_CHAN     tid 17 ← tid 9    ambiguous (no elem)  conf=45%
  ...

Dominant waits (all requests, top 10):
  ...
```

7. **JSON format:** stable schema for CI golden tests.

**Invariants (test):** For golden traces with known structure, `path_weight ≈ wall_clock ± slack`.

### `criticast export` (NEW)

```
criticast export <trace> --pprof out.pb.gz [--sample-index N]
```

- Map critical-path edges to pprof Profile (google/pprof proto or `github.com/google/pprof/profile`).
- Sample type: `critical_wait_ns` or `contentions`/`delay` — document choice; value = blocked time on path.
- Location: symbolized waker stack frames.
- **Do not** require OTLP in P1; optional `--otlp` returns “not implemented” with exit code 2.

### Deprecate / hide

- Keep `criticast eval` for **regression** (GT matrix); document as dev-only in `--help` or `criticast eval --help`.

---

## Package implementation checklist

### `internal/trace`

- [ ] Versioned header/footer; read v1 + v2.
- [ ] `EventWallTime` preserved for GT join (regression).
- [ ] Tests: round-trip, v1 compat.

### `internal/symbolize`

- [ ] `Resolver` interface: `Resolve(stackID) ([]Frame, error)`.
- [ ] P1 implementation: read stack PCs from trace STACKS section OR on-demand from BPF map dump if not in trace — minimum: parse `/proc/PID/maps` + elf symbols for target binary at record time (cache by build-id).
- [ ] Handle stripped binaries: return hex PC + build-id; no panic.
- [ ] Thread-safe cache (sync.Map or RWMutex).

### `internal/analyzer`

- [ ] `Segment` builder from `[]event.Event` (charter E.1).
- [ ] False-wakeup suppression (E.4).
- [ ] `BuildGraph` per cookie/tid.
- [ ] `SCC` + `LongestPath` on condensation (replace/adjust current Bellman-Ford-only path for production).
- [ ] `AggregateFlamechart` across requests (E.6).
- [ ] Table-driven tests + **golden file** `testdata/traces/golden_*.jsonl`.

### `internal/attribution`

- [ ] Bridge: given segment edge + optional GT, produce `confidence` + `ambiguous` flag.
- [ ] P1 analyze uses **E1 + E3** default (lineage + resource suppress); E2 only when `aux != 0`.
- [ ] Do not regress `RunExperiment` tests.

### `internal/export`

- [ ] `WritePprof(path, ProfileInput)` — critical path edges + symbolized stacks.
- [ ] Test: generate pprof, `go tool pprof -top` parses (smoke in CI if pprof installed).

### `cmd/criticast`

- [ ] `analyze.go`, `export.go`; wire in `main.go`.
- [ ] Shared `repoRoot`, duration parsing (reuse).
- [ ] Consistent error handling: wrap errors, exit 1 on user error, 2 on usage.

---

## Code quality bar (large-scale OSS)

### General

- Production-grade error handling; no panics on user input.
- Minimal diff; no drive-by refactors outside P1 scope.
- Names match charter: `wait_class`, `cookie`, `task_id`, `EV_BLOCK_END`, `confidence`.
- Comments only for non-obvious invariants (preempt bit, SCC merge, verifier limits).

### Go

- `errors` wrapping with context.
- `context.Context` on long analyze paths for cancellation.
- Bounded memory: stream events where possible; document peak memory for 1M events.
- `go test ./...` and `go vet ./...` pass.
- Table-driven tests; golden files for analyze output.
- No `init()` side effects; explicit constructors.

### Security & privacy

- Read-only observation; no target memory writes; no L7 payload inspection.
- Document capabilities: `CAP_BPF`, `CAP_PERFMON` — not `--privileged` in examples.
- Target scoping by tgid; document cross-tenant risk if cookie leaks (future cgroup scoping).
- No secrets in repo; no `.env` commits.

### Licenses

- `bpf/**` GPLv2 headers unchanged.
- Go Apache-2.0; no GPL code in Go files.

### CI (`.github/workflows/` — create if missing)

```yaml
# Minimum P1 CI
- checkout
- go test ./...
- go vet ./...
- on Linux runner with BTF:
    make bpf
    make test-bpf
```

Stretch: attribution regression job running `go test` with fixture + threshold check.

### Documentation updates (same branch)

- [README.md](../README.md): P1 quickstart with `analyze` + `export --pprof` example.
- [docs/ROADMAP.md](ROADMAP.md): mark P1 items done as shipped.
- [docs/GETTING_STARTED.md](GETTING_STARTED.md): new CLI section.
- Do not resurrect “Phase 0” gate language; use “validation baseline”.

---

## Implementation order (suggested sprints)

### Sprint 1 — Foundation (week 1)

1. Trace v2 read/write + backward compat.
2. `internal/symbolize` skeleton + unit tests (mock stack).
3. `analyze` skeleton: load trace → segments → text dump (no SCC yet).

### Sprint 2 — Analyzer core (week 2)

4. SCC + longest path + path_weight invariant tests.
5. False-wakeup filter + confidence from attribution package.
6. `analyze --format text|json` complete for single request.

### Sprint 3 — Export + UX (week 3)

7. `export --pprof`.
8. Aggregate flamechart (`--top N` without `--request`).
9. README demo script + example trace in `testdata/traces/`.

### Sprint 4 — Hardening (week 4)

10. CI workflow.
11. `record` flags polish; footer stats in trace.
12. Re-run overhead script; document “no regression” in `results/phase0/p0a-overhead.md` addendum or `results/p1-smoke.md`.
13. PR cleanup, `CHANGELOG.md`, tag `v0.1.0` (optional).

---

## Definition of done (Phase 1)

- [ ] Branch `phase1/shippable-core` merged via PR(s) with review.
- [ ] `criticast analyze trace.criticast` prints critical path for Go workload trace.
- [ ] `criticast export --pprof out.pb.gz` produces valid pprof viewable by `go tool pprof`.
- [ ] Tier-2 chan never shown as high-confidence without `aux`/elem.
- [ ] `./scripts/verify.sh` + Linux CI green.
- [ ] Golden analyze test passes; attribution regression tests pass.
- [ ] README shows end-to-end 5-minute demo.
- [ ] No scope creep: no k8s, no OTLP default, no live TUI.
- [ ] Charter registry items marked **IN** for P1 are implemented or explicitly documented if **PARTIAL**.
- [ ] README uses degradation-ladder language (Tier-0/1 default; no false Tier-2 claims).

---

## Agent prompt (copy-paste block)

```text
You are implementing Phase 1 of criticast on branch `phase1/shippable-core`.

Read first (in order):
1. CHARTER.md — §0.3, §0.5, Parts A, B (B.1–B.3,B.5), C (C.1–C.3.6), E, J, K, M, Appendix P, Q.2, R (full list in P1_IMPLEMENTATION_PROMPT.md § Charter decision registry)
2. AGENTS.md — all non-negotiable rules
3. docs/ROADMAP.md — validated learnings
4. docs/P1_IMPLEMENTATION_PROMPT.md — this spec (including charter registry table)

Context:
- Phase 0 validated: overhead OK (~1% loss, 0 ringbuf drops); attribution E1 spawn/pool/mutex 1.0; chan lineage ~0.55; E2 sudog logic 1.0 on GT replay; BPF does NOT set sudog.elem yet (trace-joined chan ~0.78).
- Phase 1 ships Tier-0/1 product: record + analyze + pprof export. NOT Tier-2 chan default. NOT k8s/OTLP/TUI.

Rules:
- Smallest correct diff; match existing package layout and naming.
- Lineage-first attribution; never naive waker-cookie propagation at mutex/pool/worker/chan.
- prev_state==0 is preempt/RUNNABLE not block.
- Filter before ringbuf_reserve; drops counted.
- Go uprobes entry-only.
- GPLv2 bpf/ vs Apache Go.

Your task for this session:
[USER: insert specific sprint item, e.g. "Implement criticast analyze with segments, SCC, longest path, text+json output, golden test"]

Before claiming done:
- go test ./...
- go vet ./...
- If BPF touched: make bpf && make test-bpf on Linux
- No panics on bad user input; wrap errors

Deliverables:
- Code on branch phase1/shippable-core
- Tests with golden or table-driven coverage
- Update docs/GETTING_STARTED.md and README if CLI surface changes
```

---

## Optional stretch (only if core DoD met early)

- `criticast record --cgroup` target filter.
- nginx workload script + demo trace (charter Part M).
- `--format flamechart` ASCII output.
- `criticast export --otlp` stub returning clear “P3” message.

Do **not** start stretch until Definition of done is satisfied.
