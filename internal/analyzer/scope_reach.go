package analyzer

// Edge reachability helpers shared by request-epoch and token-scoped analysis.

func filterEdgesIncidentToSeeds(edges []WaitEdge, seeds map[NodeID]struct{}) []WaitEdge {
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		if _, ok := seeds[e.From]; ok {
			out = append(out, e)
			continue
		}
		if _, ok := seeds[e.To]; ok {
			out = append(out, e)
		}
	}
	return out
}

func filterEdgesBothInRequestGoids(edges []WaitEdge, env scopeEnv) []WaitEdge {
	if len(env.requestGoids) == 0 {
		return nil
	}
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		from, okFrom := env.nodeTaskID(e.From)
		to, okTo := env.nodeTaskID(e.To)
		if !okFrom || !okTo {
			continue
		}
		if _, ok := env.requestGoids[from]; !ok {
			continue
		}
		if _, ok := env.requestGoids[to]; !ok {
			continue
		}
		out = append(out, e)
	}
	return out
}

func filterEdgesReachableFromSeeds(edges []WaitEdge, seeds map[NodeID]struct{}, forwardOnly bool) []WaitEdge {
	seen := make(map[NodeID]bool, len(seeds))
	queue := make([]NodeID, 0, len(seeds))
	for n := range seeds {
		seen[n] = true
		queue = append(queue, n)
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range edges {
			if forwardOnly {
				if e.From == n && !seen[e.To] {
					seen[e.To] = true
					queue = append(queue, e.To)
				}
				continue
			}
			if e.From == n && !seen[e.To] {
				seen[e.To] = true
				queue = append(queue, e.To)
			}
			if e.To == n && !seen[e.From] {
				seen[e.From] = true
				queue = append(queue, e.From)
			}
		}
	}
	out := make([]WaitEdge, 0, len(edges))
	for _, e := range edges {
		if seen[e.From] && seen[e.To] {
			out = append(out, e)
		}
	}
	return out
}
