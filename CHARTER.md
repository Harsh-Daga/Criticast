# Project Charter v3 — `criticast`

**A generic, runtime-agnostic, eBPF per-request critical-path profiler.**

> Find the single wait that dominates your P99 — across threads, goroutines, and
> async hops — with zero code changes, in under five minutes.

- **Status:** Core collector and CLI implemented; see [docs/ROADMAP.md](docs/ROADMAP.md) for backlog.
- **License:** Apache-2.0 (userspace) · GPLv2 (BPF object files).
- **Lead name:** `criticast` · fallback `critipath`.
- **Charter version:** v3. v3 supersedes v2's *forward cookie propagation* model
  (which was wrong at shared resources) with **deterministic lineage-first
  attribution**. See [§0.3](#03-what-changed-from-v2--the-load-bearing-correction)
  and [Part C](#part-c--attribution-l3l4-the-tiered-model).

### Documentation (implementers)

| Document | Role |
|----------|------|
| [README.md](README.md) | Project entry |
| [AGENTS.md](AGENTS.md) | LLM + contributor engineering rules |
| [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) | Build, run, benchmarks |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Capabilities and engineering backlog |
| [docs/README.md](docs/README.md) | Documentation index |

---

## Table of Contents

- [Part 0 — Executive Summary & Thesis](#part-0--executive-summary--thesis)
- [Part A — Foundations: the physical model](#part-a--foundations-the-physical-model)
- [Part B — Kernel Collector (L2), line level](#part-b--kernel-collector-l2-line-level)
- [Part C — Attribution (L3+L4), the tiered model](#part-c--attribution-l3l4-the-tiered-model)
- [Part D — Userspace Agent (L1+L4): threading & flow](#part-d--userspace-agent-l1l4-threading--flow)
- [Part E — Analyzer Algorithm (L4): data structures + steps](#part-e--analyzer-algorithm-l4-data-structures--steps)
- [Part F — System Architecture & Layering](#part-f--system-architecture--layering)
- [Part G — Tech Stack (decided, justified)](#part-g--tech-stack-decided-justified)
- [Part H — Phase 0: two blocking gates](#part-h--phase-0-two-blocking-gates-kill-or-confirm)
- [Part I — Roadmap](#part-i--roadmap)
- [Part J — Deployment & Integration](#part-j--deployment--integration)
- [Part K — Edge Cases (enumerated, with handling)](#part-k--edge-cases-enumerated-with-handling)
- [Part L — Risks (brutal)](#part-l--risks-brutal)
- [Part M — Success / ROI / Naming](#part-m--success--roi--naming)
- [Appendix N — Verified facts & sources](#appendix-n--verified-facts--sources)
- [Appendix O — Glossary](#appendix-o--glossary)
- [Appendix P — Wire formats & schemas](#appendix-p--wire-formats--schemas)
- [Appendix Q — Operational artifacts](#appendix-q--operational-artifacts)
- [Appendix R — Threat model, resource budget, testing](#appendix-r--threat-model-resource-budget-testing)

---

## Part 0 — Executive Summary & Thesis

### 0.1 The problem

Distributed tracing tells you *which service* is slow. CPU profilers tell you
*which function* burns cycles. Neither tells you the thing operators actually
need at 3 a.m.: **on a single host, for a single slow request, what was the
chain of waits that produced the latency, and which one wait should I fix
first?**

Most P99 latency is not CPU time — it is *off-CPU* time: blocked on a lock,
parked on a channel, waiting on disk/network I/O, or starved of a CPU on the run
queue. Existing off-CPU tools (`offcputime`, `offwaketime`) are **aggregate**:
they bucket waits by stack across the whole process. They cannot answer "for
*this* request, what was the causal critical path?"

`criticast` answers exactly that, by reconstructing — from kernel scheduler
events alone — the **per-request wait-for graph** and extracting its **longest
weighted path** (the critical path).

### 0.2 The thesis (three claims)

1. **Causality is observable from the scheduler.** `sched_waking` records the
   exact instant one task unblocks another, and the waker is the currently
   running task. This directed `wakee → waker` edge, weighted by the blocked
   duration, is *literally* a causal wait-for edge. (Proven by wPerf, OSDI '18;
   implemented by `bcc/offwaketime`.)
2. **Request identity is mostly deterministic, not heuristic.** A request's
   entry point is known (the `accept`/`recv` on the server socket). Within a
   process, goroutine ancestry is *recorded by the runtime itself*
   (`g.parentGoid`). So most work attributes to its request by **walking the
   spawn tree**, with zero heuristics.
3. **The residual ambiguity is localized and classifiable.** The only place
   attribution is genuinely hard is **shared, long-lived resources** — worker
   pools, connection pools, shared queues/mutexes. The *kind* of wakeup
   (`gopark` `waitReason`, read from a register) tells us the correct
   propagation policy for each. We **suppress** false edges and **score
   confidence** rather than assert a wrong answer.

### 0.3 What changed from v2 — the load-bearing correction

v2 said: *"propagate the waker's cookie to the wakee across every wakeup, with a
TTL."* This is **forward cookie propagation**, and it is **exactly wrong at
shared resources.**

The bug: at a mutex unlock, a connection-pool release, or a worker-pool handoff,
the waker is running for request **C**, but the wakee it unblocks is *not* doing
request-C work — the wakee was just the unlucky waiter who got the connection
that C happened to release. Inheriting C's cookie fabricates a false edge.

Two concepts were fused and must be separated:

| Concept | Question | Reliability |
|---|---|---|
| **(i) Wait-for edge** | "B waited 212 ms; A unblocked it." | Almost always correct; directly observed via `sched_waking`. |
| **(ii) Request identity** | "B's work belongs to request R." | Fragile. Propagating it *forward from the waker* is the bug. |

**The corrected model is lineage-first, not waker-first.** Request identity is
carried by the **wakee's own lineage** (entry anchor + spawn tree), *not*
inherited from whoever happened to wake it. Forward propagation is used **only**
at shared-resource boundaries, **only** when the wakeup type permits it, and
**always** with a confidence score. See [Part C](#part-c--attribution-l3l4-the-tiered-model).

### 0.4 The two honest reminders (do not bury these)

1. **The overhead gate (Phase 0-A) is a genuine coin-flip, not a formality.**
   Scheduler-tracepoint cost at >1M events/s is exactly what kept Brendan
   Gregg's 2016 chain-graph prototype out of production. In-kernel aggregation +
   `min_block` filtering + sampling is the right answer *in theory*; it is
   **unproven on your hardware** until you run [Part H](#part-h--phase-0-two-blocking-gates-kill-or-confirm).
   Build that first; trust nothing else until it is green.
2. **Tier-2 attribution is the whole differentiator and the whole risk.**
   Tiers 0/1 are excellent engineering of *known* techniques and are worth
   shipping regardless. The "automatic per-request DAG across goroutines with
   zero instrumentation" headline lives entirely on Tier-2's accuracy surviving
   contact with real async services. **Design so a Tier-2 failure is a feature
   downgrade, never a project death.**

### 0.5 Graceful degradation ladder (the project cannot "fail," only narrow)

```
Full vision:    per-request cross-goroutine causal DAG + critical path   (needs Tier-2 ≥90%)
   ↓ degrade
Lineage-only:   deterministic spawn-tree attribution; pools/locks labeled ambiguous
   ↓ degrade
Tier-1:         single-thread/single-goroutine request spans
   ↓ degrade
Tier-0:         per-process causal critical-wait chains (still beats offwaketime)
   ↓ degrade
Sampled-only:   continuous low-overhead off-CPU + wait-class breakdown
```

Each rung is a shippable, novel-or-best-in-class OSS tool. The gates in
[Part H](#part-h--phase-0-two-blocking-gates-kill-or-confirm) decide which rung
we land on — *measured*, not assumed.

---

## Part A — Foundations: the physical model

### A.1 What we actually observe

A thread of execution is always in exactly one of these states. **We track
transitions, not states** — every latency number is a difference between two
timestamps of two transition events.

```
            sched_waking (someone wakes me)
   BLOCKED ──────────────────────────────►  RUNNABLE
      ▲                                          │
      │ sched_switch                             │ sched_switch
      │ (prev_state != 0)                        │ (I'm picked: next_pid == me)
      │                                          ▼
   (I block) ◄──────────────────────────────  RUNNING
                  sched_switch
                  (prev_state == 0 → preempted, stays RUNNABLE)
```

The two measurable gaps that *are* latency:

- **BLOCKED duration** = `t(sched_waking) − t(sched_switch-out)`: real off-CPU
  wait (lock, I/O, channel, sleep). **The causal edge is born here:** the task
  running on the waker CPU at `sched_waking` is who unblocked me.
- **RUNNABLE duration** = `t(sched_switch-in) − t(sched_waking)`: run-queue /
  scheduler latency (CPU starvation). A **first-class wait class**, attributed
  to "the scheduler / CPU contention," *not* to another task.

A third, implicit gap matters for the critical path:

- **RUNNING (on-CPU) duration** = time between coming on-CPU and going off again:
  actual compute. It becomes **node weight** in the critical-path graph.

### A.2 Verified kernel facts (the load-bearing details)

**`sched_switch`** — `include/trace/events/sched.h`:

```c
TP_PROTO(bool preempt, struct task_struct *prev, struct task_struct *next,
         unsigned int prev_state)
// raw tracepoint ctx: args[0]=preempt, args[1]=prev, args[2]=next, args[3]=prev_state
```

The critical subtlety, straight from `__trace_sched_switch_state()`: if
`preempt==true` the task is reported **RUNNING** regardless of its state (it
*wanted* to keep running). Therefore:

- `prev_state == 0` (`TASK_RUNNING`) → **preempted** → goes to **RUNNABLE**.
  This is **run-queue latency**, not a block.
- `prev_state & (TASK_INTERRUPTIBLE | TASK_UNINTERRUPTIBLE)` (`1`="S", `2`="D")
  → **voluntarily blocked** (lock / I/O / sleep). **D** (uninterruptible)
  almost always = disk/I/O; **S** = lock / condvar / sleep / netpoll.

> This single bit-test is how we distinguish "starved of CPU" from "waiting on a
> dependency." **Getting it wrong corrupts every downstream number** — it is the
> single most important classification in the system. See
> [Part K, edge #1](#part-k--edge-cases-enumerated-with-handling).

The exact `prev_state` decoding (kernel folds in `preempt` before delivery on
modern kernels via `TASK_REPORT`):

| `prev_state` value | report char | meaning | our class |
|---|---|---|---|
| `0x0000` | `R` | running/preempted | RUNNABLE (runq latency) |
| `0x0001` | `S` | interruptible sleep | BLOCKED (lock/sleep/net) |
| `0x0002` | `D` | uninterruptible sleep | BLOCKED (disk/IO) |
| `0x0004`+ | `T/t/X/Z` | stopped/traced/dead/zombie | terminal — close spans |

**`sched_waking`** — `TP_PROTO(struct task_struct *p)`, `args[0] = p` is the
**wakee**. The **waker** is the currently-running task:
`bpf_get_current_pid_tgid()` inside the handler. This is precisely how
`bcc/offwaketime` builds waker→wakee edges (its `waker()` sets
`woke.w_pid = bpf_get_current_pid_tgid()`).

We prefer `sched_waking` over `sched_wakeup` (Perfetto guidance): same
information for latency, fires on the **waker** CPU (so `bpf_get_current_*` truly
identifies the waker), and avoids IPI-path ambiguity where `sched_wakeup` can
fire on the target CPU.

**Why this is enough for causality:** at the instant `T_waker` wakes
`T_wakee`, we record the directed edge `T_wakee → T_waker` ("wakee waited for
waker") with the blocked duration as weight. wPerf (OSDI '18) proves this
thread-level wait-for edge set is sufficient to find bottlenecks; we additionally
**scope it per-request** and compute **longest-path** instead of knots.

### A.3 The hard truth restated

The novelty and risk both live in **intra-process async attribution** (Tier 2,
[§C.3](#c3-tier-2--intra-process-async-attribution-corrected-lineage-first)).
Tiers 0/1 are "assemble proven techniques excellently." If Tier 2's
attribution is too noisy on real workloads, we degrade to a per-process causal
critical-path tool (still best-in-class OSS) without project failure. **Phase
0-A gates overhead; Phase 0-B gates attribution accuracy.** Neither is assumed.

---

## Part B — Kernel Collector (L2), line level

The collector is C + libbpf + CO-RE, attached via BTF raw tracepoints
(`tp_btf/*`) where available, with graceful fallback to `raw_tracepoint/*` and
classic `tracepoint/sched/*` for older kernels (probe tier selected at load by
L1; see [§D.1](#d1-processthreading-model-go--ciliumebpf)).

### B.1 Maps

```c
// vmlinux.h from BTF; CO-RE throughout.

// (1) Per-thread wait state. Task-local storage: auto-freed on task exit,
//     no GC, keyed by task_struct. Requires BPF_F_NO_PREALLOC + BTF.
struct wait_state {
    __u64 last_switch_out_ns;  // when this tid went off-CPU
    __u64 waking_ns;           // when it was woken (BLOCKED->RUNNABLE)
    __u32 waker_tid;           // who woke it (from sched_waking)
    __u32 waker_stack_id;      // waker's stack at wake instant
    __u8  prev_state;          // to classify block vs preempt
    __u8  wait_class;          // FUTEX/EPOLL/IO_URING/NET/DISK/RUNQ/UNKNOWN
    __u8  _pad[2];
    __u64 cookie;              // attribution token carried by this thread
    __u64 cookie_expire_ns;    // TTL for forward propagation at shared resources
    __u64 task_id;             // goid / tokio task id (0 => use tid)
    __u64 futex_uaddr;         // lock identity for WC_FUTEX refinement
    __u64 last_sudog_elem;     // channel element ptr (work-handoff correlation)
};
struct {
    __uint(type, BPF_MAP_TYPE_TASK_STORAGE);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, int);
    __type(value, struct wait_state);
} thread_state SEC(".maps");

// (2) Dedup'd stacks, referenced by id (the offwaketime pattern — never copy
//     a full stack per event).
struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(max_entries, 65536);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, 127 * sizeof(__u64)); // PERF_MAX_STACK_DEPTH
} stacks SEC(".maps");

// (3) Output. Ring buffer (not perf buffer): lower overhead, MPSC-ordered,
//     reserve/submit avoids a copy. 5.8+.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 16 << 20); // 16 MiB, tunable per-CPU-scaled
} events SEC(".maps");

// (4) Target filter: which tgids to trace. Checked at probe head.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u32);   // tgid
    __type(value, __u8);
} targets SEC(".maps");

// (4b) Parallel cgroup-id filter (k8s pod scoping).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u64);   // cgroup id
    __type(value, __u8);
} target_cgroups SEC(".maps");

// (5) Runtime config (single entry): sampling, min_block_ns, flags.
struct config { __u64 min_block_ns; __u32 sample_mod; __u32 flags; __u64 cookie_ttl_ns; };
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32); __type(value, struct config);
} cfg SEC(".maps");

// (6) Drop counter (observability of the observer) + per-reason stats.
struct { __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY); __uint(max_entries, 8);
         __type(key, __u32); __type(value, __u64); } stats SEC(".maps");
// stat indices: 0=ringbuf_drops 1=events_emitted 2=blocks_seen 3=preempts
//               4=runq_closed 5=short_filtered 6=sampled_out 7=stack_fail

// (7) io_uring SQE->CQE bridge (async I/O not visible via syscalls).
struct io_key { __u64 ring_ptr; __u64 user_data; };
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 16384);
    __type(key, struct io_key); __type(value, __u64 /*submit_ts*/);
} io_inflight SEC(".maps");

// (8) tid -> task_id (goid) lifted by L3 runtime resolver uprobes.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1 << 20);
    __type(key, __u32 /*tid*/); __type(value, __u64 /*task_id*/);
} tid_to_task SEC(".maps");
```

**Why each map type:**

- **Task storage** for `wait_state`: per-task lifetime tied to the kernel
  `task_struct`, freed automatically on exit — no userspace GC, no stale-tid
  reuse bug, O(1) access via the task pointer we already hold.
- **Stack trace map** for dedup: a stack walk is the #2 cost center; storing each
  stack once and referencing it by `stack_id` is the `offwaketime` pattern.
- **Ring buffer** (not perf buffer): single MPSC ring, total ordering across
  CPUs, `reserve()/submit()` writes directly into the ring (no bounce buffer),
  epoll-notify to userspace. Requires kernel ≥ 5.8.
- **LRU hash** for io_uring: in-flight SQEs self-evict under pressure; never
  blocks the probe.

### B.2 Event record (the L2→L4 contract — fixed layout, no pointers)

```c
enum ev_type : __u8 {
    EV_BLOCK_BEGIN, EV_BLOCK_END, EV_RUNQ, EV_SYSCALL_BOUNDARY,
    EV_IO_SUBMIT, EV_IO_COMPLETE, EV_TASK_STATE /*Go/Tokio*/,
    EV_SPAN_OPEN, EV_SPAN_CLOSE, EV_SPAWN /*lineage: parent->child*/
};
enum wait_class : __u8 {
    WC_UNKNOWN, WC_FUTEX, WC_EPOLL, WC_IO_URING,
    WC_NET, WC_DISK, WC_RUNQ, WC_SLEEP, WC_GC,
    WC_CHAN, WC_MUTEX, WC_SELECT, WC_SEMA, WC_COND
};
struct event {
    __u64 ts_ns;            // bpf_ktime_get_ns — single clock domain, total order
    __u32 cpu;
    __u32 tgid;
    __u32 tid;              // subject
    __u32 waker_tid;        // EV_BLOCK_END only
    __u64 blocked_ns;       // EV_BLOCK_END: measured wait length
    __s32 stack_id;         // subject user+kernel stack (-1 none)
    __s32 waker_stack_id;   // waker stack (-1)
    __u64 cookie;           // attribution token (lineage-derived)
    __u64 task_id;          // goid/tokio id; 0 => use tid
    __u64 waker_task_id;    // waker goid (for intra-process edges)
    __u64 aux;              // wait-class-specific: futex uaddr / sudog.elem / fd
    __u8  type;
    __u8  wait_class;
    __u8  prev_state;
    __u8  confidence;       // 0..100 attribution confidence (set in userspace too)
};                          // 80 bytes; 16-byte aligned, two cache lines
```

> The record is **fixed-layout and pointer-free** so it can be copied verbatim
> from the ring buffer and decoded with a single struct cast in Go (`bpf2go`
> generates the matching type). All semantic enrichment (symbolization, goid
> naming, confidence refinement) happens in userspace; the kernel emits only
> raw, cheap facts.

### B.3 The in-kernel state machine (the heart)

```c
static __always_inline bool targeted(__u32 tgid, __u64 cgid) {
    if (bpf_map_lookup_elem(&targets, &tgid)) return true;
    return bpf_map_lookup_elem(&target_cgroups, &cgid) != NULL;
}

SEC("tp_btf/sched_switch")   // BTF raw tracepoint: typed args, lowest overhead
int handle_switch(u64 *ctx) {
    struct task_struct *prev = (void *)ctx[1];
    struct task_struct *next = (void *)ctx[2];
    __u32 prev_state = (__u32)ctx[3];
    __u64 now = bpf_ktime_get_ns();

    // --- prev going off-CPU ---
    __u32 prev_tgid = BPF_CORE_READ(prev, tgid);
    if (targeted(prev_tgid, /*cgid*/0)) {
        struct wait_state *ws = bpf_task_storage_get(&thread_state, prev, NULL,
                                    BPF_LOCAL_STORAGE_GET_F_CREATE);
        if (ws) {
            ws->last_switch_out_ns = now;
            ws->prev_state = prev_state;     // 0 => preempted(RUNNABLE), else BLOCKED
            // optional EV_BLOCK_BEGIN emission gated by flags (analyzer can also
            // infer begin from the matching end; default: no begin event).
        }
    }

    // --- next coming on-CPU: close a RUNNABLE (runq-latency) segment ---
    __u32 next_tgid = BPF_CORE_READ(next, tgid);
    if (targeted(next_tgid, 0)) {
        struct wait_state *ws = bpf_task_storage_get(&thread_state, next, NULL, 0);
        if (ws && ws->waking_ns) {
            __u64 runq = now - ws->waking_ns;       // scheduler latency
            emit_runq(ctx, next, runq);             // EV_RUNQ (subject starved of CPU)
            ws->waking_ns = 0;
        }
    }
    return 0;
}

SEC("tp_btf/sched_waking")
int handle_waking(u64 *ctx) {
    struct task_struct *wakee = (void *)ctx[0];
    __u32 wakee_tgid = BPF_CORE_READ(wakee, tgid);
    if (!targeted(wakee_tgid, 0)) return 0;

    struct wait_state *ws = bpf_task_storage_get(&thread_state, wakee, NULL, 0);
    if (!ws || ws->prev_state == 0) return 0;     // was preempted, not blocked → skip

    __u64 now = bpf_ktime_get_ns();
    __u64 blocked = now - ws->last_switch_out_ns;
    struct config *c = cfg_get();
    if (blocked < c->min_block_ns) return 0;       // OVERHEAD LEVER #1: filter short

    // The waker is *us* (currently running task).
    __u64 waker_pidtgid = bpf_get_current_pid_tgid();
    __u32 waker_tid = (__u32)waker_pidtgid;
    ws->waking_ns = now;
    ws->waker_tid = waker_tid;

    // OVERHEAD LEVER #2: sampling (hash- or cookie-seeded so a sampled-in
    // request is sampled consistently across all its edges).
    if (c->sample_mod > 1 &&
        (bpf_get_prandom_u32() % c->sample_mod) != 0) return 0;

    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) { inc_stat(STAT_DROPS); return 0; }
    e->ts_ns = now; e->type = EV_BLOCK_END;
    e->cpu = bpf_get_smp_processor_id();
    e->tgid = wakee_tgid; e->tid = BPF_CORE_READ(wakee, pid);
    e->waker_tid = waker_tid; e->blocked_ns = blocked;
    e->prev_state = ws->prev_state;
    e->wait_class = ws->wait_class ? ws->wait_class : classify(ws->prev_state);
    e->aux = ws->futex_uaddr ? ws->futex_uaddr : ws->last_sudog_elem;
    // stacks: capture WAKER stack here (cheap, we're on-CPU as the waker).
    // Subject's blocked stack is captured at the blocking syscall/gopark probe,
    // where it is actually meaningful — NOT in the scheduler hot path.
    e->waker_stack_id = bpf_get_stackid(ctx, &stacks, BPF_F_USER_STACK);
    e->stack_id = -1;
    e->cookie = ws->cookie;
    e->task_id = lookup_task_id(e->tid);
    e->waker_task_id = lookup_task_id(waker_tid);
    bpf_ringbuf_submit(e, 0);
    inc_stat(STAT_EMITTED);
    return 0;
}
```

**Design notes that matter:**

- **`tp_btf/*` (BTF-typed raw tracepoints)** over classic `tracepoint/sched/*`:
  no per-arg copy into a format buffer, typed access, lowest-overhead path on
  modern kernels (≥ 5.5). Fallback to `raw_tracepoint`, then classic, selected
  at load by L1 based on detected kernel features.
- **Stack capture is gated:** only after `min_block_ns` and sampling pass. Stack
  walks are the #2 cost center (`offwaketime` explicitly warns). The subject's
  blocked stack is captured at the blocking syscall / `gopark` probe (where it
  is meaningful), not in the scheduler hot path.
- **`bpf_task_storage_get` with `F_CREATE` only on the switch-out path;**
  lookups elsewhere are non-creating to avoid allocating state for threads we
  never block-track.
- **Everything that can be filtered is filtered before `ringbuf_reserve`.** The
  reserve is the most expensive single op in the fast path.
- **Cookie is read, never written, in the scheduler hot path** in the
  lineage-first model — it is set by L3 resolvers / span probes
  (see [Part C](#part-c--attribution-l3l4-the-tiered-model)), so the hot path
  stays a pure read.

### B.4 Wait-class refinement probes (attach-on-demand)

Scheduler events tell us *that* a thread blocked, not *on what*. We refine with
cheap, targeted probes whose data is folded into `wait_state` and surfaces on the
next `EV_BLOCK_END`:

| Probe | Purpose | Cost control |
|---|---|---|
| `tp/syscalls/sys_enter_futex` | mark `WC_FUTEX` + futex `uaddr` (lock identity) | only sets fields in task storage |
| `tp/syscalls/sys_enter_epoll_wait` | mark `WC_EPOLL` | field set |
| `tp_btf/io_uring_submit_req` + `tp_btf/io_uring_complete` | bridge SQE→CQE for async I/O that bypasses syscalls | keyed by `(ring,user_data)` in LRU map (7) |
| `tp/syscalls/sys_enter_{accept,accept4,recvfrom,read}` on listen fd | Tier-1 request span open | only for target tgids |
| `tp/syscalls/sys_exit_{sendmsg,sendto,write}` | request span close | " |
| `tp/sched/sched_process_fork` / clone | thread lineage | feeds spawn tree for non-Go |
| `kprobe/finish_task_switch` (fallback) | older kernels without `tp_btf` | classic path |

The refinement probes write **only a field** into the existing task-storage
entry; they never reserve ring-buffer space themselves. The class is read on the
next `EV_BLOCK_END`, so refinement adds at most a couple of map writes per
syscall, not a per-event cost.

### B.5 Overhead budget (hard, measured in Phase 0-A)

| Mode | Throughput loss | P99 inflation | Mechanism |
|---|---|---|---|
| `record` full | < 5% | < 10% | `min_block` 1µs, in-kernel state, gated stacks |
| `record` min-block 50µs | < 2% | < 5% | filter short waits |
| continuous sampled 1/100 | < 1% | < 2% | sampling + min-block |

If full `record` cannot meet `<5%/<10%` on the Phase-0 rig, **stop** (or ship the
sampled rung). `offwaketime` confirms >1M sched events/s is real; in-kernel
aggregation is the only known answer, and "known answer" ≠ "proven on your
hardware" until measured.

**Overhead levers, ranked by impact:**

1. `min_block_ns` filter (drops the vast majority of sub-µs context switches
   before any work).
2. Sampling (`sample_mod`), cookie-seeded for consistency within a request.
3. Stack-walk gating (the single most expensive op after `ringbuf_reserve`).
4. `tp_btf` over classic tracepoints (removes per-arg format copy).
5. Per-CPU ring sizing + batched userspace drain (amortize epoll wakeups).

---

## Part C — Attribution (L3+L4), the tiered model

This is where v3 fundamentally diverges from v2. **Read [§0.3](#03-what-changed-from-v2--the-load-bearing-correction)
first.** The model is **lineage-first**: a goroutine/thread knows its own request
identity deterministically; we only fall back to (carefully policed) forward
propagation at shared resources.

### C.1 Tier 0 — process / time-window scoping (always works)

Scope = all `EV_BLOCK_END` / `EV_RUNQ` for a target tgid in `[t0, t1]`,
optionally top-N by `blocked_ns`. No request identity. Output: **per-process
causal critical-wait chains**. Phase 1 ships exactly this and already beats
`offwaketime` (which is aggregate-only — it cannot show a causal *chain*).

### C.2 Tier 1 — single-thread request span (proven: Pixie / DeepFlow)

```
sys_enter_accept4 / recvfrom on server fd  → open span: key (tid, fd),
                                             cookie = hash(tid, fd, ts_ns)
   ... all EV_BLOCK_END for tid inherit this cookie (set ws->cookie) ...
sys_exit_sendmsg / write on same fd        → close span, emit EV_SPAN_CLOSE
```

Covers **thread-per-request** (C/C++/classic Java, blocking servers) and **Go
handlers doing sequential work on one goroutine**. Solid, low-risk. The cookie
here is anchored to a real socket-level request boundary, so it is a *true*
request identity, not a heuristic.

### C.3 Tier 2 — intra-process async attribution (corrected: lineage-first)

**The key realization:** most goroutines already know their own request identity
deterministically — you never need to inherit it from a waker.

#### C.3.1 Deterministic lineage (the floor, expected ~100% precision)

- **Anchor:** request A's entry goroutine is known from Tier-1
  (`accept`/`recv` on the socket).
- **Spawn lineage is deterministic and free.** Go's `g` struct carries
  `parentGoid` (verified in `runtime2.go`:
  `parentGoid uint64 // goid of goroutine that created this goroutine`). So every
  goroutine A spawns (`go func()`) is attributable to A by **walking the spawn
  tree** — zero heuristic, zero ambiguity.
- Mechanism: a uprobe on `runtime.newproc1` (or the spawn path) records
  `EV_SPAWN{parent_goid, child_goid}`; the analyzer assigns the child the
  parent's cookie. Equivalently, read `parentGoid` lazily when a new goid is
  first seen by the `casgstatus` resolver.

So the ambiguous surface is **not** "every wakeup." It is specifically:
**long-lived goroutines with no lineage to any single request** — worker pools,
connection pools, shared queues. Those are the only places forward attribution
is needed, and exactly where it is hard. The v2 critique localized the entire
problem correctly.

#### C.3.2 Shared-resource boundaries: policy is keyed by wakeup type

In Go, `gopark`'s `waitReason` (verified, read from register RCX/PARM3)
classifies the wakeup *for free*. The wakeup type dictates the propagation
policy:

| `waitReason` at the wake | Edge type | Policy |
|---|---|---|
| `waitReasonChanReceive` / `waitReasonChanSend` carrying a work item | **work handoff** | propagate — **only if** we can match the item (see C.3.3) |
| `waitReasonSyncMutexLock`, `waitReasonSyncSemacquire`, RWMutex | **resource handoff** | **do NOT propagate.** Wakee keeps its own lineage cookie. Record as a *contention edge* on the lock/sema address. |
| connection pool (channel-of-connections) | **resource handoff disguised as a channel** | the irreducibly hard case (C.3.4) |
| `waitReasonSelect`, broadcast, chan-close waking N | **ambiguous** | low confidence; suppress propagation; mark `request-ambiguous`. |
| netpoll / `waitReasonIOWait` | **external I/O** | cookie comes from the **connection (4-tuple)**, not the netpoller goroutine. |

This **eliminates the mutex and semaphore false edges entirely**: the waiter
keeps its own identity; we only annotate "waited *T* ms on contended lock X." The
**wait-for edge (critical path) is preserved**; the **request identity is not
corrupted**.

#### C.3.3 Work-handoff via `sudog.elem` correlation (an experiment, not a given)

For a channel that carries *work items*, we can sometimes match an enqueued item
to the dequeuing goroutine by correlating the channel **element pointer**.
Go's `g.waiting` points to `sudog` structures, and `sudog.elem` is the data
element pointer for the blocked send/recv. If the sender's `sudog.elem` (item
written) matches the receiver's `sudog.elem` (item read) for the same channel,
that is a real work handoff and we propagate the sender's cookie to the receiver
**for that one work segment only**.

> **Honest caveat (P0-B tests this):** `sudog.elem` correlation depends on
> reading channel internals stably across Go versions — the same offset-drift tax
> as `goid`. It may recover worker-pool attribution, or it may just add
> complexity for marginal gain. **We do not assume it works; P0-B measures it.**

#### C.3.4 The irreducible case: channel-of-connections vs channel-of-work

A channel-of-connections pool and a channel-of-work-items look **identical** at
the kernel and even at `waitReason` (`waitReasonChanReceive` for both). We cannot
tell "this channel carries work" from "this channel carries resources" without
app context. **That is real and permanent.** Honest answers:

1. **Item correlation (C.3.3):** if `sudog.elem` matches enqueue↔dequeue, treat
   as work handoff.
2. **Cookie-entropy detector:** a node woken by *many different* request cookies
   over a window is a **shared resource**. Mark its edges `request-ambiguous`
   with a confidence derived from entropy, rather than asserting a wrong answer.
3. **Honest UI:** "this 180 ms wait was on a shared pool; request attribution
   unavailable." Never a confident wrong answer.

P0-B quantifies **how much of real critical-path time falls into this
unattributable bucket**. If it is large for common frameworks (`database/sql`
pools, etc.), that materially caps the product's value — and we deserve to know
the number *before* building.

#### C.3.5 The forward-propagation primitive (now bounded & policed)

Forward propagation still exists, but only at shared-resource work-handoffs that
pass the policy gate, and always TTL-bounded and confidence-scored:

```
on sched_waking(wakee=B, waker=A):
    wsA = state(A); wsB = state(B)
    policy = policy_for(wait_reason(B))          # from gopark waitReason
    if policy == PROPAGATE and item_matches(A, B) and
       wsA.cookie != 0 and now < wsA.cookie_expire_ns:
        wsB.cookie            = wsA.cookie        # only for next work segment
        wsB.cookie_expire_ns  = now + COOKIE_TTL  # bounded spread
        record edge (B inherits-from A, cookie=R, confidence=f(policy, entropy))
    else:
        # B keeps its own lineage cookie; if a wait-for edge exists, record it
        # as a contention/ambiguous edge WITHOUT changing B's identity.
        record contention_edge(B waited on resource X, conf=...)
```

**Risk mitigations, each measurable:**

- **TTL/decay (`COOKIE_TTL`)** bounds how far a possibly-wrong cookie spreads.
- **False-wakeup suppression** (wPerf's lesson): if B re-blocks within ε without
  doing work, drop the edge.
- **Confidence scoring per edge:** futex/mutex single-waiter handoff = high;
  broadcast wake = low; netpoll wake = medium. Surface confidence in output;
  allow threshold filtering.
- **Wait-class gating:** propagate across matched channel work-handoffs (high
  signal), never across mutex/sema/generic sleep/timer wakes.
- **Entropy gating:** shared-resource nodes are detected and demoted to
  ambiguous automatically.

#### C.3.6 Confidence scoring & the entropy detector (concrete)

Every attributed edge carries an integer **confidence** `0..100`. It is a product
of independent factors, each in `[0,1]`, scaled to 100:

```
confidence = round(100 * w_mech * w_entropy * w_waiters * w_timing)
```

| Factor | Definition | Values |
|---|---|---|
| `w_mech` | mechanism prior from the wakeup type | spawn-lineage 1.0 · matched chan work-handoff 0.9 · netpoll(4-tuple) 0.7 · select 0.4 · broadcast 0.2 · mutex/sema (suppressed → not propagated) |
| `w_entropy` | `1 − H_norm(node)` where `H_norm` is normalized cookie entropy at the waking node (see below) | 1.0 (single-request node) … 0.0 (uniformly shared) |
| `w_waiters` | `1/n` for a wake that released `n` waiters in the same ε window (thundering herd) | 1.0 single waiter … →0 herd |
| `w_timing` | penalize implausibly long idle between item-enqueue and dequeue | 1.0 immediate … decays past `COOKIE_TTL` |

**Cookie entropy at a node** (the shared-resource detector): maintain, per
candidate shared node `v` (a long-lived goroutine/worker), a sliding-window
multiset of the request cookies that have woken it. Shannon entropy

```
H(v) = − Σ_r p_r · log2(p_r),   p_r = count_r / Σ counts
H_norm(v) = H(v) / log2(K)      # K = distinct cookies in window
```

`H_norm → 1` means "woken roughly uniformly by many requests" = a shared resource
→ demote all of `v`'s outbound identity edges to `request-ambiguous`. `H_norm → 0`
means "almost always the same request" = safe to attribute. **P0-B's
entropy-detector AUC** measures whether `H_norm` predicts ground-truth ambiguity.

The userspace analyzer (not the kernel) computes entropy and final confidence;
the kernel only stamps the raw mechanism and `aux` pointer.

### C.4 Tier 3 — cross-process / host (explicitly best-effort)

Socket 4-tuple + TCP seq correlation (Pixie `ConnTracker`), opportunistic parse
of `traceparent` / `X-Request-ID` / gRPC metadata **if present**. Never injected,
never required. Optional **inbound** bridge: ingest existing OTel span context to
*seed* cookies → we become an **APM complement, not a competitor.** Coverage
advertised as partial (DeepFlow's honest ceiling).

**Mechanics (when enabled):**

1. **Connection identity.** On `tcp_sendmsg`/`tcp_recvmsg` (or
   `sock_sendmsg`/`sock_recvmsg`) kprobes, read the 4-tuple
   `(saddr, sport, daddr, dport)` from the `struct sock`. This keys a connection.
2. **Sequence bridging.** Record `(seq, len)` on send and `(ack_seq)` on the peer
   receive; the byte-range overlap stitches "this client's request N" to "this
   server's accept" *without payload inspection* — only header timing/sequence.
3. **Cookie seeding.** If an inbound request carries `traceparent`, its
   `trace_id`/`span_id` becomes the cookie root so `criticast` spans nest under
   the existing distributed trace. Absent that, the 4-tuple+seq is the join key.
4. **Honesty ceiling.** NAT, connection pooling at the L7 proxy, HTTP/2 stream
   multiplexing, and TLS all erode this. Coverage is advertised as **partial**;
   missing joins are shown as a boundary node "→ external (unjoined)," never
   guessed.

### C.5 Runtime identity resolvers (L3 plugins)

**`none` (default):** `task_id = tid`. C/C++ / sync Rust / classic Java. Zero
extra probes.

**`go`:**

```c
// uprobe on runtime.casgstatus — the choke point for scheduler transitions.
// regabi amd64: gp=RAX(ax), newval=RCX(cx). Current g in R14 (amd64)/R28 (arm64).
SEC("uprobe//proc/self/exe:runtime.casgstatus")
int up_casgstatus(struct pt_regs *ctx) {
    void *gp = (void*)PT_REGS_PARM1(ctx);     // ax
    __u32 newval = (__u32)PT_REGS_PARM3(ctx); // cx
    __u64 goid;
    bpf_probe_read_user(&goid, 8, gp + GOID_OFF); // GOID_OFF resolved per-binary
    __u32 tid = (__u32)bpf_get_current_pid_tgid();
    bpf_map_update_elem(&tid_to_task, &tid, &goid, BPF_ANY);
    return 0;
}
// uprobe on runtime.gopark: reason = waitReason in PARM3 (RCX). Gives wait-class:
//   waitReasonChanReceive/Send, waitReasonSelect, waitReasonSyncMutexLock,
//   waitReasonGCAssistMarking, etc.  Turns "blocked 200ms" -> "chan recv 200ms".
// uprobe on runtime.newproc1 (or go-spawn path): read parentGoid -> EV_SPAWN.
```

- **`GOID_OFF` resolution (no hardcoding):** (1) read DWARF
  `AttrDataMemberLoc` for `runtime.g.goid` if symbols present; (2) else embedded
  `offsets.json` keyed by `runtime.buildVersion` from `.go.buildinfo`; CI-tested
  against every Go release. (Verified: `goid`/`parentGoid` are stable *fields*
  but their *offsets move* across versions — tutorials' `0x98` is
  version-specific.)
- **`gopark` signature drift:** 1.19–1.20 `(unlockf, lock, reason, traceEv byte,
  traceskip)`; 1.21+ `(…, reason, traceReason traceBlockReason, traceskip)`.
  `reason` stays the 3rd arg (RCX) → robust. We still version-guard.
- **uretprobe ban:** Go's movable stacks corrupt uretprobe return-address
  hijacking (bcc #1320, cilium/ebpf #759). **Entry uprobes only;** for exit
  timing, place plain uprobes at RET offsets (go-bpf-gen technique). **No
  exceptions.**

**`tokio` (opt-in, near-zero instrumentation):** attach eBPF USDT to
`tokio-dtrace` probes (`task-poll-start/end`, `spawn`, `terminate`, `arg0` =
task `Id`); requires `tokio_unstable` + a one-line `register_hooks(&mut builder)`.
Honest: "one crate, one line," **not** zero. Task-id reuse disambiguated by
`(spawn_ts, id)`.

---

## Part D — Userspace Agent (L1+L4): threading & flow

### D.1 Process/threading model (Go + cilium/ebpf)

```
main
 ├─ loader goroutine: CO-RE load, BTF resolve, probe attach by kernel-feature tier
 ├─ target-watcher goroutine: discover pids/cgroups (k8s informer / proc scan),
 │     update `targets`/`target_cgroups` maps; autodetect runtime
 │     (read .go.buildinfo, ELF notes)
 ├─ ringbuf reader goroutine(s): drain `events`; one reader, N decode workers,
 │     backpressure via bounded channel; count drops vs `stats` map
 ├─ resolver goroutines: maintain tid->task_id maps from L3 plugin streams
 ├─ symbolizer pool: async stack_id -> frames (DWARF/pclntab/BTF/JIT), LRU cache
 └─ analyzer: segment builder -> wait-for graph -> per-request critical path
```

**Lock-free where it counts** (per-CPU decode, per-shard graph merge — wPerf's
parallel-analysis pattern). **The ring drain must never block the kernel** → on
overflow we drop + count, never stall. Adaptive backpressure raises `min_block_ns`
via the `cfg` map when drop rate climbs, instead of stalling the probe.

### D.2 Symbolization pipeline (per-runtime)

| Target | Unwinding | Symbolization |
|---|---|---|
| C/C++/Rust w/ frame pointers | `bpf_get_stackid(BPF_F_USER_STACK)` | ELF symtab + DWARF (`.debug_info`/`.eh_frame`); `addr2line`-equivalent in-process |
| `-fomit-frame-pointer` builds | DWARF CFI / `.eh_frame` unwinding **in userspace** (don't unwind DWARF in BPF) | as above |
| Go | FP unwinding (default ≥ 1.21, amd64/arm64 — Datadog's <1% contribution) | `debug/gosym` + pclntab; goid from resolver |
| JVM | FP (`-XX:+PreserveFramePointer`) | perf-map-agent / JFR symbol map (wPerf used perf-map-agent) |
| containers | resolve build-id, fetch debuginfo (debuginfod) | build-id–keyed symbol cache |

**Symbol cache keyed by `(build_id, file_offset)`** so it survives ASLR and is
shareable across pods (one binary, many replicas → one symbol table).

### D.3 End-to-end sequence (a slow request, Go, Tier-2)

```
DB goroutine G2 finishes, sends on chan -> wakes G1
        │  Kernel (eBPF L2)
        ▼
1. sched_switch(prev=G1.tid, prev_state=S)  -> record block-begin in task storage
2. sched_waking(wakee=G1, waker=G2.tid) blocked_ns=212ms >= min_block
3. waitReason was ChanReceive + item matches -> work handoff; edge G1<-G2 conf=high
4. EV_BLOCK_END{tid=G1, waker=G2, blocked=212ms, wait=WC_CHAN, stack_ids, cookie=R}
        │  Ringbuf reader (L1)
        ▼
5. resolve stack_id + waker_stack_id (async, cached)        │ Symbolizer
6. normalize event (goid via resolver, lineage cookie R)
        │  Analyzer (L4)
        ▼
7. build segments; match wait<->wake; cascaded redistribution
8. scope to cookie R; SCC-collapse cycles; dag_longest_path(start->end)
        │  CLI/Export (L5)
        ▼
9. CriticalPath{212ms chan-recv G1<-G2  <-  188ms epoll G2<-netpoll(DB)  ...}
10. render chain + flamechart; export pprof / OTLP-Profiles
```

---

## Part E — Analyzer Algorithm (L4): data structures + steps

### E.1 Segment reconstruction

Per `(tid | task_id)`, fold the event stream into an ordered segment list:

```go
type SegKind uint8
const ( Running SegKind = iota; Runnable; Blocked )

type Segment struct {
    Start, End uint64      // ns (bpf_ktime domain)
    Kind       SegKind     // RUNNING | RUNNABLE | BLOCKED
    WaitClass  WaitClass
    WakerID    uint64      // task that ended this BLOCKED segment
    Cookie     uint64
    Confidence uint8
    BlockedStk, WakerStk StackID
}
```

Match each `BLOCKED.end` to the `sched_waking` naming this id (the waker edge).
Drop false wakeups (immediate re-block, ε ≈ a few µs). RUNNABLE segments become
`WC_RUNQ` edges to a synthetic **"CPU/scheduler"** node.

### E.2 Wait-for graph + cascaded redistribution (wPerf, verbatim primitive)

```
function cascade(seg):                         # seg is a BLOCKED segment of thread w.ID
    addWeight(edge w.ID -> w.wakerID, seg.End - seg.Start)
    for each BLOCKED segment s of w.wakerID overlapping [seg.Start, seg.End):
        s' = clamp(s, seg.Start, seg.End)
        cascade(s')                            # recursive: emphasize nested waits
```

Nested waits (A waits B, B waits C during the same interval) get the `C→B` weight
bumped — correctly prioritizing edges whose optimization helps multiple tasks.
(This is wPerf Fig. 4; we keep it exactly.)

### E.3 Per-request critical path (our departure from wPerf)

1. **Scope** edges to one cookie R → subgraph `G_R` with timestamped nodes
   (segment endpoints).
2. **SCC-collapse (Tarjan):** a cycle within a request window = a *contention
   episode* → a single weighted super-node (preserves wPerf's "knot = bottleneck"
   insight inside the per-request view).
3. On the resulting DAG, **`longest_weighted_path(start_node → end_node)`** where
   node weight = on-CPU time, edge weight = `blocked_ns`. This path is the
   critical path (same primitive as Meta's HolisticTraceAnalysis
   `dag_longest_path`).
4. Emit ordered edges with: duration, wait_class, waker identity + stacks,
   **confidence**, and % of total wall-clock.

**Complexity:** Tarjan SCC is `O(V+E)`; DAG longest path on the condensation is
`O(V+E)` via topological order. Per request this is tiny; aggregation across
requests is the cost, handled by sharded parallel merge.

### E.4 False-wakeup suppression (concrete)

Not every `sched_waking` is a real handoff. We drop an edge `B ← A` when **B
re-blocks within ε without doing useful work**:

```
on segment close for B:
    run_after_wake = B.next_running_segment.duration   # on-CPU time after the wake
    if run_after_wake < EPS_RUN (≈ 5–20 µs) and
       B.next_blocked_segment.WaitClass == B.prev_blocked.WaitClass:
        mark edge (B<-A) spurious; do NOT add weight; do NOT propagate cookie
```

This is wPerf's lesson: a thread woken from a futex that immediately re-parks on
the same futex (lost race in a broadcast) did no work for the waker. `EPS_RUN` is
tunable and reported, because it trades recall (too high → drop real fast
handoffs) for precision (too low → keep herd noise). P0-B sweeps it.

### E.5 Longest weighted path (the critical-path DP)

After SCC-collapse the per-request graph `G_R` is a DAG with node weight =
on-CPU time and edge weight = `blocked_ns`. Single-source longest path on a DAG
is linear via topological order (no negative-cycle problem because it is acyclic
post-condensation):

```
topo = topological_order(G_R)            # Kahn or DFS finish order
dist[start] = weight(start)
pred[*] = nil
for u in topo:                           # relax in topo order
    for (u -> v) in edges:               # edge weight = blocked_ns on that wait
        cand = dist[u] + weight(v) + ewt(u->v)
        if cand > dist[v]:
            dist[v] = cand; pred[v] = u
# critical path = walk pred[] back from argmax_{end nodes} dist[end]
path = reconstruct(pred, argmax_end)
```

- **Node weight** = time the task spent *running* in that segment (compute on the
  path); **edge weight** = the blocked duration the downstream task waited.
- The path's total equals the request's wall-clock latency (start anchor → response
  send), which is the invariant we assert in tests: `Σpath == wall_clock ± slack`.
- Same primitive as Meta's HolisticTraceAnalysis `dag_longest_path`.

### E.6 Aggregation

Across requests: sum each critical-path edge's contribution; rank by total
ns-on-critical-path → **"critical-path flamechart."** This answers *"across my
P99s, which single wait dominates?"* — the actual user question. Edges below a
confidence threshold are shown separately as "ambiguous waits," never silently
folded into a confident number.

**Sharded parallel merge** (wPerf's pattern): requests are independent, so the
analyzer shards by `cookie mod N` across worker goroutines, each building its own
critical paths; a final reduce step sums per-edge contributions into a global
flamechart. Graph state stays in-process (no external store) — the only shared
structure is the symbol cache (read-mostly, `(build_id, file_offset)` keyed).

---

## Part F — System Architecture & Layering

```
 L5  CLI / Export        criticast record|analyze|top; pprof + OTLP-Profiles out; TUI
 ────────────────────────────────────────────────────────────────────────────────
 L4  Analyzer            segments -> wait-for graph -> SCC -> per-request critical path
 ────────────────────────────────────────────────────────────────────────────────
 L3  Attribution/Resolve lineage (parentGoid), span anchors, gopark waitReason,
                         tid->task_id, confidence + entropy scoring
 ────────────────────────────────────────────────────────────────────────────────
 L2  Kernel Collector    tp_btf sched_switch/waking + refinement probes; maps; ringbuf
 ────────────────────────────────────────────────────────────────────────────────
 L1  Loader/Agent        cilium/ebpf CO-RE load, attach tiering, ringbuf drain, symbolize
 ────────────────────────────────────────────────────────────────────────────────
 L0  Kernel              scheduler, BTF, task storage, ring buffer
```

- **L0/L2 is the only GPLv2 surface** (BPF object). Everything L1+ is Apache-2.0.
- **The L2→L4 contract is the fixed `struct event`** ([§B.2](#b2-event-record-the-l2l4-contract--fixed-layout-no-pointers)).
  Kernel emits raw facts; userspace does all enrichment. This keeps the kernel
  side small, auditable, and portable.
- **Internal IPC:** none required for the one-shot CLI (single process). The
  DaemonSet form ([Part J](#part-j--deployment--integration)) adds an OTLP
  exporter goroutine; storage is delegated to Pyroscope/Grafana — we do not
  reinvent a TSDB.

### F.1 Data-flow invariants

- **Single clock domain:** `bpf_ktime_get_ns` for all in-kernel timestamps →
  total order, no cross-CPU skew. Userspace wall-clock is used *only* for
  correlation/display, never for ordering (avoids wPerf's vDSO/perf-clock pain).
- **No probe stall:** ring overflow → drop + count, never block. The observer
  must never become the bottleneck it measures.
- **Idempotent enrichment:** symbolization and goid naming are pure functions of
  cached state; replaying the same event yields the same normalized event.

---

## Part G — Tech Stack (decided, justified)

| Layer | Choice | Justification (fact-based) |
|---|---|---|
| Kernel BPF | C + libbpf + CO-RE, `tp_btf` raw tracepoints | Only mature CO-RE path. Aya/rustc cannot emit BTF relocations yet (FOSDEM '26 Aya talk; aya #722). Rust kernel-side = portability landmine today. |
| Userspace | Go + cilium/ebpf | Pure-Go CO-RE loader, single static binary, best Go-symbolization libs (`debug/gosym`), native OTel/Pyroscope ecosystem. Runner-up `libbpf-rs` rejected on symbolization/OTel weight. |
| Codegen | `bpf2go` | Generates Go bindings + embeds the compiled BPF object; one `go build`. |
| Analyzer | Go (in-process), sharded parallel | wPerf's parallel pattern; keep graph in-proc. |
| Min kernel | 5.8+ (ringbuf, task_storage), BTF required | matches hud/OTel profiler; BTFhub fallback for older kernels. |
| Export | protobuf internal; OTLP-Profiles + pprof out | OTLP Profiles **public Alpha (2026)**, lossless pprof round-trip; integrate Pyroscope/Grafana, don't reinvent storage. |
| CI | kernel matrix `{5.8, 5.15, 6.1, 6.8, 6.12}` × Go `{1.21 → latest}` | CO-RE + offset correctness must be proven (aya #722 is the cautionary tale). |

> **OTLP-Profiles note (verified 2026):** the Profiles signal is in **public
> Alpha** — "the fourth signal," with transparent lossless pprof↔OTLP conversion
> and an OTel Collector `pprof` receiver. Production-ready *backends* are still
> emerging, so we ship **pprof as the stable default export** and OTLP-Profiles
> as the forward-looking path, gated behind a flag until it reaches Beta/GA.

---

## Part H — Phase 0: two blocking gates (kill-or-confirm)

**Phase 0 (≤ 3 weeks, two parallel PoCs; Phase 1 starts only if BOTH pass).**
They are independent — correctness measurement does not need the
overhead-optimized collector — so they run in parallel, and **P0-B can start day
one.** If P0-A fails, the product can't run in prod. If P0-B fails, the product
can't be trusted. Either failure stops Phase 1; both are cheap to learn now
versus six months in.

### H.1 P0-A — Overhead

**Single question:** can the required probes run on a busy service within the
[§B.5](#b5-overhead-budget-hard-measured-in-phase-0-a) budget?

**Build:**

1. **`bpftrace` spike first (1 day):** `tp_btf:sched_switch`/`sched_waking`,
   min-block filter, count events/sec + emit a histogram. Confirms event rates
   and classification on real load *before* writing C.
2. **libbpf-C collector:** maps (1)(2)(3)(5)(6), state machine
   [§B.3](#b3-the-in-kernel-state-machine-the-heart), record mode only.
3. **Go loader** (cilium/ebpf, `bpf2go`), ringbuf drain + drop accounting.
4. **`runtime.casgstatus` uprobe + goid** (DWARF + fallback) on a Go target;
   verify **no crash** (uprobe-only) across Go 1.21/1.22/1.24.

**Workloads:** (a) Go HTTP svc, channel fan-out to 3 simulated backends,
`wrk` 30–50k rps; (b) nginx/C epoll, same load (proves language-agnostic floor).

**Measure (≥ 5 runs, medians + p99):** throughput Δ, P50/99/99.9 Δ, events/s,
ringbuf drops, per-hit probe latency; in 3 modes (full / min-block 50µs /
sample 1/100).

**Gates:**

- full `<5%/<10%` → proceed full-fat.
- only sampled passes → proceed as **"sampled critical-path."**
- nothing passes → **STOP**, publish negative result (valuable in itself).

### H.2 P0-B — Attribution Accuracy (new, gates Phase 1)

This is a first-class Phase-0 deliverable, **parallel to and independent of**
P0-A. Correctness measurement can use a **fat, un-optimized** trace capture, so
the two PoCs decouple cleanly.

**Target service (deliberately adversarial topology):**

```
HTTP handler (Tier-1 anchor, known request id)
   ├─ go func() x2          <- deterministic lineage (parentGoid) — should be ~100%
   ├─ submit to WORKER POOL (M long-lived workers on shared chan)  <- ambiguous: work handoff
   ├─ acquire from CONN POOL (K conns via channel-of-connections)  <- ambiguous: resource handoff
   ├─ contend on shared MUTEX (cache)                              <- resource handoff
   └─ fan-in / response assembly                                   <- join
```

Drive with **interleaved concurrent load** (requests A, B, C overlapping) so
pools and locks genuinely contend — **ambiguity only appears under concurrency,
so a single-request test would lie.**

**Ground truth (two independent sources that must agree):**

1. **OpenTelemetry spans** with explicit parent links on every hop.
2. **A propagated ground-truth token in `context.Context`**, logged with the live
   goid at every goroutine boundary and every channel send/recv. This yields, for
   each wakeup edge the eBPF observes, the **true** request the woken goroutine
   was working for — the **gold label per edge.**

**Metrics (per-edge, with per-mechanism breakdown — this is the deliverable):**

| Metric | Definition |
|---|---|
| Edge precision | of edges we attributed to R, fraction truly R |
| Edge recall | of edges truly R, fraction we attributed to R |
| False-edge rate | fraction of attributed edges that are wrong |
| Per-mechanism matrix | precision/recall split by `{spawn-lineage, chan-work-handoff, conn-pool, mutex, broadcast, netpoll}` |
| Critical-path fidelity | edge-set overlap (Jaccard) between reconstructed critical path and ground-truth longest wall-clock path |
| Entropy-detector AUC | does cookie-entropy at a node predict ground-truth ambiguity? (validates the shared-resource detector) |

> **The per-mechanism matrix is the whole point.** It tells us which mechanisms
> to trust and ship, and which to label ambiguous — instead of one misleading
> aggregate number.

**Specific experiments inside P0-B:**

1. **Lineage-only attribution** (parentGoid spawn tree) → expect very high
   precision; establishes the floor.
2. **Work-handoff via `sudog.elem`** pointer correlation across the worker pool →
   measure if item-matching recovers worker attribution.
3. **Resource-handoff suppression** (mutex/pool keep waiter's own cookie) →
   confirm it removes false edges vs naive forward propagation.
4. **Naive v2 forward-propagation as the baseline to beat** → quantify exactly
   how wrong the original model was (expected to be the "~50% wrong" we fear; we
   want it on record).

**Decision gates (honest, mechanism-specific):**

| Outcome | Ship decision |
|---|---|
| Lineage + work-handoff ≥ 90% precision, resource edges correctly suppressed | Ship Tier-2 **default**; render resource/pool edges as "contention (request-ambiguous, confidence N%)." |
| Lineage ≥ 90% but work-handoff 50–90% | Ship **lineage-deterministic** attribution default; work-handoff behind a confidence threshold; pools/locks shown ambiguous. Still a real product. |
| Even lineage < 90% (something fundamental is wrong) | Tier-2 not viable; fall back to **Tier-0/1**. Still novel OSS, just without the auto-DAG headline. |
| Critical-path fidelity Jaccard < 0.7 | Do **not** market "the critical path"; market "dominant waits." |

This is what stops us shipping something 50% wrong: **we measure precision per
mechanism on adversarial concurrency before writing the product, and we let the
numbers — not optimism — decide** which mechanisms become default vs.
labeled-ambiguous vs. dropped.

**Two things to stay brutal about:**

1. **The channel-of-connections case may be permanently unattributable** without
   app context. The right response is honest UI, never a confident wrong answer.
   P0-B quantifies how much real critical-path time lands in this bucket; if it's
   large for common frameworks (`database/sql` pools), that caps product value —
   and we want the number *first*.
2. **`sudog.elem` correlation is itself an unproven experiment** (offset-drift
   tax like goid). P0-B tests whether it recovers worker-pool attribution or just
   adds complexity for marginal gain.

---

## Part I — Roadmap

| Phase | Duration | Deliverable |
|---|---|---|
| **P0** | ≤ 3 w | Two blocking gates (P0-A overhead, P0-B accuracy). Phase 1 starts only if **both** pass. |
| **P1** | 4–6 w | Tier-0/1, thread-level, C/C++/Go-thread. `record`/`analyze` CLI, critical-path text + flamechart, pprof export. **Shippable novel OSS — plant the flag.** |
| **P2** | 6–8 w | Go task-level (goid + gopark), lineage-first Tier-2 + accuracy validation gate vs instrumented ground truth. |
| **P3** | 4–6 w | continuous sampled mode, k8s DaemonSet, OTLP-Profiles, Grafana/Pyroscope, live TUI. |
| **P4** | — | Tokio (USDT), Tier-3 cross-process, JVM virtual threads (research), io_uring deep support. |

Each phase: **CI green on full matrix, overhead re-measured, docs, release. No
phase starts before prior gates pass.**

---

## Part J — Deployment & Integration

**Forms:**

- **(a) One-shot:** `criticast record --pid|--cgroup X --dur 10s` → report.
- **(b) k8s DaemonSet:** `hostPID: true`, `CAP_BPF` + `CAP_PERFMON` preferred
  over privileged; mount `/sys/kernel/btf` + `/proc`; OTLP-Profiles → collector →
  Pyroscope.
- **(c) Always-on sampled agent.**

**Posture:** complement APM. **Ingest** OTel span context (Tier-3 seed); **emit**
pprof/OTLP so existing Grafana/Pyroscope visualize it. Maximizes adoption ROI.

**Privacy advantage:** we read **timing + identity, not request payloads** (unlike
protocol tracers) — easier security sign-off.

### J.1 CLI surface (proposed)

```
criticast record  --pid <pid> | --cgroup <path> | --k8s-pod <ns/pod>
                  --dur 10s [--min-block 50us] [--sample 1] [--runtime auto|go|none|tokio]
                  -o trace.criticast
criticast analyze trace.criticast [--request <cookie>] [--top 10]
                  [--min-confidence 80] [--format text|json|pprof|otlp]
criticast top     --pid <pid> --dur 30s     # live "which wait dominates" TUI
criticast export  trace.criticast --pprof out.pb.gz | --otlp <endpoint>
```

### J.2 Permissions & security

- Prefer `CAP_BPF`+`CAP_PERFMON` (+`CAP_SYSLOG` for kallsyms) over
  `--privileged`.
- BTF required (`/sys/kernel/btf/vmlinux`); BTFhub-fetched BTF for kernels
  lacking embedded BTF.
- uprobes on the target binary require read access to the binary path; in k8s,
  resolve via `/proc/<pid>/root`.

---

## Part K — Edge Cases (enumerated, with handling)

1. **Preempt vs block** — `prev_state==0` & `preempt` → RUNNABLE; mishandling
   corrupts everything. (Verified via `__trace_sched_switch_state`.)
2. **Busy-wait / spinlocks** — never enter kernel; v1 blind spot, documented;
   future uprobe / HW-watchpoint (Tapestry).
3. **io_uring** — `io_uring_submit_req`/`complete` tracepoints, SQE→CQE LRU map;
   BPF-controlled io_uring (kernel 6.x) = "stuck in syscall" blind spot,
   documented.
4. **Broadcast / thundering-herd wakeups** — false edges; TTL +
   false-wakeup suppression + confidence + entropy demotion.
5. **Go uretprobe crash** — entry-only / RET-offset uprobes. No exceptions.
6. **goid / gopark version drift** — DWARF-first + `offsets.json` + CI per
   release.
7. **Thread migration mid-wait (M:N)** — why `task_id` is mandatory, not tid.
8. **Tokio id reuse** — `(spawn_ts, id)` namespacing.
9. **Ring overflow under burst** — drop + count, adaptive `min_block` backpressure
   via `cfg` map, never stall the probe.
10. **Stripped binaries** — `offsets.json` / BTF-of-binary; degrade to
    thread-level.
11. **PID namespaces / containers** — `/proc` + cgroup translation, build-id
    symbol cache, debuginfod.
12. **Short-lived threads** — merge into a virtual long-running thread per
    spawn-site (wPerf M-SHORT).
13. **Clock domains** — single `bpf_ktime_get_ns` in-kernel for total order;
    userspace clocks only for correlation, never ordering.
14. **GC pauses** — `gopark` reason `waitReasonGC*` / JVM safepoint → distinct
    `WC_GC` wait class.
15. **D-state (uninterruptible)** — disk/IO vs S lock/sleep; refine with
    syscall / io_uring context.
16. **Shared-resource false attribution (the v2 bug)** — lineage-first identity +
    resource-handoff suppression + entropy detector; never inherit a waker's
    request cookie at a mutex/pool. (See [§0.3](#03-what-changed-from-v2--the-load-bearing-correction).)
17. **Channel-of-connections vs channel-of-work** — `sudog.elem` correlation when
    possible; otherwise honest "request-ambiguous" labeling. Permanent limitation,
    not a bug.

---

## Part L — Risks (brutal)

| Risk | Severity | Likelihood | Mitigation / kill |
|---|---|---|---|
| Overhead unacceptable on hot path | Fatal | Medium | P0-A gates; STOP or ship sampled rung |
| Tier-2 attribution too noisy | High | Med-High | Lineage-first; degrade to Tier-0/1; ship default only if precision ≥ 90% per mechanism |
| Channel-of-connections unattributable | Medium | Certain (partial) | Honest "ambiguous" UI; quantify the bucket in P0-B |
| `sudog.elem` correlation doesn't pay off | Medium | Medium | It's an experiment in P0-B; drop if marginal |
| Go offset maintenance tax | Medium | Certain over time | DWARF-first + CI per release; budget ongoing |
| Someone ships first | Medium | Medium | 6–12 mo window; ship P1 fast; OSS community moat |
| CO-RE breakage across kernels | Medium | Medium | CI matrix; no Rust kernel-side; BTFhub |
| Scope creep into distributed tracing | High | High | Charter discipline: **single-host critical path, complements APM** |

---

## Part M — Success / ROI / Naming

- **Technical gates:** P0-A overhead met; P0-B lineage ≥ 90% precision and at
  least one shippable mechanism; per-request critical-path fidelity Jaccard ≥ 0.7
  (else market "dominant waits," not "the critical path").
- **Adoption:** the README critical-path screenshot is the growth engine; target
  an HN / eBPF-Summit-grade P1 demo on **nginx + Go svc**. Benchmarks for
  reference: fgprof ~3.1k★, hud ~150★/3mo.
- **Impact statement:** root-cause a P99 regression, **zero code changes,
  < 5 min.**
- **Name:** `criticast` (lead) / `critipath`. **License:** Apache-2.0 userspace,
  GPLv2 BPF object.

---

## Appendix N — Verified facts & sources

| Fact | Source (verified) |
|---|---|
| `sched_switch` proto `(preempt, prev, next, prev_state)`; `preempt==true` reports RUNNING | `include/trace/events/sched.h`, `__trace_sched_switch_state()` |
| `sched_waking` proto `(struct task_struct *p)`; waker = current task | kernel tracepoint; `bcc/offwaketime` `waker()` uses `bpf_get_current_pid_tgid()` |
| Prefer `sched_waking` over `sched_wakeup` for latency | Perfetto scheduling docs |
| Thread-level wait-for edge set suffices to find bottlenecks; "knot = bottleneck"; cascaded redistribution (Fig. 4) | wPerf, OSDI '18 |
| `g.parentGoid uint64 // goid of goroutine that created this goroutine`; `g.waiting *sudog`; offsets drift across versions | Go `src/runtime/runtime2.go` (verified across go1.23.4 and master) |
| `gopark` `waitReason` is 3rd arg (RCX/PARM3); reasons incl. `waitReasonChanReceive/Send`, `waitReasonSyncMutexLock`, `waitReasonGC*` | Go runtime; signature drift 1.19–1.20 vs 1.21+ |
| Go uretprobe corrupts on movable stacks → entry-only uprobes | bcc #1320, cilium/ebpf #759 |
| Go FP unwinding default ≥ 1.21 (amd64/arm64), <1% overhead | Datadog profiler reports |
| Aya/rustc cannot emit BTF relocations yet → C+libbpf for kernel side | FOSDEM '26 Aya talk; aya #722 |
| Ring buffer + task storage require kernel ≥ 5.8 | kernel feature history |
| OTLP Profiles = public Alpha (2026), lossless pprof↔OTLP round-trip, OTel Collector pprof receiver, eBPF profiler reference agent | opentelemetry.io/blog/2026/profiles-alpha; Elastic Observability Labs |
| `dag_longest_path` as critical-path primitive | Meta HolisticTraceAnalysis |
| Socket 4-tuple / TCP-seq correlation; partial coverage honesty | Pixie `ConnTracker`; DeepFlow |
| tokio USDT probes (`task-poll-start/end`, spawn, terminate) need `tokio_unstable` + `register_hooks` | tokio-dtrace / `tokio_unstable` |

---

## Appendix O — Glossary

- **Critical path** — the longest weighted path through a request's wait-for DAG;
  the chain of waits whose sum equals the request's wall-clock latency.
- **Wait-for edge** — directed `wakee → waker` edge born at `sched_waking`,
  weighted by blocked duration. Observed, almost always correct.
- **Request identity / cookie** — the token saying "this work belongs to request
  R." Carried by lineage (entry anchor + spawn tree), *not* inherited from a
  waker (v3 correction).
- **Lineage-first attribution** — assigning request identity by deterministic
  spawn ancestry (`parentGoid`) rather than heuristic waker propagation.
- **Resource handoff** — a wakeup at a mutex/semaphore/connection-pool where the
  waker's request is *not* the wakee's request. Identity must **not** propagate.
- **Work handoff** — a wakeup at a channel carrying a work item; identity may
  propagate **iff** the item matches (`sudog.elem`).
- **Entropy detector** — flags a node woken by many different request cookies as a
  shared resource, demoting its edges to `request-ambiguous`.
- **Wait class** — `FUTEX/EPOLL/IO_URING/NET/DISK/RUNQ/SLEEP/GC/CHAN/MUTEX/...`;
  refined from `prev_state` + syscall/gopark context.
- **CO-RE** — Compile Once, Run Everywhere (BTF-relocated BPF).
- **`tp_btf`** — BTF-typed raw tracepoint; lowest-overhead attach on ≥ 5.5.

---

## Appendix P — Wire formats & schemas

### P.1 `struct event` (L2→L4, 80 bytes)

See [§B.2](#b2-event-record-the-l2l4-contract--fixed-layout-no-pointers).
Fixed layout, pointer-free, `bpf2go`-mirrored in Go. All multi-byte fields are
native-endian (same host); the trace file records endianness + struct version in
its header.

### P.2 `.criticast` trace file (on-disk capture)

```
Header  { magic="CRTC", version, endianness, ktime_base_ns, wall_base_ns,
          kernel_release, arch, runtime, go_build_version, struct_event_version }
Section STACKS   : stack_id -> [pc...]            (deduped, build-id keyed)
Section BUILDIDS : build_id -> module path/offset base
Section EVENTS   : []struct event                 (time-ordered by ts_ns)
Section SPAWNS   : []{parent_task_id, child_task_id, ts_ns}   (lineage)
Section SPANS    : []{cookie, tid, fd, open_ts, close_ts}     (Tier-1 anchors)
Footer  { stats: emitted, drops, short_filtered, sampled_out }
```

### P.3 Export targets

- **pprof** (stable default): one profile per "critical-path wait" sample type;
  location = waker stack; value = `blocked_ns`-on-critical-path. Round-trips
  losslessly to OTLP-Profiles.
- **OTLP-Profiles** (flagged, forward-looking): map each critical-path edge to a
  profile `Sample` with `attribute`s `{wait_class, confidence, request_cookie,
  waker_task_id}`; resource attributes carry k8s pod/container identity via the
  Collector `k8sattributesprocessor`.
- **JSON** (debug / programmatic): the full per-request DAG with confidence per
  edge.

---

## Appendix Q — Operational artifacts

### Q.1 Phase 0-A bpftrace spike (day-1, before any C)

The cheapest possible confidence check: confirm event rates and the
preempt-vs-block classification on the real target *before* writing the C
collector.

```c
// criticast-spike.bt — count sched events/s and block-vs-preempt split.
tracepoint:sched:sched_switch {
    if (args->prev_state == 0) { @preempt = count(); }
    else { @block = count(); @blockclass[args->prev_state] = count(); }
}
tracepoint:sched:sched_waking {
    @wakes = count();
}
interval:s:1 {
    print(@preempt); print(@block); print(@wakes);
    clear(@preempt); clear(@block); clear(@wakes);
}
// Run: bpftrace criticast-spike.bt -p <pid>   (or system-wide for the floor)
```

If `@wakes`/s already approaches the >1M/s danger zone with the target under
load, the in-kernel `min_block` + sampling levers are mandatory, not optional —
and that result alone shapes the collector design.

### Q.2 `offsets.json` schema (Go runtime offset database)

DWARF-first resolution; this file is the fallback for stripped binaries and is
CI-generated per Go release.

```jsonc
{
  "schema": 1,
  "go1.21.0": {
    "g.goid":        { "amd64": 152, "arm64": 152 },
    "g.parentGoid":  { "amd64": 280, "arm64": 280 },
    "g.atomicstatus":{ "amd64": 144, "arm64": 144 },
    "g.waiting":     { "amd64": 192, "arm64": 192 },   // *sudog
    "sudog.elem":    { "amd64": 0,   "arm64": 0   },
    "gopark.reason_reg": "RCX",                          // 3rd arg in regabi
    "g_register":    { "amd64": "R14", "arm64": "R28" }
  },
  "go1.22.0": { "...": "..." }   // regenerated by CI on each release tag
}
```

Resolution order at attach time: (1) DWARF `AttrDataMemberLoc` if the binary has
symbols; (2) `offsets.json[buildVersion]` keyed by `runtime.buildVersion` read
from the `.go.buildinfo` ELF section; (3) refuse goid lifting and **degrade to
thread-level** (still a working Tier-0/1 product).

### Q.3 k8s DaemonSet (security-minimal)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata: { name: criticast, namespace: observability }
spec:
  template:
    spec:
      hostPID: true
      containers:
        - name: criticast
          image: ghcr.io/criticast/agent:vX
          args: ["agent", "--sample", "100", "--min-block", "50us",
                 "--otlp", "$(OTLP_ENDPOINT)"]
          securityContext:
            capabilities:
              add: ["BPF", "PERFMON", "SYS_RESOURCE"]   # NOT privileged
              drop: ["ALL"]
          volumeMounts:
            - { name: btf,  mountPath: /sys/kernel/btf, readOnly: true }
            - { name: proc, mountPath: /host/proc,      readOnly: true }
      volumes:
        - { name: btf,  hostPath: { path: /sys/kernel/btf } }
        - { name: proc, hostPath: { path: /proc } }
```

Prefer `CAP_BPF`+`CAP_PERFMON` over `privileged: true`; `CAP_SYS_RESOURCE` only
where `RLIMIT_MEMLOCK` is not auto-managed (pre-5.11 memcg accounting).

---

## Appendix R — Threat model, resource budget, testing

### R.1 Threat model / security posture

- **Read-only observation.** `criticast` never writes target memory, never
  injects context, never modifies syscalls. uprobes are *entry-only reads*.
- **No payload access.** We read **timing + identity** (pids, goids, 4-tuples,
  lock addresses), never request bodies — the core privacy advantage over L7
  protocol tracers, and the easiest path to security sign-off.
- **Capability surface.** `CAP_BPF`+`CAP_PERFMON` (+`CAP_SYSLOG` for kallsyms).
  No `CAP_SYS_ADMIN`/privileged in the supported posture.
- **Verifier as a safety net.** All BPF is CO-RE + verifier-checked: bounded
  loops, bounded stack, no arbitrary kernel writes. The BPF object is GPLv2 and
  auditable in isolation from the Apache-2.0 userspace.
- **Multi-tenant scoping.** Target filtering by tgid/cgroup means a node agent
  only emits events for opted-in workloads; cookie/identity never crosses tenant
  cgroup boundaries.

### R.2 Userspace agent resource budget (targets)

| Resource | Target | Mechanism |
|---|---|---|
| Agent CPU | < 0.5 core under sampled mode | per-CPU decode, batched ring drain |
| Agent RSS | < 256 MiB steady | LRU symbol cache, bounded event channel |
| Ring buffer | 16 MiB × scale; drop+count on overflow | never stalls probe (Part F invariant) |
| Symbol cache | shared `(build_id, file_offset)` | one table per binary across replicas |
| Trace file | ~bounded by `--dur` × event-rate × 80 B | min-block + sampling cap it |

### R.3 Testing & CI matrix

- **Correctness:** kernel matrix `{5.8, 5.15, 6.1, 6.8, 6.12}` × Go
  `{1.21 → latest}` in VM-based CI (e.g. `vmtest`/`qemu`); every Go release
  regenerates `offsets.json` and re-runs the goid/gopark/parentGoid extraction
  tests against a known binary.
- **Attribution regression:** the P0-B adversarial service ships as a CI fixture;
  per-mechanism precision/recall is asserted to not regress below the shipped
  thresholds (lineage ≥ 90%, declared work-handoff threshold, Jaccard ≥ 0.7).
- **Overhead regression:** the Part H workloads run nightly on a fixed rig;
  throughput/P99 deltas are tracked over time so a probe change that breaches
  §B.5 fails CI.
- **Fuzz / robustness:** stripped binaries, exotic kernels (no embedded BTF →
  BTFhub), PID-namespace containers, and burst load (ring overflow) are explicit
  test cases mapping to [Part K](#part-k--edge-cases-enumerated-with-handling).

---

*End of Charter v3. The two gates in [Part H](#part-h--phase-0-two-blocking-gates-kill-or-confirm)
decide everything downstream. Build P0-A and P0-B first; trust nothing else until
both are green.*
