package analyzer

import (
	"sort"
)

// PathEdge is one edge on the critical path with display metadata.
type PathEdge struct {
	WaitEdge
}

// CriticalPath is the longest weighted wait-for path after SCC collapse.
type CriticalPath struct {
	Edges      []PathEdge
	PathWeight uint64
	WallNs     uint64
}

// LongestPath computes the maximum blocked-ns path on a DAG (post-SCC).
func LongestPath(edges []WaitEdge) CriticalPath {
	if len(edges) == 0 {
		return CriticalPath{}
	}
	type adjEdge struct {
		to NodeID
		ew WaitEdge
	}
	adj := make(map[NodeID][]adjEdge)
	inDeg := make(map[NodeID]int)
	nodes := make(map[NodeID]struct{})
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], adjEdge{to: e.To, ew: e})
		nodes[e.From] = struct{}{}
		nodes[e.To] = struct{}{}
		inDeg[e.To]++
		if _, ok := inDeg[e.From]; !ok {
			inDeg[e.From] = 0
		}
	}
	for from := range adj {
		sort.Slice(adj[from], func(i, j int) bool {
			if adj[from][i].to != adj[from][j].to {
				return adj[from][i].to < adj[from][j].to
			}
			return adj[from][i].ew.Key < adj[from][j].ew.Key
		})
	}

	// Kahn topological order.
	var queue []NodeID
	for n := range nodes {
		if inDeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	var order []NodeID
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		order = append(order, u)
		for _, ae := range adj[u] {
			inDeg[ae.to]--
			if inDeg[ae.to] == 0 {
				queue = append(queue, ae.to)
			}
		}
	}
	if len(order) != len(nodes) {
		// Cycle remnant: fall back to bounded relaxation.
		return longestPathRelax(edges)
	}

	dist := make(map[NodeID]uint64)
	pred := make(map[NodeID]WaitEdge)
	for _, u := range order {
		for _, ae := range adj[u] {
			cand := dist[u] + ae.ew.BlockedNs
			if cand > dist[ae.to] || (cand == dist[ae.to] && tieBreakWaitEdge(ae.ew, pred[ae.to])) {
				dist[ae.to] = cand
				pred[ae.to] = ae.ew
			}
		}
	}
	return rebuildPath(pred, dist)
}

func longestPathRelax(edges []WaitEdge) CriticalPath {
	nodes := make(map[NodeID]struct{})
	for _, e := range edges {
		nodes[e.From] = struct{}{}
		nodes[e.To] = struct{}{}
	}
	best := make(map[NodeID]uint64)
	pred := make(map[NodeID]WaitEdge)
	for n := range nodes {
		best[n] = 0
	}
	maxIter := len(nodes)
	if maxIter < 1 {
		maxIter = 1
	}
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, e := range edges {
			if best[e.From]+e.BlockedNs > best[e.To] {
				best[e.To] = best[e.From] + e.BlockedNs
				pred[e.To] = e
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return rebuildPath(pred, best)
}

// tieBreakWaitEdge picks a deterministic predecessor on equal blocked_ns weight.
func tieBreakWaitEdge(a, b WaitEdge) bool {
	if b.Key == "" {
		return true
	}
	if a.Key == "" {
		return false
	}
	return a.Key < b.Key
}

func maxDistNode(dist map[NodeID]uint64) (NodeID, uint64) {
	var nodes []NodeID
	for n := range dist {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	var end NodeID
	var maxW uint64
	for _, n := range nodes {
		if dist[n] >= maxW {
			maxW = dist[n]
			end = n
		}
	}
	return end, maxW
}

// PathWeightInvariantOK checks path_weight ≤ wall + slack (Bar B / temporal path law).
func PathWeightInvariantOK(path CriticalPath, wallNs uint64, slackNs uint64) bool {
	if wallNs == 0 {
		return path.PathWeight == 0
	}
	return path.PathWeight <= wallNs+slackNs
}
