package analyzer

import (
	"testing"
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

func TestCalibrateRequestGoids(t *testing.T) {
	base := time.Date(2026, 6, 1, 20, 31, 26, 0, time.UTC)
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: base.Format(time.RFC3339Nano),
	}
	entry := base
	exit := base.Add(14 * time.Millisecond)
	kFrom := uint64(1_000_000_000)
	kTo := uint64(1_020_000_000)
	events := []event.Event{
		{TsNs: 1_000_000_000, Type: event.EVBlockEnd, TaskID: 9001, Tid: 1, BlockedNs: 1000},
		{TsNs: 1_010_000_000, Type: event.EVBlockEnd, TaskID: 9002, Tid: 2, BlockedNs: 1000},
	}
	tl := groundtruth.NewTimeline([]groundtruth.Record{
		{TS: entry, Goid: 16, Token: "A", Site: groundtruth.SiteHandlerEntry},
		{TS: entry.Add(10 * time.Millisecond), Goid: 77, Token: "A", Site: groundtruth.SiteWorkerRecv},
	})
	gt := map[uint64]struct{}{16: {}, 77: {}}
	out := calibrateRequestGoids(hdr, events, tl, "A", gt, entry, exit)
	if len(out) < 2 {
		t.Fatalf("expected multiple calibrated trace tasks, got %+v", out)
	}
	if _, ok := out[9001]; !ok {
		t.Fatalf("expected trace task 9001, got %+v", out)
	}
	_ = kFrom
	_ = kTo
}
