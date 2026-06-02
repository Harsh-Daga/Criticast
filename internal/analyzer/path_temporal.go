package analyzer

// DefaultTemporalTolNs allows sub-microsecond clock ordering jitter between edges.
const DefaultTemporalTolNs = 1000

// edgeInterval returns [start, end) bpf_ktime for a wait edge.
func edgeInterval(e WaitEdge) (start, end uint64) {
	start = e.StartNs
	end = e.EndNs
	if end == 0 && e.BlockedNs > 0 {
		end = start + e.BlockedNs
	}
	if start == 0 && e.BlockedNs > 0 && end >= e.BlockedNs {
		start = end - e.BlockedNs
	}
	if end < start {
		end = start + e.BlockedNs
	}
	return start, end
}

// LongestPathTemporal is longest blocked-ns path where waits are non-overlapping in time:
// edge e follows predecessor u only if e.StartNs >= predEnd[u] - tolNs.
func LongestPathTemporal(edges []WaitEdge, tolNs uint64) CriticalPath {
	if len(edges) == 0 {
		return CriticalPath{}
	}
	return longestPathTemporalDAG(edges, tolNs)
}

func longestPathTemporalDAG(edges []WaitEdge, tolNs uint64) CriticalPath {
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
	var queue []NodeID
	for n := range nodes {
		if inDeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	sortNodes(queue)
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
		return longestPathTemporalRelax(edges, tolNs)
	}

	dist := make(map[NodeID]uint64)
	predEnd := make(map[NodeID]uint64)
	pred := make(map[NodeID]WaitEdge)
	for _, u := range order {
		for _, ae := range adj[u] {
			start, end := edgeInterval(ae.ew)
			if end > 0 && predEnd[u] > 0 && start+tolNs < predEnd[u] {
				continue
			}
			cand := dist[u] + ae.ew.BlockedNs
			if cand > dist[ae.to] || (cand == dist[ae.to] && tieBreakWaitEdge(ae.ew, pred[ae.to])) {
				dist[ae.to] = cand
				pred[ae.to] = ae.ew
				if end > predEnd[ae.to] {
					predEnd[ae.to] = end
				}
			}
		}
	}
	return rebuildPath(pred, dist)
}

func longestPathTemporalRelax(edges []WaitEdge, tolNs uint64) CriticalPath {
	nodes := make(map[NodeID]struct{})
	for _, e := range edges {
		nodes[e.From] = struct{}{}
		nodes[e.To] = struct{}{}
	}
	best := make(map[NodeID]uint64)
	predEnd := make(map[NodeID]uint64)
	pred := make(map[NodeID]WaitEdge)
	maxIter := len(nodes)
	if maxIter < 1 {
		maxIter = 1
	}
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, e := range edges {
			start, end := edgeInterval(e)
			if predEnd[e.From] > 0 && start+tolNs < predEnd[e.From] {
				continue
			}
			if best[e.From]+e.BlockedNs > best[e.To] {
				best[e.To] = best[e.From] + e.BlockedNs
				pred[e.To] = e
				if end > predEnd[e.To] {
					predEnd[e.To] = end
				}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return rebuildPath(pred, best)
}

func rebuildPath(pred map[NodeID]WaitEdge, dist map[NodeID]uint64) CriticalPath {
	end, maxW := maxDistNode(dist)
	var path []PathEdge
	seen := make(map[NodeID]struct{})
	for {
		e, ok := pred[end]
		if !ok {
			break
		}
		if _, loop := seen[end]; loop {
			break
		}
		seen[end] = struct{}{}
		path = append([]PathEdge{{WaitEdge: e}}, path...)
		end = e.From
	}
	var wall uint64
	for _, pe := range path {
		if pe.EndNs > wall {
			wall = pe.EndNs
		}
	}
	return CriticalPath{Edges: path, PathWeight: maxW, WallNs: wall}
}

func sortNodes(ids []NodeID) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
}
