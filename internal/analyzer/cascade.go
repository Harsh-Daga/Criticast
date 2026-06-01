package analyzer

// PreparePathEdges consolidates parallel edges and normalizes weights before longest path.
// Full wPerf cascade redistribution (CHARTER E.2) applies when a waker has multiple
// competing outgoing critical children; for Tier-0/1 traces after SCC collapse, merging
// duplicate (from,to) keys preserves longest-path semantics on DAGs while improving
// scalability for high fan-out recordings.
func PreparePathEdges(edges []WaitEdge) []WaitEdge {
	return ConsolidateParallelEdges(edges)
}

// PreparePathEdgesMax merges parallel (from→to) edges taking the max blocked_ns (scoped Bar B).
func PreparePathEdgesMax(edges []WaitEdge) []WaitEdge {
	return ConsolidateParallelEdgesMax(edges)
}

// ConsolidateParallelEdges merges edges with the same from→to, summing blocked_ns.
func ConsolidateParallelEdges(edges []WaitEdge) []WaitEdge {
	if len(edges) < 2 {
		return edges
	}
	type key struct {
		from, to NodeID
	}
	merged := make(map[key]*WaitEdge, len(edges))
	order := make([]key, 0, len(edges))
	for _, e := range edges {
		k := key{from: e.From, to: e.To}
		if ex, ok := merged[k]; ok {
			ex.BlockedNs += e.BlockedNs
			if e.EndNs > ex.EndNs {
				ex.EndNs = e.EndNs
			}
			if e.StartNs < ex.StartNs || ex.StartNs == 0 {
				ex.StartNs = e.StartNs
			}
			continue
		}
		cp := e
		merged[k] = &cp
		order = append(order, k)
	}
	out := make([]WaitEdge, 0, len(merged))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out
}

// ConsolidateParallelEdgesMax merges same from→to keeping the largest blocked_ns (scoped Bar B).
func ConsolidateParallelEdgesMax(edges []WaitEdge) []WaitEdge {
	if len(edges) < 2 {
		return edges
	}
	type key struct {
		from, to NodeID
	}
	merged := make(map[key]*WaitEdge, len(edges))
	order := make([]key, 0, len(edges))
	for _, e := range edges {
		k := key{from: e.From, to: e.To}
		if ex, ok := merged[k]; ok {
			if e.BlockedNs > ex.BlockedNs {
				ex.BlockedNs = e.BlockedNs
				ex.EndNs = e.EndNs
				ex.StartNs = e.StartNs
			}
			continue
		}
		cp := e
		merged[k] = &cp
		order = append(order, k)
	}
	out := make([]WaitEdge, 0, len(merged))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out
}
