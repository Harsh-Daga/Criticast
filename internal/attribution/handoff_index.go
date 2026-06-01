package attribution

import "time"

// ChanHandoffIndex indexes GT chan flights by goid for O(k) block-end lookup (not O(all handoffs)).
type ChanHandoffIndex struct {
	byGoid map[uint64][]ChanHandoff
}

// NewChanHandoffIndex builds a goid → handoff slice index.
func NewChanHandoffIndex(handoffs []ChanHandoff) *ChanHandoffIndex {
	idx := &ChanHandoffIndex{byGoid: make(map[uint64][]ChanHandoff)}
	for _, h := range handoffs {
		if h.HandlerGoid != 0 {
			idx.byGoid[h.HandlerGoid] = append(idx.byGoid[h.HandlerGoid], h)
		}
		if h.WorkerGoid != 0 {
			idx.byGoid[h.WorkerGoid] = append(idx.byGoid[h.WorkerGoid], h)
		}
	}
	return idx
}

// Token returns the request token for wakee at ts from indexed handoffs only.
func (idx *ChanHandoffIndex) Token(wakee uint64, ts time.Time) string {
	if idx == nil {
		return ""
	}
	return TokenForChanHandoff(idx.byGoid[wakee], wakee, ts)
}
