package loader

// Config mirrors the bpf config map (CHARTER §B.1).
type Config struct {
	MinBlockNs  uint64
	SampleMod   uint32
	Flags       uint32
	CookieTTLNs uint64
}
