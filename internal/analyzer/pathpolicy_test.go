package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestFilterPathCandidatesSelfLoop(t *testing.T) {
	edges := []WaitEdge{
		{From: 1, To: 1, BlockedNs: 1e9, WaitClass: event.WCUnknown},
		{From: 2, To: 3, BlockedNs: 1e6, WaitClass: event.WCChan},
	}
	out := FilterPathCandidates(edges, PathPolicy{DropSelfLoops: true})
	if len(out) != 1 || out[0].From != 2 {
		t.Fatalf("got %+v", out)
	}
}

func TestFilterPathCandidatesUnscopedRunQ(t *testing.T) {
	edges := []WaitEdge{
		{From: 2, To: 3, BlockedNs: 1e6, WaitClass: event.WCRunQ},
		{From: 3, To: 4, BlockedNs: 2e6, WaitClass: event.WCChan, Aux: 1},
	}
	pol := PathPolicyForScope(false)
	out := FilterPathCandidates(edges, pol)
	if len(out) != 1 || out[0].WaitClass != event.WCChan {
		t.Fatalf("got %+v", out)
	}
}

func TestFilterScopedSubgraph(t *testing.T) {
	edges := []WaitEdge{
		{From: 10, To: 20, Cookie: 0x1234, Tid: 1, BlockedNs: 100, WaitClass: event.WCChan, Aux: 1},
		{From: 20, To: 30, Cookie: 0x1234, Tid: 2, BlockedNs: 200, WaitClass: event.WCChan, Aux: 2},
		{From: 99, To: 100, Cookie: 0xffff, Tid: 3, BlockedNs: 999, WaitClass: event.WCChan},
	}
	out := FilterScopedSubgraph(edges, scopeEnv{
		scope: RequestScope{Cookie: 0x1234}, scoped: true,
	})
	if len(out) != 2 {
		t.Fatalf("got %d edges %+v", len(out), out)
	}
}
