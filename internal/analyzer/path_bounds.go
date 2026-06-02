package analyzer

// ClipBlockedNsToWindow returns the portion of a wait edge overlapping [kFrom, kTo] (bpf_ktime).
func ClipBlockedNsToWindow(e WaitEdge, kFrom, kTo uint64) uint64 {
	if kTo <= kFrom {
		return e.BlockedNs
	}
	start := e.StartNs
	if start == 0 && e.BlockedNs > 0 && e.EndNs >= e.BlockedNs {
		start = e.EndNs - e.BlockedNs
	}
	end := e.EndNs
	if end < kFrom || start > kTo {
		return 0
	}
	if start < kFrom {
		start = kFrom
	}
	if end > kTo {
		end = kTo
	}
	if end <= start {
		return 0
	}
	clipped := end - start
	if e.BlockedNs > 0 && clipped > e.BlockedNs {
		return e.BlockedNs
	}
	return clipped
}

// ApplyWindowClip rewrites BlockedNs on edges to the in-window overlap (Bar B path weight).
func ApplyWindowClip(edges []WaitEdge, kFrom, kTo uint64) []WaitEdge {
	if kTo <= kFrom {
		return edges
	}
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		w := ClipBlockedNsToWindow(e, kFrom, kTo)
		if w == 0 {
			continue
		}
		e.BlockedNs = w
		out = append(out, e)
	}
	return out
}
