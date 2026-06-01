package analyzer

import "testing"

func TestParseRequestScopeGoid(t *testing.T) {
	sc, ok := ParseRequestScope("goid=121853")
	if !ok || sc.TaskID != 121853 || sc.Tid != 0 || sc.Cookie != 0 {
		t.Fatalf("goid scope: ok=%v sc=%+v", ok, sc)
	}
}

func TestParseRequestScopeTidDecimal(t *testing.T) {
	sc, ok := ParseRequestScope("42")
	if !ok || sc.Tid != 42 || sc.TaskID != 0 {
		t.Fatalf("tid scope: ok=%v sc=%+v", ok, sc)
	}
}

func TestEdgeMatchesScopeByTaskID(t *testing.T) {
	scope := RequestScope{TaskID: 99}
	e := WaitEdge{From: 1, To: 99, Cookie: 0, Tid: 0}
	if !EdgeMatchesScope(e, scope, true) {
		t.Fatal("expected wakee match")
	}
	e2 := WaitEdge{From: 99, To: 2}
	if !EdgeMatchesScope(e2, scope, true) {
		t.Fatal("expected waker match")
	}
	if EdgeMatchesScope(WaitEdge{From: 1, To: 2}, scope, true) {
		t.Fatal("expected no match")
	}
}

func TestFilterByScopeGoid(t *testing.T) {
	edges := []WaitEdge{
		{From: 1, To: 10, Cookie: 0},
		{From: 10, To: 20, Cookie: 0},
		{From: 20, To: 30, Cookie: 0},
	}
	scope := RequestScope{TaskID: 10}
	out := FilterByScope(edges, scopeEnv{scope: scope, scoped: true})
	if len(out) != 2 {
		t.Fatalf("FilterByScope want edges touching goid 10, got %+v", out)
	}
}
