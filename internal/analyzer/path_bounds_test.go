package analyzer

import "testing"

func TestClipBlockedNsToWindow(t *testing.T) {
	e := WaitEdge{StartNs: 100, EndNs: 300, BlockedNs: 200}
	if w := ClipBlockedNsToWindow(e, 0, 1000); w != 200 {
		t.Fatalf("full overlap: got %d", w)
	}
	if w := ClipBlockedNsToWindow(e, 150, 250); w != 100 {
		t.Fatalf("partial: got %d", w)
	}
	if w := ClipBlockedNsToWindow(e, 400, 500); w != 0 {
		t.Fatalf("outside: got %d", w)
	}
}

func TestFilterScopedSubgraphHandlerRoot(t *testing.T) {
	handler := NodeFromTask(10, 0)
	edges := []WaitEdge{
		{From: 20, To: handler, Cookie: 0x1234, BlockedNs: 100, WaitClass: 1, Aux: 1},
		{From: 99, To: 100, Cookie: 0x1234, BlockedNs: 999},
	}
	out := FilterScopedSubgraph(edges, scopeEnv{
		scope:        RequestScope{Cookie: 0x1234},
		scoped:       true,
		handlerSeeds: map[NodeID]struct{}{handler: {}},
	})
	if len(out) != 1 || out[0].From != 20 {
		t.Fatalf("got %+v", out)
	}
}
