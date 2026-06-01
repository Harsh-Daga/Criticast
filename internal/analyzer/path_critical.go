package analyzer

import (
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/mechanism"
)

// handlerTemporalCriticalPath prefers temporal longest path, then handler dominant wait,
// then the heaviest labeled wait (inputs should already be window-clipped).
func handlerTemporalCriticalPath(
	collapsed []WaitEdge,
	handlerSeeds map[NodeID]struct{},
	tolNs uint64,
) CriticalPath {
	path := longestPathOrDominantHandlerWait(collapsed, handlerSeeds, tolNs)
	if len(path.Edges) > 0 {
		return path
	}
	path = dominantHandlerWait(collapsed, handlerSeeds)
	if len(path.Edges) > 0 {
		return path
	}
	return dominantWaitInEdges(collapsed)
}

// longestPathOrDominantHandlerWait uses temporal longest path; if that undercounts a
// single dominant labeled wait at the handler, prefer that edge (parallel waits in one
// handler window cannot all appear on one monotonic chain).
func longestPathOrDominantHandlerWait(edges []WaitEdge, handlerSeeds map[NodeID]struct{}, tolNs uint64) CriticalPath {
	path := LongestPathTemporal(edges, tolNs)
	if len(handlerSeeds) == 0 {
		return path
	}
	dom := dominantHandlerWait(edges, handlerSeeds)
	if dom.PathWeight > path.PathWeight {
		return dom
	}
	return path
}

func dominantHandlerWait(edges []WaitEdge, handlerSeeds map[NodeID]struct{}) CriticalPath {
	var best WaitEdge
	for _, e := range edges {
		if !edgeTouchesHandler(e, handlerSeeds) || !isDominantWaitEdge(e) {
			continue
		}
		if e.BlockedNs > best.BlockedNs {
			best = e
		}
	}
	if best.BlockedNs == 0 {
		return CriticalPath{}
	}
	return CriticalPath{
		Edges:      []PathEdge{{WaitEdge: best}},
		PathWeight: best.BlockedNs,
	}
}

func edgeTouchesHandler(e WaitEdge, handlerSeeds map[NodeID]struct{}) bool {
	if _, ok := handlerSeeds[e.To]; ok {
		return true
	}
	if _, ok := handlerSeeds[e.From]; ok {
		return true
	}
	return false
}

func isDominantWaitEdge(e WaitEdge) bool {
	switch e.Meta.Mechanism {
	case mechanism.ChanWorkHandoff, mechanism.ConnPool, mechanism.Mutex:
		return true
	}
	switch e.WaitClass {
	case event.WCChan, event.WCMutex, event.WCNet, event.WCFutex:
		return true
	default:
		return e.Meta.Mechanism != "" && e.Meta.Mechanism != mechanism.Unknown
	}
}

func dominantWaitInEdges(edges []WaitEdge) CriticalPath {
	var best WaitEdge
	for _, e := range edges {
		if !isDominantWaitEdge(e) && e.BlockedNs > best.BlockedNs {
			if best.BlockedNs == 0 {
				best = e
			}
			continue
		}
		if e.BlockedNs > best.BlockedNs {
			best = e
		}
	}
	if best.BlockedNs == 0 {
		for _, e := range edges {
			if e.BlockedNs > best.BlockedNs {
				best = e
			}
		}
	}
	if best.BlockedNs == 0 {
		return CriticalPath{}
	}
	return CriticalPath{
		Edges:      []PathEdge{{WaitEdge: best}},
		PathWeight: best.BlockedNs,
	}
}
