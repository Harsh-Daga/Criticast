package analyzer

import (
	"fmt"

	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
)

// WaitEdge is a directed wait-for edge (waker → wakee) weighted by blocked_ns.
type WaitEdge struct {
	From       NodeID
	To         NodeID
	BlockedNs  uint64
	StartNs    uint64
	EndNs      uint64
	WaitClass  event.WaitClass
	Cookie     uint64
	Tid        uint32
	WakerTid   uint32
	Aux        uint64
	StackID    int32
	WakerStkID int32
	Key        string
	Meta       attribution.TraceEdgeMeta
}

// BuildWaitEdges extracts block-end wait-for edges from the event stream.
func BuildWaitEdges(events []event.Event) []WaitEdge {
	var out []WaitEdge
	for _, ev := range events {
		if ev.Type != event.EVBlockEnd || ev.BlockedNs == 0 {
			continue
		}
		wakee := NodeFromEvent(ev)
		wakerTask := ev.WakerTaskID
		if wakerTask == 0 {
			wakerTask = uint64(ev.WakerTid)
		}
		if wakerTask == 0 {
			continue
		}
		waker := NodeFromTask(wakerTask, ev.WakerTid)
		meta := attribution.AttributeTraceEdge(ev.WaitClass, ev.Aux, ev.Cookie)
		e := WaitEdge{
			From:       waker,
			To:         wakee,
			BlockedNs:  ev.BlockedNs,
			StartNs:    ev.TsNs - ev.BlockedNs,
			EndNs:      ev.TsNs,
			WaitClass:  ev.WaitClass,
			Cookie:     ev.Cookie,
			Tid:        ev.Tid,
			WakerTid:   ev.WakerTid,
			Aux:        ev.Aux,
			StackID:    ev.StackID,
			WakerStkID: ev.WakerStackID,
			Key:        fmt.Sprintf("%d->%d", waker, wakee),
			Meta:       meta,
		}
		out = append(out, e)
	}
	return out
}

// FilterByScope keeps edges matching cookie or tid when scoped.
func FilterByScope(edges []WaitEdge, scopeCookie uint64, scopeTid uint32, scoped bool) []WaitEdge {
	if !scoped {
		return edges
	}
	var out []WaitEdge
	for _, e := range edges {
		if MatchesScope(e.Cookie, e.Tid, scopeCookie, scopeTid, true) {
			out = append(out, e)
		}
	}
	return out
}

// FilterByConfidence splits edges into path-eligible vs ambiguous/low-confidence buckets.
func FilterByConfidence(edges []WaitEdge, minConf uint8) (kept, ambiguous []WaitEdge) {
	for _, e := range edges {
		if e.Meta.Ambiguous || e.Meta.Confidence < minConf {
			ambiguous = append(ambiguous, e)
			continue
		}
		kept = append(kept, e)
	}
	return kept, ambiguous
}
