package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestBuildSegmentsBlockEnd(t *testing.T) {
	events := []event.Event{
		{
			TsNs: 1000, Type: event.EVBlockEnd, Tid: 1, TaskID: 10,
			BlockedNs: 500, WakerTaskID: 20, WaitClass: event.WCChan,
		},
	}
	byTask := BuildSegments(events)
	segs := byTask[TaskKey{TaskID: 10, Tid: 1}]
	if len(segs) != 1 {
		t.Fatalf("got %d segments", len(segs))
	}
	if segs[0].Kind != Blocked || segs[0].Start != 500 || segs[0].End != 1000 {
		t.Fatalf("segment: %+v", segs[0])
	}
	if segs[0].WakerID != 20 {
		t.Fatalf("waker: %d", segs[0].WakerID)
	}
}

func TestBuildSegmentsRunQ(t *testing.T) {
	events := []event.Event{
		{TsNs: 2000, Type: event.EVRunQ, Tid: 2, BlockedNs: 100},
	}
	byTask := BuildSegments(events)
	segs := byTask[TaskKey{Tid: 2}]
	if len(segs) != 1 || segs[0].Kind != Runnable {
		t.Fatalf("got %+v", segs)
	}
}
