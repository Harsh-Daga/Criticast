package analyzer

// SCC runs Tarjan's algorithm. Returns components in reverse finish order.
func SCC(nodes []NodeID, edges []WaitEdge) [][]NodeID {
	index := make(map[NodeID]int)
	lowlink := make(map[NodeID]int)
	onStack := make(map[NodeID]bool)
	var stack []NodeID
	var comps [][]NodeID
	nextIndex := 0

	var strongConnect func(v NodeID)
	strongConnect = func(v NodeID) {
		index[v] = nextIndex
		lowlink[v] = nextIndex
		nextIndex++
		stack = append(stack, v)
		onStack[v] = true

		for _, e := range edges {
			if e.From != v {
				continue
			}
			w := e.To
			if _, ok := index[w]; !ok {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] && index[w] < lowlink[v] {
				lowlink[v] = index[w]
			}
		}

		if lowlink[v] == index[v] {
			var comp []NodeID
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			comps = append(comps, comp)
		}
	}

	seen := make(map[NodeID]struct{})
	for _, n := range nodes {
		seen[n] = struct{}{}
	}
	for _, e := range edges {
		seen[e.From] = struct{}{}
		seen[e.To] = struct{}{}
	}
	for n := range seen {
		if _, ok := index[n]; !ok {
			strongConnect(n)
		}
	}
	return comps
}

// CollapseSCC merges each strongly connected component into one super-node.
func CollapseSCC(edges []WaitEdge, comps [][]NodeID) ([]WaitEdge, map[NodeID]NodeID) {
	if len(comps) == 0 {
		return edges, nil
	}
	rep := make(map[NodeID]NodeID)
	for i, comp := range comps {
		super := NodeID(0xFFFF0000 + uint64(i))
		for _, n := range comp {
			rep[n] = super
		}
	}
	var collapsed []WaitEdge
	for _, e := range edges {
		from := rep[e.From]
		to := rep[e.To]
		if from == 0 {
			from = e.From
		}
		if to == 0 {
			to = e.To
		}
		if from == to {
			continue
		}
		collapsed = append(collapsed, WaitEdge{
			From:       from,
			To:         to,
			BlockedNs:  e.BlockedNs,
			Key:        e.Key,
			WaitClass:  e.WaitClass,
			Meta:       e.Meta,
			WakerStkID: e.WakerStkID,
			Cookie:     e.Cookie,
			Tid:        e.Tid,
			Aux:        e.Aux,
		})
	}
	return collapsed, rep
}
