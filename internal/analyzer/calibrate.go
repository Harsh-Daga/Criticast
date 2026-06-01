package analyzer

import (
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

const gtTraceCalibSlack = 3 * time.Millisecond

// nearestTraceTaskAt finds the trace event task_id closest in time to a wall-clock instant.
func nearestTraceTaskAt(hdr trace.Header, events []event.Event, ts time.Time, slack time.Duration) uint64 {
	target, ok := hdr.WallToKtime(ts.UTC())
	if !ok {
		return 0
	}
	maxNs := uint64(slack.Nanoseconds())
	var best uint64
	var bestDelta uint64
	for _, ev := range events {
		if ev.TaskID == 0 {
			continue
		}
		var d uint64
		if ev.TsNs >= target {
			d = ev.TsNs - target
		} else {
			d = target - ev.TsNs
		}
		if d > maxNs {
			continue
		}
		if best == 0 || d < bestDelta {
			bestDelta = d
			best = ev.TaskID
		}
	}
	return best
}

// calibrateRequestGoids maps each GT site in the handler span to a BPF task_id (nearest in time).
func calibrateRequestGoids(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	gtGoids map[uint64]struct{},
	from, to time.Time,
) map[uint64]struct{} {
	if tl == nil || token == "" {
		return nil
	}
	out := make(map[uint64]struct{})
	for _, rec := range tl.RecordsBetween(from.UTC(), to.UTC()) {
		if rec.Token != token || rec.Goid == 0 {
			continue
		}
		if len(gtGoids) > 0 {
			if _, ok := gtGoids[rec.Goid]; !ok {
				continue
			}
		}
		if task := nearestTraceTaskAt(hdr, events, rec.TS, gtTraceCalibSlack); task != 0 {
			out[task] = struct{}{}
		}
	}
	return out
}

// buildTraceWaitClassFromGT maps trace task_id → wait_class from GT sites in the span.
func buildTraceWaitClassFromGT(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	from, to time.Time,
) map[uint64]event.WaitClass {
	if tl == nil || token == "" {
		return nil
	}
	out := make(map[uint64]event.WaitClass)
	for _, rec := range tl.RecordsBetween(from.UTC(), to.UTC()) {
		if rec.Token != token {
			continue
		}
		task := nearestTraceTaskAt(hdr, events, rec.TS, gtTraceCalibSlack)
		if task == 0 {
			continue
		}
		wc := waitClassForMechanism(groundtruth.Mechanism(rec.Site))
		if wc == event.WCUnknown {
			continue
		}
		out[task] = wc
	}
	return out
}
