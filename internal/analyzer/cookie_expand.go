package analyzer

import (
	"time"

	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

// expandRequestGoidsByCookie adds shared worker trace task_ids reached by sudog.elem
// handoffs from GT-calibrated request goids (Bar B: pool workers are not in spawn tree).
func expandRequestGoidsByCookie(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	base map[uint64]struct{},
	from, to time.Time,
	kFrom, kTo uint64,
) map[uint64]struct{} {
	out := make(map[uint64]struct{}, len(base)+8)
	for g := range base {
		out[g] = struct{}{}
	}
	if tl == nil || token == "" {
		return out
	}
	lin := attribution.ReplayLineageFromTimeline(tl)
	pad := 50 * time.Millisecond
	handoffs := attribution.BuildChanHandoffsBetween(
		tl.AllRecords(), from.Add(-pad), to.Add(pad),
	)

	for _, h := range handoffs {
		if h.Token != token {
			continue
		}
		if h.SendTS.After(to) || h.SendTS.Before(from) {
			continue
		}
		if task := nearestTraceTaskAt(hdr, events, h.SendTS, gtTraceCalibSlack); task != 0 {
			out[task] = struct{}{}
		}
		if h.WorkerGoid != 0 {
			ts := h.RecvTS
			if ts.IsZero() {
				ts = h.SendTS
			}
			if task := nearestTraceTaskAt(hdr, events, ts, gtTraceCalibSlack); task != 0 {
				out[task] = struct{}{}
			}
		}
	}

	for _, ev := range events {
		if ev.Type != event.EVBlockEnd || ev.TaskID == 0 {
			continue
		}
		if kTo > kFrom && (ev.TsNs < kFrom || ev.TsNs > kTo) {
			continue
		}
		ts := hdr.EventWallTime(ev.TsNs)
		te := attribution.TraceEdge{
			WakeeGoid: ev.TaskID,
			Ts:        ts,
			SudogElem: ev.Aux,
		}
		tok := attribution.TokenForWakee(tl, lin, te)
		if tok == "" {
			tok = attribution.TokenForChanHandoff(handoffs, ev.TaskID, ts)
		}
		if tok != token {
			continue
		}
		out[ev.TaskID] = struct{}{}
		if ev.WakerTaskID != 0 && ev.WakerTaskID <= 1<<30 {
			out[ev.WakerTaskID] = struct{}{}
		}
	}
	return out
}
