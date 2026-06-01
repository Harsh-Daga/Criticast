package analyzer

import "testing"

func TestHandlerTimelineWeightNs_sparseBlocksFillsWindow(t *testing.T) {
	const (
		kFrom = uint64(1_000_000_000)
		kTo   = kFrom + 14_500_000
	)
	byTask := map[TaskKey][]Segment{
		{TaskID: 42}: {
			{Start: kFrom + 5_000_000, End: kFrom + 6_000_000, Kind: Blocked},
		},
	}
	got := handlerTimelineWeightNs(byTask, 42, kFrom, kTo)
	if got != kTo-kFrom {
		t.Fatalf("timeline=%d want full window %d", got, kTo-kFrom)
	}
}

func TestHandlerTimelineWeightNs_noSegmentsIsZero(t *testing.T) {
	const (
		kFrom = uint64(100)
		kTo   = uint64(14_600_100)
	)
	got := handlerTimelineWeightNs(map[TaskKey][]Segment{}, 42, kFrom, kTo)
	if got != 0 {
		t.Fatalf("timeline=%d want 0 without BPF segments", got)
	}
}

func TestHandlerTimelineWeightNs_overlappingSegmentsMerged(t *testing.T) {
	const (
		kFrom = uint64(1_000_000_000)
		kTo   = kFrom + 10_000_000
	)
	byTask := map[TaskKey][]Segment{
		{TaskID: 42}: {
			{Start: kFrom + 1_000_000, End: kFrom + 8_000_000, Kind: Blocked},
			{Start: kFrom + 2_000_000, End: kFrom + 9_000_000, Kind: Blocked},
		},
	}
	got := handlerTimelineWeightNs(byTask, 42, kFrom, kTo)
	if got > kTo-kFrom {
		t.Fatalf("timeline=%d exceeds window %d", got, kTo-kFrom)
	}
	if got < 8_000_000 {
		t.Fatalf("timeline=%d want ~union blocked+runnable within window", got)
	}
}
