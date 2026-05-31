/* SPDX-License-Identifier: GPL-2.0-only */
/*
 * Go casgstatus uprobe — included from collector.c only (single BPF object).
 * Maps tid_to_task and go_cfg_map are defined above in collector.c.
 */

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
	/* Reject pointer-sized garbage; real goids are small positive integers. */
	if (goid == 0 || goid > (1ULL << 30))
		return 0;

	__u32 tid = (__u32)bpf_get_current_pid_tgid();
	bpf_map_update_elem(&tid_to_task, &tid, &goid, BPF_ANY);
	return 0;
}
