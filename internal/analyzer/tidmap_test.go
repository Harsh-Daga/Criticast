package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestEdgeTouchesRequestGoidsViaTidMap(t *testing.T) {
	env := scopeEnv{
		requestGoids: map[uint64]struct{}{20: {}},
		tidToTaskID:  map[uint32]uint64{5: 20},
	}
	// Node uses tid bit because task_id was missing on the edge builder path.
	e := WaitEdge{From: NodeFromTask(0, 6), To: NodeFromTask(0, 5)}
	if !env.edgeTouchesRequestGoids(e) {
		t.Fatal("expected tid 5 → goid 20 match")
	}
}

func TestBuildTidToTaskID(t *testing.T) {
	events := []event.Event{{Tid: 1, TaskID: 42}, {Tid: 1, TaskID: 43}}
	m := buildTidToTaskID(events)
	if m[1] != 43 {
		t.Fatalf("last wins: got %d", m[1])
	}
}
