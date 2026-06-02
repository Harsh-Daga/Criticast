/* SPDX-License-Identifier: GPL-2.0-only */
/*
 * Go runtime uprobes — included from collector.c only (single BPF object).
 * Entry uprobes only (CHARTER §B.4).
 */

#include "go_waitreason.h"

static __always_inline void *go_current_g(struct pt_regs *ctx)
{
#if defined(__TARGET_ARCH_x86)
	return (void *)(unsigned long)BPF_CORE_READ(ctx, r14);
#elif defined(__TARGET_ARCH_arm64)
	return (void *)(unsigned long)BPF_CORE_READ(ctx, regs[28]);
#else
	return NULL;
#endif
}

static __always_inline struct wait_state *current_wait_state(void)
{
	struct task_struct *task = bpf_get_current_task_btf();
	if (!task)
		return NULL;
	return bpf_task_storage_get(&thread_state, task, NULL,
				    BPF_LOCAL_STORAGE_GET_F_CREATE);
}

static __always_inline int capture_sudog_elem(struct pt_regs *ctx, struct go_cfg *gc,
					    struct wait_state *ws)
{
	if (!gc || !gc->waiting_off || !ws)
		return 0;

	void *gp = go_current_g(ctx);
	if (!gp)
		return 0;

	void *sudog = NULL;
	if (bpf_probe_read_user(&sudog, sizeof(sudog), (char *)gp + gc->waiting_off) < 0)
		return 0;
	if (!sudog)
		return 0;

	__u64 elem = 0;
	if (bpf_probe_read_user(&elem, sizeof(elem), (char *)sudog + gc->sudog_elem_off) < 0)
		return 0;
	if (!elem)
		return 0;

	ws->last_sudog_elem = elem;
	__u64 now = bpf_ktime_get_ns();
	bpf_map_update_elem(&sudog_elem_seen, &elem, &now, BPF_ANY);
	return 0;
}

SEC("uprobe")
int up_casgstatus(struct pt_regs *ctx)
{
	__u32 k = 0;
	struct go_cfg *gc = bpf_map_lookup_elem(&go_cfg_map, &k);
	if (!gc || gc->goid_off == 0)
		return 0;

	void *gp = (void *)PT_REGS_PARM1(ctx);
	__u64 goid = 0;
	if (bpf_probe_read_user(&goid, sizeof(goid), (char *)gp + gc->goid_off) < 0)
		return 0;
	if (goid == 0 || goid > (1ULL << 30))
		return 0;

	__u32 tid = (__u32)bpf_get_current_pid_tgid();
	bpf_map_update_elem(&tid_to_task, &tid, &goid, BPF_ANY);
	return 0;
}

SEC("uprobe")
int up_gopark(struct pt_regs *ctx)
{
	__u32 k = 0;
	struct go_cfg *gc = bpf_map_lookup_elem(&go_cfg_map, &k);
	struct wait_state *ws = current_wait_state();
	if (!ws)
		return 0;

	__u32 reason = (__u32)PT_REGS_PARM3(ctx);
	ws->wait_class = go_reason_to_wait_class(reason);

	if (reason == GO_WAIT_REASON_SYNC_MUTEX_LOCK ||
	    reason == GO_WAIT_REASON_SYNC_RW_MUTEX_RLOCK ||
	    reason == GO_WAIT_REASON_SYNC_RW_MUTEX_LOCK) {
		__u64 lock = (__u64)PT_REGS_PARM2(ctx);
		if (lock)
			ws->futex_uaddr = lock;
	}

	if (gc)
		capture_sudog_elem(ctx, gc, ws);

	__s32 sid = bpf_get_stackid(ctx, &stacks, BPF_F_USER_STACK);
	if (sid >= 0)
		ws->subject_stack_id = sid;
	else
		inc_stat(STAT_STACK_FAIL);

	return 0;
}
