package trace

import "time"

func (h Header) wallBase() (time.Time, bool) {
	if h.KtimeBaseNs == 0 || h.WallBaseUTC == "" {
		return time.Time{}, false
	}
	base, err := time.Parse(time.RFC3339Nano, h.WallBaseUTC)
	if err != nil {
		base, err = time.Parse(time.RFC3339, h.WallBaseUTC)
		if err != nil {
			return time.Time{}, false
		}
	}
	return base.UTC(), true
}

// WallToKtime maps a wall-clock instant to bpf_ktime_get_ns using the record anchor.
func (h Header) WallToKtime(t time.Time) (uint64, bool) {
	base, ok := h.wallBase()
	if !ok {
		return 0, false
	}
	delta := t.UTC().Sub(base)
	if delta < 0 {
		return 0, false
	}
	return h.KtimeBaseNs + uint64(delta.Nanoseconds()), true
}

// KtimeToWall maps bpf_ktime_get_ns to wall clock (inverse of WallToKtime).
func (h Header) KtimeToWall(tsNs uint64) (time.Time, bool) {
	base, ok := h.wallBase()
	if !ok {
		return time.Time{}, false
	}
	if tsNs < h.KtimeBaseNs {
		return time.Time{}, false
	}
	return base.Add(time.Duration(int64(tsNs - h.KtimeBaseNs))), true
}
