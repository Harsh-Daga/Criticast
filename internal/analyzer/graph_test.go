package analyzer

import (
	"testing"
	"time"

	"github.com/criticast/criticast/internal/attribution"
)

func TestCriticalPathKeys(t *testing.T) {
	g := Graph{
		Token: "A",
		Edges: []Edge{
			{From: 1, To: 2, BlockedNs: 100, Key: "1->2"},
			{From: 2, To: 3, BlockedNs: 200, Key: "2->3"},
		},
	}
	keys := CriticalPathKeys(g)
	if len(keys) == 0 {
		t.Fatal("expected path keys")
	}
}

func TestCriticalPathKeysCycleTerminates(t *testing.T) {
	g := Graph{
		Token: "A",
		Edges: []Edge{
			{From: 1, To: 2, BlockedNs: 1, Key: "1->2"},
			{From: 2, To: 1, BlockedNs: 1, Key: "2->1"},
		},
	}
	done := make(chan struct{})
	go func() {
		_ = CriticalPathKeys(g)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CriticalPathKeys did not terminate on cycle")
	}
}

func TestBuildGraphs(t *testing.T) {
	edges := []attribution.TraceEdge{{WakeeGoid: 2, WakerGoid: 1, BlockedNs: 50}}
	labels := []attribution.Label{{WakeeToken: "T"}}
	graphs := BuildGraphs(edges, labels)
	if len(graphs) != 1 || graphs[0].Token != "T" {
		t.Fatalf("graphs %+v", graphs)
	}
}
