// Package analyzer builds wait-for graphs and critical paths (L4).
//
// Graph construction and longest-path keys for Jaccard vs ground truth.
// SCC / cascade DP: CHARTER Part E, docs/ROADMAP.md.
package analyzer

import (
	"fmt"
	"sort"

	"github.com/criticast/criticast/internal/attribution"
)

// Edge is a directed wait-for edge with weight (blocked ns).
type Edge struct {
	From      uint64
	To        uint64
	BlockedNs uint64
	Key       string
}

// Graph is a wait-for DAG fragment for one request token.
type Graph struct {
	Token string
	Edges []Edge
}

// BuildGraphs groups trace edges by wakee token (from labels).
func BuildGraphs(edges []attribution.TraceEdge, labels []attribution.Label) []Graph {
	if len(edges) != len(labels) {
		return nil
	}
	byToken := make(map[string][]Edge)
	for i, te := range edges {
		tok := labels[i].WakeeToken
		if tok == "" {
			continue
		}
		key := fmt.Sprintf("%d->%d", te.WakerGoid, te.WakeeGoid)
		byToken[tok] = append(byToken[tok], Edge{
			From:      te.WakerGoid,
			To:        te.WakeeGoid,
			BlockedNs: te.BlockedNs,
			Key:       key,
		})
	}
	tokens := make([]string, 0, len(byToken))
	for t := range byToken {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	out := make([]Graph, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, Graph{Token: t, Edges: byToken[t]})
	}
	return out
}

// CriticalPathKeys returns edge keys on a longest-weight path (simple DP on small graphs).
func CriticalPathKeys(g Graph) []string {
	if len(g.Edges) == 0 {
		return nil
	}
	// Build adjacency from waker -> wakee
	type edge struct {
		to  uint64
		key string
		w   uint64
	}
	adj := make(map[uint64][]edge)
	nodes := make(map[uint64]struct{})
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], edge{to: e.To, key: e.Key, w: e.BlockedNs})
		nodes[e.From] = struct{}{}
		nodes[e.To] = struct{}{}
	}
	best := make(map[uint64]uint64)
	prev := make(map[uint64]uint64)
	prevKey := make(map[uint64]string)
	for n := range nodes {
		best[n] = 0
	}
	// Bellman-Ford style relaxation (bounded — cycles would otherwise spin forever).
	maxIter := len(nodes)
	if maxIter < 1 {
		maxIter = 1
	}
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for from, outs := range adj {
			for _, e := range outs {
				if best[from]+e.w > best[e.to] {
					best[e.to] = best[from] + e.w
					prev[e.to] = from
					prevKey[e.to] = e.key
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	var end uint64
	var maxW uint64
	for n, w := range best {
		if w >= maxW {
			maxW = w
			end = n
		}
	}
	var keys []string
	visited := make(map[uint64]struct{})
	for end != 0 {
		if _, loop := visited[end]; loop {
			break
		}
		visited[end] = struct{}{}
		k, ok := prevKey[end]
		if !ok {
			break
		}
		keys = append([]string{k}, keys...)
		end = prev[end]
	}
	return keys
}

// PathKeySet flattens critical path keys for Jaccard comparison.
func PathKeySet(graphs []Graph) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, g := range graphs {
		for _, k := range CriticalPathKeys(g) {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}
