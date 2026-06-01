/* SPDX-License-Identifier: GPL-2.0-only */
/*
 * L2→L4 wire contract — CHARTER §B.2 (80 bytes, fixed layout).
 * Layout change requires event version bump + bpf2go regen.
 */
#ifndef CRITICAST_EVENT_H
#define CRITICAST_EVENT_H

/*
 * BPF (-D__BPF__): use portable typedefs below — never host <linux/types.h>
 * (breaks clang -target bpf: needs asm/types.h).
 * Userspace uses Go types in internal/event; this header is for bpf C sources only.
 */
#ifdef __BPF__
#ifndef __u8
typedef unsigned char __u8;
#endif
#ifndef __u32
typedef unsigned int __u32;
#endif
#ifndef __u64
typedef unsigned long long __u64;
#endif
#ifndef __s32
typedef int __s32;
#endif
#else
#include <linux/types.h>
#endif

enum ev_type : __u8 {
	EV_BLOCK_BEGIN = 0,
	EV_BLOCK_END,
	EV_RUNQ,
	EV_SYSCALL_BOUNDARY,
	EV_IO_SUBMIT,
	EV_IO_COMPLETE,
	EV_TASK_STATE,
	EV_SPAN_OPEN,
	EV_SPAN_CLOSE,
	EV_SPAWN,
};

enum wait_class : __u8 {
	WC_UNKNOWN = 0,
	WC_FUTEX,
	WC_EPOLL,
	WC_IO_URING,
	WC_NET,
	WC_DISK,
	WC_RUNQ,
	WC_SLEEP,
	WC_GC,
	WC_CHAN,
	WC_MUTEX,
	WC_SELECT,
	WC_SEMA,
	WC_COND,
};

/* Must remain 80 bytes — verified by _Static_assert in collector.c */
struct event {
	__u64 ts_ns;
	__u32 cpu;
	__u32 tgid;
	__u32 tid;
	__u32 waker_tid;
	__u64 blocked_ns;
	__s32 stack_id;
	__s32 waker_stack_id;
	__u64 cookie;
	__u64 task_id;
	__u64 waker_task_id;
	__u64 aux;
	__u8 type;
	__u8 wait_class;
	__u8 prev_state;
	__u8 confidence;
};

#define EVENT_SIZE sizeof(struct event)

enum stat_idx {
	STAT_RINGBUF_DROPS = 0,
	STAT_EVENTS_EMITTED,
	STAT_BLOCKS_SEEN,
	STAT_PREEMPTS,
	STAT_RUNQ_CLOSED,
	STAT_SHORT_FILTERED,
	STAT_SAMPLED_OUT,
	STAT_STACK_FAIL,
	STAT_SWITCH_SEEN,   /* any sched_switch (debug: prog running) */
	STAT_TARGET_PREV,   /* sched_switch where prev is in targets map */
	STAT_MAX,
};

#endif /* CRITICAST_EVENT_H */
