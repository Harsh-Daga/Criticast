package analyzer

// DefaultSpuriousWakeNs is charter E.4 default (10µs).
const DefaultSpuriousWakeNs = 10_000

// FilterSpuriousWakeups drops edges where the wakee re-blocked within ε with same wait_class.
func FilterSpuriousWakeups(edges []WaitEdge, byTask map[TaskKey][]Segment, epsNs uint64) []WaitEdge {
	if epsNs == 0 || len(byTask) == 0 {
		return edges
	}
	var out []WaitEdge
	for _, e := range edges {
		if spuriousWake(e, byTask[TaskKeyFromNode(e.To, e.Tid)], epsNs) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func spuriousWake(e WaitEdge, segs []Segment, epsNs uint64) bool {
	if len(segs) == 0 {
		return false
	}
	var cur *Segment
	for i := range segs {
		if segs[i].Kind == Blocked && segs[i].End == e.EndNs {
			cur = &segs[i]
			break
		}
	}
	if cur == nil {
		return false
	}
	for i := range segs {
		s := &segs[i]
		if s.Kind != Blocked || s.Start < cur.End {
			continue
		}
		if s.Start-cur.End < epsNs && s.WaitClass == cur.WaitClass {
			return true
		}
		break
	}
	return false
}
