package groundtruth

import (
	"testing"
	"time"
)

func TestGoidsWithTokenBetween(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tl := NewTimeline([]Record{
		{TS: t0, Goid: 10, Token: "A", Site: SiteHandlerEntry},
		{TS: t0.Add(5 * time.Millisecond), Goid: 20, Token: "A", Site: SiteWorkerRecv},
		{TS: t0.Add(20 * time.Millisecond), Goid: 30, Token: "B", Site: SiteHandlerEntry},
	})
	got := tl.GoidsWithTokenBetween("A", t0, t0.Add(10*time.Millisecond))
	if len(got) != 2 {
		t.Fatalf("got %d goids", len(got))
	}
	_, ok10 := got[10]
	_, ok20 := got[20]
	if !ok10 || !ok20 {
		t.Fatalf("got %+v", got)
	}
}

func TestGoidsForHandlerSpanPinsHandlerGoid(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	entry := t0.Add(50 * time.Millisecond)
	exit := entry.Add(14 * time.Millisecond)
	tl := NewTimeline([]Record{
		{TS: entry, Goid: 10, Token: "A", Site: SiteHandlerEntry},
		{TS: entry.Add(2 * time.Millisecond), Goid: 11, Token: "A", Site: SiteHandlerEntry},
		{TS: entry.Add(3 * time.Millisecond), Goid: 12, Token: "A", Site: SiteWorkerRecv},
		{TS: exit, Goid: 10, Token: "A", Site: SiteHandlerExit},
	})
	got := tl.GoidsForHandlerSpan("A", 10, entry, exit, 0)
	if len(got) != 1 {
		t.Fatalf("want pinned handler only, got %+v", got)
	}
	if _, ok := got[10]; !ok {
		t.Fatalf("missing handler goid 10 in %+v", got)
	}
}
