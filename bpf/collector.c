/* SPDX-License-Identifier: GPL-2.0-only */
/*
 * criticast kernel collector — CHARTER §B.1–B.3
 *
 * Lineage-first: cookie is read on the sched path, never written here (L3/uprobes set it).
 *
 * Current scope:
 *   - sched_switch / sched_waking wait-for edges + run-queue close
 *   - Subject stack at gopark; waker stack on sched_waking / EV_BLOCK_END
 *   - tid_to_task via casgstatus; last_sudog_elem via gopark → event.aux on block end
 *   - Filter order: targeted → prev_state (block only) → min_block → sample → ringbuf
 */
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#include "event.h"
#include "go_cfg.h"

/* config.flags bits (userspace: internal/loader.CfgEmitRunning) */
#define CFG_EMIT_RUNNING (1u << 0)

char LICENSE[] SEC("license") = "GPL";

struct wait_state {
	__u64 last_switch_out_ns;
	__u64 last_switch_in_ns;
	__u64 waking_ns;
	__u32 waker_tid;
	__u32 waker_stack_id;
	__s32 subject_stack_id;
	__u8 prev_state;
	__u8 wait_class;
	__u8 _pad[1];
	__u64 cookie;
	__u64 cookie_expire_ns;
	__u64 task_id;
	__u64 futex_uaddr;
	__u64 last_sudog_elem;
};

struct config {
	__u64 min_block_ns;
	__u32 sample_mod;
	__u32 flags;
	__u64 cookie_ttl_ns;
};

struct {
	__uint(type, BPF_MAP_TYPE_TASK_STORAGE);
	__uint(map_flags, BPF_F_NO_PREALLOC);
	__type(key, int);
	__type(value, struct wait_state);
} thread_state SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_STACK_TRACE);
	__uint(max_entries, 65536);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, 127 * sizeof(__u64));
} stacks SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 16 << 20);
} events SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 4096);
	__type(key, __u32);
	__type(value, __u8);
} targets SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct config);
} cfg SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, STAT_MAX);
	__type(key, __u32);
	__type(value, __u64);
} stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1 << 20);
	__type(key, __u32);
	__type(value, __u64);
} tid_to_task SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct go_cfg);
} go_cfg_map SEC(".maps");

/* sudog.elem pointer → last seen ktime (LRU); avoids stale aux after reuse (charter K). */
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 8192);
	__type(key, __u64);
	__type(value, __u64);
} sudog_elem_seen SEC(".maps");

_Static_assert(sizeof(struct event) == 80, "struct event must be 80 bytes");

static __always_inline bool sudog_elem_fresh(__u64 elem, __u64 now, __u64 ttl_ns)
{
	if (!elem)
		return false;
	if (!ttl_ns)
		return true;
	__u64 *seen = bpf_map_lookup_elem(&sudog_elem_seen, &elem);
	if (!seen)
		return false;
	return (now - *seen) <= ttl_ns;
}

static __always_inline void inc_stat(__u32 idx)
{
	__u64 *v = bpf_map_lookup_elem(&stats, &idx);
	if (v)
		__sync_fetch_and_add(v, 1);
}

static __always_inline struct config *cfg_get(void)
{
	__u32 k = 0;
	return bpf_map_lookup_elem(&cfg, &k);
}

static __always_inline bool targeted(__u32 tgid)
{
	return bpf_map_lookup_elem(&targets, &tgid) != NULL;
}

static __always_inline __u64 lookup_task_id(__u32 tid)
{
	__u64 *id = bpf_map_lookup_elem(&tid_to_task, &tid);
	return id ? *id : 0;
}

static __always_inline enum wait_class classify_prev(__u8 prev_state)
{
	if (prev_state == 2)
		return WC_DISK;
	if (prev_state == 1)
		return WC_UNKNOWN;
	return WC_UNKNOWN;
}

static __always_inline int emit_runq(void *ctx, struct task_struct *task, __u64 runq_ns)
{
	struct config *c = cfg_get();
	if (!c)
		return 0;

	if (c->sample_mod > 1 && (bpf_get_prandom_u32() % c->sample_mod) != 0) {
		inc_stat(STAT_SAMPLED_OUT);
		return 0;
	}

	struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
	if (!e) {
		inc_stat(STAT_RINGBUF_DROPS);
		return 0;
	}

	__u32 tid = BPF_CORE_READ(task, pid);
	__u32 tgid = BPF_CORE_READ(task, tgid);

	__builtin_memset(e, 0, sizeof(*e));
	e->ts_ns = bpf_ktime_get_ns();
	e->type = EV_RUNQ;
	e->cpu = bpf_get_smp_processor_id();
	e->tgid = tgid;
	e->tid = tid;
	e->blocked_ns = runq_ns;
	e->wait_class = WC_RUNQ;
	e->cookie = 0;
	e->task_id = lookup_task_id(tid);
	e->stack_id = -1;
	e->waker_stack_id = -1;

	bpf_ringbuf_submit(e, 0);
	inc_stat(STAT_EVENTS_EMITTED);
	inc_stat(STAT_RUNQ_CLOSED);
	return 0;
}

static __always_inline int emit_running(void *ctx, struct task_struct *task, __u64 running_ns)
{
	struct config *c = cfg_get();
	if (!c || running_ns == 0 || !(c->flags & CFG_EMIT_RUNNING))
		return 0;

	if (c->sample_mod > 1 && (bpf_get_prandom_u32() % c->sample_mod) != 0) {
		inc_stat(STAT_SAMPLED_OUT);
		return 0;
	}

	struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
	if (!e) {
		inc_stat(STAT_RINGBUF_DROPS);
		return 0;
	}

	__u32 tid = BPF_CORE_READ(task, pid);
	__u32 tgid = BPF_CORE_READ(task, tgid);

	__builtin_memset(e, 0, sizeof(*e));
	e->ts_ns = bpf_ktime_get_ns();
	e->type = EV_TASK_STATE;
	e->cpu = bpf_get_smp_processor_id();
	e->tgid = tgid;
	e->tid = tid;
	e->blocked_ns = running_ns;
	e->wait_class = WC_UNKNOWN;
	e->task_id = lookup_task_id(tid);
	e->stack_id = -1;
	e->waker_stack_id = -1;

	bpf_ringbuf_submit(e, 0);
	inc_stat(STAT_EVENTS_EMITTED);
	inc_stat(STAT_RUNNING_EMITTED);
	return 0;
}

static __always_inline int emit_block_end(void *ctx, struct task_struct *wakee,
					struct wait_state *ws, __u64 blocked,
					__u32 waker_tid)
{
	struct config *c = cfg_get();
	if (!c)
		return 0;

	if (blocked < c->min_block_ns) {
		inc_stat(STAT_SHORT_FILTERED);
		return 0;
	}

	if (c->sample_mod > 1 && (bpf_get_prandom_u32() % c->sample_mod) != 0) {
		inc_stat(STAT_SAMPLED_OUT);
		return 0;
	}

	struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
	if (!e) {
		inc_stat(STAT_RINGBUF_DROPS);
		return 0;
	}

	__u32 tid = BPF_CORE_READ(wakee, pid);
	__u32 tgid = BPF_CORE_READ(wakee, tgid);

	__builtin_memset(e, 0, sizeof(*e));
	e->ts_ns = bpf_ktime_get_ns();
	e->type = EV_BLOCK_END;
	e->cpu = bpf_get_smp_processor_id();
	e->tgid = tgid;
	e->tid = tid;
	e->waker_tid = waker_tid;
	e->blocked_ns = blocked;
	e->prev_state = ws->prev_state;
	e->wait_class = ws->wait_class ? ws->wait_class : classify_prev(ws->prev_state);
	__u64 now = e->ts_ns;
	if (ws->futex_uaddr) {
		e->aux = ws->futex_uaddr;
	} else if (sudog_elem_fresh(ws->last_sudog_elem, now, c->cookie_ttl_ns)) {
		e->aux = ws->last_sudog_elem;
	} else {
		e->aux = 0;
	}
	e->cookie = ws->cookie;
	e->task_id = lookup_task_id(tid);
	e->waker_task_id = lookup_task_id(waker_tid);
	e->stack_id = ws->subject_stack_id;
	e->waker_stack_id = bpf_get_stackid(ctx, &stacks, BPF_F_USER_STACK);
	if (e->waker_stack_id < 0)
		inc_stat(STAT_STACK_FAIL);

	ws->last_sudog_elem = 0;
	ws->futex_uaddr = 0;
	ws->wait_class = 0;
	ws->subject_stack_id = -1;

	bpf_ringbuf_submit(e, 0);
	inc_stat(STAT_EVENTS_EMITTED);
	inc_stat(STAT_BLOCKS_SEEN);
	return 0;
}

SEC("tp_btf/sched_switch")
int handle_switch(u64 *ctx)
{
	struct task_struct *prev = (struct task_struct *)ctx[1];
	struct task_struct *next = (struct task_struct *)ctx[2];
	__u32 prev_state = (__u32)ctx[3];
	__u64 now = bpf_ktime_get_ns();

	inc_stat(STAT_SWITCH_SEEN);

	__u32 prev_tgid = BPF_CORE_READ(prev, tgid);
	if (targeted(prev_tgid)) {
		inc_stat(STAT_TARGET_PREV);
		struct wait_state *ws = bpf_task_storage_get(
			&thread_state, prev, NULL,
			BPF_LOCAL_STORAGE_GET_F_CREATE);
		if (ws) {
			if (ws->last_switch_in_ns && now > ws->last_switch_in_ns)
				emit_running(ctx, prev, now - ws->last_switch_in_ns);
			ws->last_switch_out_ns = now;
			ws->last_switch_in_ns = 0;
			ws->prev_state = (__u8)prev_state;
			if (prev_state == 0)
				inc_stat(STAT_PREEMPTS);
		}
	}

	__u32 next_tgid = BPF_CORE_READ(next, tgid);
	if (targeted(next_tgid)) {
		struct wait_state *ws = bpf_task_storage_get(
			&thread_state, next, NULL,
			BPF_LOCAL_STORAGE_GET_F_CREATE);
		if (ws) {
			ws->last_switch_in_ns = now;
			if (ws->waking_ns) {
				__u64 runq = now - ws->waking_ns;
				emit_runq(ctx, next, runq);
				ws->waking_ns = 0;
			}
		}
	}
	return 0;
}

SEC("tp_btf/sched_waking")
int handle_waking(u64 *ctx)
{
	struct task_struct *wakee = (struct task_struct *)ctx[0];
	__u32 wakee_tgid = BPF_CORE_READ(wakee, tgid);
	if (!targeted(wakee_tgid))
		return 0;

	struct wait_state *ws = bpf_task_storage_get(&thread_state, wakee, NULL, 0);
	if (!ws || ws->prev_state == 0)
		return 0;

	__u64 now = bpf_ktime_get_ns();
	__u64 blocked = now - ws->last_switch_out_ns;

	__u64 waker_pidtgid = bpf_get_current_pid_tgid();
	__u32 waker_tid = (__u32)waker_pidtgid;
	ws->waking_ns = now;
	ws->waker_tid = waker_tid;

	return emit_block_end(ctx, wakee, ws, blocked, waker_tid);
}

#include "go_probe.c"
