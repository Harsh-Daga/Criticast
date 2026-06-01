package trace

import (
	"encoding/binary"
	"encoding/json"
)

const (
	Magic = "CRTC"

	Version1 = 1
	Version2 = 2

	// LatestVersion is the format written by record.
	LatestVersion = Version2

	sectionStacks  = "stacks"
	sectionModules = "modules"
	sectionFooter  = "footer"
)

// V2Header is the first line of a v2 trace (charter Appendix P).
type V2Header struct {
	Magic              string `json:"magic"`
	Version            int    `json:"version"`
	Endianness         string `json:"endianness"`
	Tgid               uint32 `json:"tgid"`
	MinBlockNs         uint64 `json:"min_block_ns"`
	SampleMod          uint32 `json:"sample_mod"`
	StartedUTC         string `json:"started_utc"`
	KtimeBaseNs        uint64 `json:"ktime_base_ns,omitempty"`
	WallBaseUTC        string `json:"wall_base_utc,omitempty"`
	StructEventVersion int    `json:"struct_event_version"`
	TargetBinary       string `json:"target_binary,omitempty"`
}

// StacksSection maps BPF stack_id to program counters (deduped).
type StacksSection struct {
	Section string             `json:"_section"`
	Stacks  map[int32][]uint64 `json:"stacks"`
}

// ModulesSection stores /proc/pid/maps executable regions captured at record.
type ModulesSection struct {
	Section string `json:"_section"`
	Modules []Module `json:"modules"`
}

// Module is one executable mapping (mirrors symbolize.Module JSON).
type Module struct {
	Path    string `json:"path"`
	Start   uint64 `json:"start"`
	End     uint64 `json:"end"`
	BuildID string `json:"build_id,omitempty"`
}

// Footer holds userspace and BPF summary counters at end of trace.
type Footer struct {
	Section string `json:"_section"`

	UserspaceReceived   uint64 `json:"userspace_received,omitempty"`
	UserspaceChanDrops  uint64 `json:"userspace_chan_drops,omitempty"`
	UserspaceReadErrors uint64 `json:"userspace_read_errors,omitempty"`
	UserspaceMalformed  uint64 `json:"userspace_malformed,omitempty"`

	BPFRingbufDrops  uint64 `json:"bpf_ringbuf_drops,omitempty"`
	BPFEventsEmitted uint64 `json:"bpf_events_emitted,omitempty"`
	BPFBlocksSeen    uint64 `json:"bpf_blocks_seen,omitempty"`
	BPFPreempts      uint64 `json:"bpf_preempts,omitempty"`
	BPFRunQClosed    uint64 `json:"bpf_runq_closed,omitempty"`
	BPFShortFiltered uint64 `json:"bpf_short_filtered,omitempty"`
	BPFSampledOut    uint64 `json:"bpf_sampled_out,omitempty"`
	BPFStackFail     uint64 `json:"bpf_stack_fail,omitempty"`
}

func hostEndian() string {
	var u uint16 = 0x0102
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, u)
	if b[0] == 0x02 {
		return "little"
	}
	return "big"
}

// headerFromV2 maps v2 header into v1-compatible Header for EventWallTime etc.
func headerFromV2(h V2Header) Header {
	return Header{
		Version:     Version2,
		Tgid:        h.Tgid,
		MinBlock:    h.MinBlockNs,
		SampleMod:   h.SampleMod,
		Started:     h.StartedUTC,
		KtimeBaseNs: h.KtimeBaseNs,
		WallBaseUTC:  h.WallBaseUTC,
		TargetBinary: h.TargetBinary,
	}
}

// isSectionLine returns true if line is a v2 section marker.
func isSectionLine(raw []byte) (section string, ok bool) {
	var probe struct {
		Section string `json:"_section"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil || probe.Section == "" {
		return "", false
	}
	return probe.Section, true
}

// structEventVersion is the on-disk struct event layout generation.
const structEventVersion = 1
