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
	lin := replayLineage(tl)
	idx := NewChanHandoffIndex(BuildChanHandoffs(tl.AllRecords()))
	return joinStats(hdr, events, tl, lin, idx)
}

func joinStats(hdr trace.Header, events []event.Event, tl *groundtruth.Timeline, lin *LineageStore, handoffIdx *ChanHandoffIndex) TraceJoinStats {
	var st TraceJoinStats
	st.ClockCorrelated = hdr.KtimeBaseNs != 0 && hdr.WallBaseUTC != ""
	if !st.ClockCorrelated {
		return st
	}
	for _, ev := range events {
		if ev.Type != event.EVBlockEnd {
			continue
		}
		st.BlockEnds++
		if !plausibleGoid(ev.TaskID) {
			continue
		}
		st.WithGoid++
		te := TraceEdge{
			WakeeGoid: ev.TaskID,
			Ts:        hdr.EventWallTime(ev.TsNs),
			SudogElem: ev.Aux,
		}
		if tokenForWakee(tl, lin, handoffIdx, te) != "" {
			st.Labeled++
		}
	}
	return st
}

// ReplayLineageFromTimeline rebuilds sudog + spawn lineage from GT order (Bar B / eval join).
func ReplayLineageFromTimeline(tl *groundtruth.Timeline) *LineageStore {
	return replayLineage(tl)
}

func replayLineage(tl *groundtruth.Timeline) *LineageStore {
	if tl == nil {
		return nil
	}
	lin := NewLineageStore(0)
	for _, rec := range tl.AllRecords() {
		lin.ApplyRecord(rec)
	}
	return lin
}

// TokenForWakee resolves request token for a block-end wakee (GT site, sudog.elem, spawn).
func TokenForWakee(tl *groundtruth.Timeline, lin *LineageStore, te TraceEdge) string {
	var idx *ChanHandoffIndex
	if tl != nil {
		idx = NewChanHandoffIndex(BuildChanHandoffs(tl.AllRecords()))
	}
	return tokenForWakee(tl, lin, idx, te)
}

func tokenForWakee(tl *groundtruth.Timeline, lin *LineageStore, handoffIdx *ChanHandoffIndex, te TraceEdge) string {
	// Sudog.elem wins over per-goid TokenAt — shared workers accumulate stale tokens (P0-B pool).
	if lin != nil && te.SudogElem != 0 {
		if tok := lin.SudogToken(te.SudogElem); tok != "" {
			return tok
		}
	}
	if handoffIdx != nil {
		if tok := handoffIdx.Token(te.WakeeGoid, te.Ts); tok != "" {
			return tok
		}
	}
	if tl != nil {
		if tok := tl.TokenAt(te.WakeeGoid, te.Ts); tok != "" {
			return tok
		}
	}
	if lin != nil {
		if tok := lin.Cookie(te.WakeeGoid, te.Ts); tok != "" {
			return tok
		}
	}
	return ""
}

// LabelTraceEdges joins trace edges to GT timeline for gold wakee tokens.
func LabelTraceEdges(trace []TraceEdge, hdr trace.Header, tl *groundtruth.Timeline) ([]Label, []TraceEdge) {
	if hdr.KtimeBaseNs == 0 || hdr.WallBaseUTC == "" {
		return nil, nil
	}
	lin := replayLineage(tl)
	handoffIdx := NewChanHandoffIndex(BuildChanHandoffs(tl.AllRecords()))
	var gold []Label
	var edges []TraceEdge
	for _, te := range trace {
		tok := tokenForWakee(tl, lin, handoffIdx, te)
		if tok == "" {
			continue
		}
		gold = append(gold, Label{WakeeToken: tok})
		edges = append(edges, te)
	}
	return gold, edges
}
