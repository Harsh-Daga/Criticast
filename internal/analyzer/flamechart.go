package analyzer

import "sort"

// RankedWait is one aggregated wait for flamechart-style output.
type RankedWait struct {
	WaitEdge
	Count int
}

// AggregateDominantWaits ranks edges by total blocked time (charter E.6).
func AggregateDominantWaits(edges []WaitEdge, topN int) []RankedWait {
	type key struct {
		from, to NodeID
		wc       uint8
	}
	agg := make(map[key]*RankedWait)
	for _, e := range edges {
		k := key{from: e.From, to: e.To, wc: uint8(e.WaitClass)}
		rw, ok := agg[k]
		if !ok {
			rw = &RankedWait{WaitEdge: e, Count: 0}
			agg[k] = rw
		}
		rw.BlockedNs += e.BlockedNs
		rw.Count++
	}
	out := make([]RankedWait, 0, len(agg))
	for _, rw := range agg {
		out = append(out, *rw)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BlockedNs > out[j].BlockedNs })
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out
}
