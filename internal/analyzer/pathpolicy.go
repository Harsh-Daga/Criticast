package analyzer

import (
	"github.com/criticast/criticast/internal/event"
)

// RuntimeSentinelNode is the bootstrap/runtime goid seen on unscoped idle paths (P1 smoke).
const RuntimeSentinelNode NodeID = 1

// PathPolicy selects edges eligible for longest-path analysis (charter E / P2).
type PathPolicy struct {
	DropSelfLoops       bool
	DropRunQ            bool
	DropRuntimeSentinel bool
}

// PathPolicyForScope returns defaults: unscoped mode keeps scheduler edges in dominant
// waits but drops them from the critical-path DAG; scoped mode keeps run-queue edges
// that match the request.
func PathPolicyForScope(scoped bool) PathPolicy {
	return PathPolicy{
		DropSelfLoops:       true,
		DropRunQ:            !scoped,
		DropRuntimeSentinel: !scoped,
	}
}

// FilterPathCandidates removes edges that must not dominate an unscoped headline path.
func FilterPathCandidates(edges []WaitEdge, pol PathPolicy) []WaitEdge {
	if !pol.DropSelfLoops && !pol.DropRunQ && !pol.DropRuntimeSentinel {
		return edges
	}
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		if pol.DropSelfLoops && e.From == e.To {
			continue
		}
		if pol.DropRunQ && e.WaitClass == event.WCRunQ {
			continue
		}
		if pol.DropRuntimeSentinel && isRuntimeSentinelEdge(e) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// FilterScopedSubgraph keeps edges on the wait-for component for this request.
// When handlerSeeds is set (Bar B), reachability is rooted at the handler task only so
// pooled-worker cliques from neighboring requests are not chained into one path.
func FilterScopedSubgraph(edges []WaitEdge, env scopeEnv) []WaitEdge {
	if !env.scoped || len(edges) == 0 {
		return edges
	}
	seeds := env.handlerSeeds
	if len(seeds) == 0 {
		seeds = make(map[NodeID]struct{})
		for _, e := range edges {
			if env.matchesEdge(e) {
				seeds[e.To] = struct{}{}
				seeds[e.From] = struct{}{}
			}
		}
	}
	if len(seeds) == 0 {
		return edges
	}
	fwd, rev := waitAdjacency(edges)
	reach := bidirectionalReach(seeds, fwd, rev)
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		if reach[e.From] && reach[e.To] {
			out = append(out, e)
		}
	}
	return out
}

func waitAdjacency(edges []WaitEdge) (fwd, rev map[NodeID][]NodeID) {
	fwd = make(map[NodeID][]NodeID)
	rev = make(map[NodeID][]NodeID)
	for _, e := range edges {
		fwd[e.From] = append(fwd[e.From], e.To)
		rev[e.To] = append(rev[e.To], e.From)
	}
	return fwd, rev
}

func bidirectionalReach(seeds map[NodeID]struct{}, fwd, rev map[NodeID][]NodeID) map[NodeID]bool {
	seen := make(map[NodeID]bool, len(seeds))
	queue := make([]NodeID, 0, len(seeds))
	for n := range seeds {
		seen[n] = true
		queue = append(queue, n)
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, m := range fwd[n] {
			if !seen[m] {
				seen[m] = true
				queue = append(queue, m)
			}
		}
		for _, m := range rev[n] {
			if !seen[m] {
				seen[m] = true
				queue = append(queue, m)
			}
		}
	}
	return seen
}

func isRuntimeSentinelEdge(e WaitEdge) bool {
	return e.From == RuntimeSentinelNode && e.To == RuntimeSentinelNode
}
