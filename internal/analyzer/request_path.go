package analyzer

import (
	"fmt"
)

// filterEpochEdges keeps waits for one handler request epoch (see docs/p2-bar-b-epoch-path.md).
func filterEpochEdges(allEdges []WaitEdge, env scopeEnv, epoch RequestEpoch) []WaitEdge {
	if epoch.KPaddedTo <= epoch.KPaddedFrom || len(env.requestGoids) == 0 {
		return nil
	}
	var pool []WaitEdge
	for _, e := range allEdges {
		if !edgeOverlapsKtime(e, epoch.KPaddedFrom, epoch.KPaddedTo) {
			continue
		}
		if epochEdgeAllowed(e, env, epoch.HandlerTask) {
			pool = append(pool, e)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	if len(env.handlerSeeds) == 0 {
		return pool
	}
	if fwd := filterEdgesReachableFromSeeds(pool, env.handlerSeeds, true); len(fwd) > 0 {
		return fwd
	}
	if inc := filterEdgesIncidentToSeeds(pool, env.handlerSeeds); len(inc) > 0 {
		return inc
	}
	return filterEdgesBothInRequestGoids(pool, env)
}

func epochEdgeAllowed(e WaitEdge, env scopeEnv, handlerTask uint64) bool {
	fromT, fOK := env.nodeTaskID(e.From)
	toT, tOK := env.nodeTaskID(e.To)
	if handlerTask != 0 && tOK && toT == handlerTask {
		return true
	}
	if handlerTask != 0 && fOK && fromT == handlerTask && tOK {
		if _, ok := env.requestGoids[toT]; ok {
			return true
		}
	}
	if !fOK || !tOK {
		return false
	}
	_, fin := env.requestGoids[fromT]
	_, tin := env.requestGoids[toT]
	return fin && tin
}

// computeEpochCriticalPath builds the epoch critical path; path_weight is from the temporal
// DP and clipped edge weights only — never derived from epoch wall (see pathWeightFromPathEdges).
func computeEpochCriticalPath(
	edges []WaitEdge,
	byTask map[TaskKey][]Segment,
	epoch RequestEpoch,
	handlerSeeds map[NodeID]struct{},
	minConf uint8,
) (CriticalPath, error) {
	pathEdges := FilterPathCandidates(edges, PathPolicyForScope(true))
	pathEdges = clipEdgesToEpochWindow(pathEdges, epoch.KPaddedFrom, epoch.KPaddedTo)
	if len(pathEdges) == 0 {
		return CriticalPath{}, fmt.Errorf("analyzer: no wait edges in handler epoch window")
	}

	kept, _ := FilterByConfidence(pathEdges, minConf)
	kept = PreparePathEdgesMax(kept)
	if len(kept) == 0 {
		return CriticalPath{}, fmt.Errorf("analyzer: no path-eligible edges after filter")
	}
	collapsed, _ := CollapseSCC(kept, SCC(nil, kept))

	path := handlerTemporalCriticalPath(collapsed, handlerSeeds, DefaultTemporalTolNs)
	if len(path.Edges) == 0 {
		path = handlerTemporalCriticalPath(kept, handlerSeeds, DefaultTemporalTolNs)
	}
	if len(path.Edges) == 0 {
		path = dominantHandlerWait(kept, handlerSeeds)
	}
	if len(path.Edges) == 0 {
		path = dominantWaitInEdges(kept)
	}
	if len(path.Edges) == 0 {
		return CriticalPath{}, fmt.Errorf("analyzer: could not derive critical path for request epoch")
	}

	for i := range path.Edges {
		path.Edges[i].BlockedNs = clipEdgeWeightForEpoch(
			path.Edges[i].WaitEdge, epoch.KStrictFrom, epoch.KStrictTo, epoch.KPaddedFrom, epoch.KPaddedTo,
		)
	}
	path.PathWeight = pathWeightFromPathEdges(path)
	if path.PathWeight == 0 {
		return CriticalPath{}, fmt.Errorf("analyzer: epoch path_weight is zero with %d path edges", len(path.Edges))
	}
	_ = byTask // occupancy from measured RUNNING segments is diagnostic only (probe script)
	return path, nil
}

func clipEdgesToEpochWindow(edges []WaitEdge, kFrom, kTo uint64) []WaitEdge {
	if kTo <= kFrom {
		return edges
	}
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		w := ClipBlockedNsToWindow(e, kFrom, kTo)
		if w == 0 {
			continue
		}
		e.BlockedNs = w
		out = append(out, e)
	}
	return out
}

// clipEdgeWeightForEpoch prefers strict-window overlap; falls back to padded when BPF/GT skew zeros strict.
func clipEdgeWeightForEpoch(e WaitEdge, strictFrom, strictTo, padFrom, padTo uint64) uint64 {
	if strictW := ClipBlockedNsToWindow(e, strictFrom, strictTo); strictW > 0 {
		return strictW
	}
	if padW := ClipBlockedNsToWindow(e, padFrom, padTo); padW > 0 {
		return padW
	}
	return e.BlockedNs
}

// pathWeightFromPathEdges sums observed blocked_ns on the critical path (no GT wall fill).
func pathWeightFromPathEdges(path CriticalPath) uint64 {
	var sum uint64
	for _, pe := range path.Edges {
		sum += pe.BlockedNs
	}
	return sum
}

// measuredHandlerOccupancyNs is BPF-observed busy time (blocked+runnable+running union) in a window.
// Used by bar-b-probe-epoch.sh; not added to path_weight.
func measuredHandlerOccupancyNs(byTask map[TaskKey][]Segment, handlerTask uint64, kFrom, kTo uint64) uint64 {
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
			if en > st {
				clipped = append(clipped, Segment{Start: st, End: en})
			}
		}
	}
	return mergedIntervalNs(clipped)
}

func edgeOverlapsKtime(e WaitEdge, kFrom, kTo uint64) bool {
	start, end := edgeInterval(e)
	if end == 0 && e.BlockedNs > 0 {
		end = start + e.BlockedNs
	}
	return end >= kFrom && start <= kTo
}

func mergedIntervalNs(segs []Segment) uint64 {
	if len(segs) == 0 {
		return 0
	}
	ivs := mergeTimeIntervals(segs)
	var total uint64
	for _, iv := range ivs {
		total += iv.end - iv.start
	}
	return total
}
