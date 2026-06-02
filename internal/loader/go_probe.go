package loader

// GoProbeOffsets are runtime.g field offsets for Go uprobes (from bpf/offsets.json or DWARF).
type GoProbeOffsets struct {
	GoidOff      uint32
	WaitingOff   uint32
	SudogElemOff uint32
}
