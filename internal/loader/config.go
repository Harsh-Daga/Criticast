package loader

// Config mirrors the bpf config map (CHARTER §B.1).
type Config struct {
	MinBlockNs  uint64
	SampleMod   uint32
	Flags       uint32
	CookieTTLNs uint64 // sudog.elem freshness in BPF (0 = no TTL check)
}

// CfgEmitRunning enables EV_TASK_STATE RUNNING on sched_switch (diagnostics; higher volume).
const CfgEmitRunning uint32 = 1 << 0

// DefaultConfig returns charter-friendly defaults for record.
func DefaultConfig() Config {
	return Config{
		MinBlockNs:  1000,
		SampleMod:   1,
		CookieTTLNs: 30 * 1e9, // 30s sudog.elem LRU window
	}
}
