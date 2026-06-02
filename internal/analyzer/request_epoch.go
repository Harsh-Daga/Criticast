package analyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

// maxEpochMembership rejects GT epochs polluted by concurrent handlers (p0b @ ~500 rps).
const maxEpochMembership = 16

// RequestEpoch is the GT handler entry→exit window for Bar B literal analysis.
type RequestEpoch struct {
	WallFrom, WallTo       time.Time
	KStrictFrom, KStrictTo uint64
	KPaddedFrom, KPaddedTo uint64
	WallNs                 uint64
	HandlerGoid            uint64
	HandlerTask            uint64
}

func buildRequestEpoch(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	handlerGoid uint64,
	winFrom, winTo time.Time,
	pad time.Duration,
) (RequestEpoch, map[uint64]struct{}, map[NodeID]struct{}, error) {
	if handlerGoid == 0 || winFrom.IsZero() || winTo.IsZero() {
		return RequestEpoch{}, nil, nil, fmt.Errorf("analyzer: request epoch requires handler goid and scope window")
	}
	kPadFrom, kPadTo, ok := applyScopeWindow(hdr, winFrom, winTo, pad)
	if !ok {
		return RequestEpoch{}, nil, nil, fmt.Errorf("analyzer: trace missing wall_base_utc/ktime_base_ns for scope window")
	}
	kStrictFrom, kStrictTo, ok := applyScopeWindowStrict(hdr, winFrom, winTo)
	if !ok || kStrictTo <= kStrictFrom {
		return RequestEpoch{}, nil, nil, fmt.Errorf("analyzer: invalid strict handler window")
	}
	epoch := RequestEpoch{
		WallFrom:    winFrom,
		WallTo:      winTo,
		KStrictFrom: kStrictFrom,
		KStrictTo:   kStrictTo,
		KPaddedFrom: kPadFrom,
		KPaddedTo:   kPadTo,
		WallNs:      kStrictTo - kStrictFrom,
		HandlerGoid: handlerGoid,
	}
	pinned := map[uint64]struct{}{handlerGoid: {}}
	epoch.HandlerTask = handlerTraceTaskID(hdr, events, tl, token, handlerGoid, winFrom)
	requestGoids := calibrateRequestGoids(hdr, events, tl, token, pinned, winFrom, winTo)
	if handlerGoid != 0 {
		requestGoids[handlerGoid] = struct{}{}
	}
	if epoch.HandlerTask != 0 {
		requestGoids[epoch.HandlerTask] = struct{}{}
	}
	requestGoids = expandEpochMembershipFromHandlerWakeups(
		events, epoch.HandlerTask, kStrictFrom, kStrictTo, requestGoids,
	)
	if len(requestGoids) > maxEpochMembership {
		return RequestEpoch{}, nil, nil, fmt.Errorf(
			"analyzer: epoch membership %d tasks exceeds %d (concurrent handler pollution?)",
			len(requestGoids), maxEpochMembership,
		)
	}
	var handlerSeeds map[NodeID]struct{}
	if epoch.HandlerTask != 0 {
		handlerSeeds = map[NodeID]struct{}{NodeFromTask(epoch.HandlerTask, 0): {}}
	}
	return epoch, requestGoids, handlerSeeds, nil
}

func isRequestEpochLiteral(opts Options, scope RequestScope, winFrom time.Time) bool {
	return opts.ScopeHandlerGoid != 0 && scope.Token != "" && !winFrom.IsZero()
}

// handlerTraceTaskID maps a GT handler goid to the BPF task_id at handler-entry.
func handlerTraceTaskID(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	handlerGoid uint64,
	entry time.Time,
) uint64 {
	if handlerGoid != 0 && tl != nil && token != "" {
		for _, rec := range tl.AllRecords() {
			if rec.Token != token || rec.Goid != handlerGoid {
				continue
			}
			if rec.Site != groundtruth.SiteHandlerEntry {
				continue
			}
			if task := nearestTraceTaskAt(hdr, events, rec.TS, gtTraceCalibSlack); task != 0 {
				return task
			}
		}
	}
	if task := nearestTraceTaskAt(hdr, events, entry, gtTraceCalibSlack); task != 0 {
		return task
	}
	for _, ev := range events {
		if ev.TaskID == handlerGoid {
			return handlerGoid
		}
	}
	return 0
}

// handlerTimelineWeightNs is occupied handler time in [kFrom, kTo] from BPF segments + gaps (E.5).
func handlerTimelineWeightNs(byTask map[TaskKey][]Segment, handlerTask uint64, kFrom, kTo uint64) uint64 {
	if kTo <= kFrom || handlerTask == 0 {
		return 0
	}
	var clipped []Segment
	for k, segs := range byTask {
		if k.TaskID != handlerTask {
			continue
		}
		for _, s := range segs {
			if s.End <= kFrom || s.Start >= kTo {
				continue
			}
			st, en := s.Start, s.End
			if st < kFrom {
				st = kFrom
			}
			if en > kTo {
				en = kTo
			}
			if en <= st {
				continue
			}
			clipped = append(clipped, Segment{Start: st, End: en, Kind: s.Kind})
		}
	}
	if len(clipped) == 0 {
		return 0
	}
	merged := mergeTimeIntervals(clipped)
	var total uint64
	cur := kFrom
	for _, iv := range merged {
		if iv.start > cur {
			total += iv.start - cur
		}
		total += iv.end - iv.start
		if iv.end > cur {
			cur = iv.end
		}
	}
	if kTo > cur {
		total += kTo - cur
	}
	if cap := kTo - kFrom; total > cap {
		return cap
	}
	return total
}

type timeInterval struct {
	start, end uint64
}

func mergeTimeIntervals(segs []Segment) []timeInterval {
	if len(segs) == 0 {
		return nil
	}
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].Start != segs[j].Start {
			return segs[i].Start < segs[j].Start
		}
		return segs[i].End < segs[j].End
	})
	out := []timeInterval{{start: segs[0].Start, end: segs[0].End}}
	for i := 1; i < len(segs); i++ {
		last := &out[len(out)-1]
		if segs[i].Start <= last.end {
			if segs[i].End > last.end {
				last.end = segs[i].End
			}
			continue
		}
		out = append(out, timeInterval{start: segs[i].Start, end: segs[i].End})
	}
	return out
}
