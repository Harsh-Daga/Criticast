package analyzer

// enrichCriticalPathFromGT applies GT mechanism labels to edges on the computed path.
func enrichCriticalPathFromGT(path *CriticalPath, env scopeEnv) {
	if path == nil || len(path.Edges) == 0 {
		return
	}
	edges := make([]WaitEdge, len(path.Edges))
	for i, pe := range path.Edges {
		edges[i] = pe.WaitEdge
	}
	enrichScopedEdgesFromGT(edges, env)
	for i := range path.Edges {
		path.Edges[i].WaitEdge = edges[i]
	}
}
