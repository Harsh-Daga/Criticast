package attribution

import (
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

// maxPlausibleGoid filters bogus task_id values (wrong offset or stale pointer bits).
const maxPlausibleGoid = uint64(1 << 30)

func plausibleGoid(id uint64) bool {
	return id > 0 && id <= maxPlausibleGoid
}

// TraceEdge is a wait-for edge from BPF with goid fields when casgstatus is attached.
type TraceEdge struct {
	WakeeGoid uint64
	WakerGoid uint64
	BlockedNs uint64
	WaitClass event.WaitClass
	Ts        time.Time
	SudogElem uint64
}

// EdgesFromTrace extracts block-end edges with task_id as goid.
// hdr correlates bpf_ktime_get_ns to wall time for ground-truth join.
func EdgesFromTrace(hdr trace.Header, events []event.Event) []TraceEdge {
	var out []TraceEdge
	for _, ev := range events {
		if ev.Type != event.EVBlockEnd {
			continue
		}
		if !plausibleGoid(ev.TaskID) {
			continue
		}
		out = append(out, TraceEdge{
			WakeeGoid: ev.TaskID,
			WakerGoid: ev.WakerTaskID,
			BlockedNs: ev.BlockedNs,
			WaitClass: ev.WaitClass,
			Ts:        hdr.EventWallTime(ev.TsNs),
			SudogElem: ev.Aux,
		})
	}
	return out
}

// TraceJoinStats counts trace edges before/after GT labeling (diagnostics).
type TraceJoinStats struct {
	BlockEnds       int
	WithGoid        int
	Labeled         int
	ClockCorrelated bool
}

// JoinStatsFromTrace reports why trace join may yield zero labeled edges.
func JoinStatsFromTrace(hdr trace.Header, events []event.Event, tl *groundtruth.Timeline) TraceJoinStats {
	var st TraceJoinStats
	st.ClockCorrelated = hdr.KtimeBaseNs != 0 && hdr.WallBaseUTC != ""
	for _, ev := range events {
		if ev.Type != event.EVBlockEnd {
			continue
		}
		st.BlockEnds++
		if !plausibleGoid(ev.TaskID) {
			continue
		}
		st.WithGoid++
		if tl.TokenAt(ev.TaskID, hdr.EventWallTime(ev.TsNs)) != "" {
			st.Labeled++
		}
	}
	return st
}

// LabelTraceEdges joins trace edges to GT timeline for gold wakee tokens.
func LabelTraceEdges(trace []TraceEdge, tl *groundtruth.Timeline) ([]Label, []TraceEdge) {
	var gold []Label
	var edges []TraceEdge
	for _, te := range trace {
		tok := tl.TokenAt(te.WakeeGoid, te.Ts)
		if tok == "" {
			continue
		}
		gold = append(gold, Label{WakeeToken: tok})
		edges = append(edges, te)
	}
	return gold, edges
}
