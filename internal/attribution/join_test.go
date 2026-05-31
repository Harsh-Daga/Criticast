package attribution

import (
	"testing"
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

func TestEdgesFromTraceWallClockJoin(t *testing.T) {
	wall, err := time.Parse(time.RFC3339Nano, "2026-05-31T20:00:00.123456789Z")
	if err != nil {
		t.Fatal(err)
	}
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: wall.Format(time.RFC3339Nano),
	}
	events := []event.Event{
		{
			TsNs:      1_500_000_000,
			Type:      event.EVBlockEnd,
			TaskID:    42,
			BlockedNs: 1000,
		},
	}
	recs := []groundtruth.Record{
		{Token: "A", Site: groundtruth.SiteMutexLock, Goid: 42, TS: wall.Add(400 * time.Millisecond)},
	}
	tl := groundtruth.NewTimeline(recs)
	edges := EdgesFromTrace(hdr, events)
	if len(edges) != 1 {
		t.Fatalf("edges %d", len(edges))
	}
	gold, labeled := LabelTraceEdges(edges, tl)
	if len(labeled) != 1 || gold[0].WakeeToken != "A" {
		t.Fatalf("labeled %+v gold %+v", labeled, gold)
	}
	st := JoinStatsFromTrace(hdr, events, tl)
	if st.Labeled != 1 || !st.ClockCorrelated {
		t.Fatalf("stats %+v", st)
	}
}

func TestEdgesFromTraceWithoutClockBaseNoJoin(t *testing.T) {
	wall, _ := time.Parse(time.RFC3339Nano, "2026-05-31T20:00:00Z")
	hdr := trace.Header{}
	events := []event.Event{{TsNs: 1, Type: event.EVBlockEnd, TaskID: 1}}
	recs := []groundtruth.Record{{Token: "A", Site: groundtruth.SiteMutexLock, Goid: 1, TS: wall}}
	tl := groundtruth.NewTimeline(recs)
	_, labeled := LabelTraceEdges(EdgesFromTrace(hdr, events), tl)
	if len(labeled) != 0 {
		t.Fatalf("expected no join without clock base, got %d", len(labeled))
	}
}
