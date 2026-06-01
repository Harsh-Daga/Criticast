package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestLongestPathChain(t *testing.T) {
	// waker 30 -> 20 -> 10 with increasing blocked time
	edges := []WaitEdge{
		{From: 30, To: 20, BlockedNs: 100},
		{From: 20, To: 10, BlockedNs: 200},
	}
	path := LongestPath(edges)
	if path.PathWeight != 300 {
		t.Fatalf("weight=%d", path.PathWeight)
	}
	if len(path.Edges) != 2 {
		t.Fatalf("edges=%d", len(path.Edges))
	}
}

func TestSCCCollapseSelfLoop(t *testing.T) {
	edges := []WaitEdge{
		{From: 1, To: 2, BlockedNs: 50},
		{From: 2, To: 1, BlockedNs: 50},
		{From: 1, To: 3, BlockedNs: 100},
	}
	comps := SCC(nil, edges)
	if len(comps) < 1 {
		t.Fatal("expected SCC")
	}
	collapsed, _ := CollapseSCC(edges, comps)
	path := LongestPath(collapsed)
	if path.PathWeight == 0 {
		t.Fatalf("path=%+v collapsed=%d", path, len(collapsed))
	}
}

func TestBuildWaitEdgesChain(t *testing.T) {
	events := []event.Event{
		{TsNs: 300, Type: event.EVBlockEnd, TaskID: 10, BlockedNs: 100, WakerTaskID: 20},
		{TsNs: 500, Type: event.EVBlockEnd, TaskID: 20, BlockedNs: 200, WakerTaskID: 30},
	}
	edges := BuildWaitEdges(events)
	path := LongestPath(edges)
	if path.PathWeight < 200 {
		t.Fatalf("path weight=%d edges=%d", path.PathWeight, len(path.Edges))
	}
}
