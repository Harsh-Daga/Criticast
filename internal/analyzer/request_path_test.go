package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestClipEdgeWeightForEpoch_strictPreferred(t *testing.T) {
	const (
		strictFrom = uint64(1_100_000_000)
		strictTo   = uint64(1_100_010_000)
		padFrom    = uint64(1_099_000_000)
		padTo      = uint64(1_101_000_000)
	)
	e := WaitEdge{
		StartNs:   strictFrom - 500_000,
		EndNs:     strictFrom + 2_000_000,
		BlockedNs: 2_000_000,
	}
	got := clipEdgeWeightForEpoch(e, strictFrom, strictTo, padFrom, padTo)
	if got == 0 {
		t.Fatal("expected non-zero weight from padded fallback or strict overlap")
	}
	if got > strictTo-strictFrom {
		t.Fatalf("clip weight %d exceeds strict window", got)
	}
}

func TestPathWeightFromPathEdges_neverUsesWall(t *testing.T) {
	const wallNs = uint64(14_500_000)
	epoch := RequestEpoch{WallNs: wallNs}
	path := CriticalPath{
		Edges: []PathEdge{{
			WaitEdge: WaitEdge{BlockedNs: 1_600_000},
		}},
	}
	got := pathWeightFromPathEdges(path)
	if got == wallNs {
		t.Fatal("path_weight must not equal GT wall (tautology)")
	}
	if got != 1_600_000 {
		t.Fatalf("path_weight=%d want 1600000 from edges only", got)
	}
	_ = epoch
}

func TestPathWeightFromPathEdges_emptyPath(t *testing.T) {
	if got := pathWeightFromPathEdges(CriticalPath{}); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}
}

func TestExpandEpochMembershipFromHandlerWakeups(t *testing.T) {
	const (
		kFrom = uint64(1_000_000_000)
		kTo   = kFrom + 20_000_000
	)
	events := []event.Event{
		{
			Type: event.EVBlockEnd, TsNs: kFrom + 10_000_000, BlockedNs: 5_000_000,
			TaskID: 42, WakerTaskID: 99, Tgid: 1, Tid: 1,
		},
	}
	got := expandEpochMembershipFromHandlerWakeups(events, 42, kFrom, kTo, nil)
	if _, ok := got[42]; !ok {
		t.Fatal("missing handler")
	}
	if _, ok := got[99]; !ok {
		t.Fatal("missing waker worker from handler wakeup edge")
	}
}
