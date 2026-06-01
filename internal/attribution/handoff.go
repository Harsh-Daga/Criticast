package attribution

import (
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
)

// ChanHandoff is one worker-pool item flight (send → recv) from GT.
type ChanHandoff struct {
	Token       string
	Elem        uint64
	HandlerGoid uint64
	WorkerGoid  uint64
	SendTS      time.Time
	RecvTS      time.Time
}

// BuildChanHandoffsBetween pairs send/recv only for records with TS in [from, to].
func BuildChanHandoffsBetween(records []groundtruth.Record, from, to time.Time) []ChanHandoff {
	if from.IsZero() && to.IsZero() {
		return BuildChanHandoffs(records)
	}
	filtered := make([]groundtruth.Record, 0, len(records)/4)
	for _, rec := range records {
		if rec.TS.Before(from) || rec.TS.After(to) {
			continue
		}
		filtered = append(filtered, rec)
	}
	return BuildChanHandoffs(filtered)
}

// BuildChanHandoffs pairs worker-pool-send with worker-recv by sudog elem id.
func BuildChanHandoffs(records []groundtruth.Record) []ChanHandoff {
	pending := make(map[uint64]*ChanHandoff)
	var out []ChanHandoff
	for _, rec := range records {
		elem := parseElem(rec.Extra)
		if elem == 0 {
			continue
		}
		switch rec.Site {
		case groundtruth.SiteWorkerPoolSend:
			pending[elem] = &ChanHandoff{
				Token:       rec.Token,
				Elem:        elem,
				HandlerGoid: rec.Goid,
				SendTS:      rec.TS,
			}
		case groundtruth.SiteWorkerRecv:
			h, ok := pending[elem]
			if !ok {
				continue
			}
			h.WorkerGoid = rec.Goid
			h.RecvTS = rec.TS
			out = append(out, *h)
			delete(pending, elem)
		}
	}
	return out
}

// TokenForChanHandoff labels a block-end on wakee at ts from paired GT chan flights.
func TokenForChanHandoff(handoffs []ChanHandoff, wakee uint64, ts time.Time) string {
	const postRecvSlack = 5 * time.Millisecond
	for _, h := range handoffs {
		if h.Token == "" {
			continue
		}
		if wakee == h.WorkerGoid && !ts.Before(h.SendTS) {
			if h.RecvTS.IsZero() || !ts.After(h.RecvTS.Add(postRecvSlack)) {
				return h.Token
			}
		}
		if wakee == h.HandlerGoid && !ts.Before(h.SendTS.Add(-2*time.Millisecond)) && ts.Before(h.RecvTS.Add(postRecvSlack)) {
			return h.Token
		}
	}
	return ""
}
