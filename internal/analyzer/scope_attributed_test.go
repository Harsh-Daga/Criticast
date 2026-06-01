package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/groundtruth"
)

func TestFilterScopedToken_bothEndpointsInRequestGoids(t *testing.T) {
	handler := NodeID(42)
	worker := NodeID(99)
	env := scopeEnv{
		scoped: true,
		scope:  RequestScope{Token: "A"},
		tl:     groundtruth.NewTimeline(nil),
		requestGoids: map[uint64]struct{}{
			42: {},
			99: {},
		},
		kFrom: 0,
		kTo:   1_000_000_000,
	}
	all := []WaitEdge{
		{From: handler, To: worker, BlockedNs: 2_000_000, StartNs: 100, EndNs: 2_000_100},
		{From: worker, To: worker + 1, BlockedNs: 9_000_000, StartNs: 100, EndNs: 9_000_100},
	}
	got := FilterScopedToken(all, env)
	if len(got) != 1 {
		t.Fatalf("edges=%d want handler→worker only, not full pool", len(got))
	}
	if got[0].To != worker {
		t.Fatalf("edge to=%v want worker", got[0].To)
	}
}
