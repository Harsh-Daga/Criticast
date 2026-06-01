package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

func TestFilterByScopeToken(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: base.Format(time.RFC3339Nano),
	}
	tl := groundtruth.NewTimeline([]groundtruth.Record{
		{TS: base, Goid: 10, Token: "A", Site: groundtruth.SiteHandlerEntry},
		{TS: base.Add(100 * time.Millisecond), Goid: 20, Token: "A", Site: groundtruth.SiteWorkerRecv},
		{TS: base.Add(200 * time.Millisecond), Goid: 30, Token: "B", Site: groundtruth.SiteHandlerEntry},
	})
	edges := []WaitEdge{
		{From: 99, To: 10, EndNs: 1_100_000_000, BlockedNs: 1_000_000}, // wakee A
		{From: 10, To: 20, EndNs: 1_150_000_000, BlockedNs: 500_000},  // waker A -> wakee A
		{From: 99, To: 30, EndNs: 1_200_000_000, BlockedNs: 800_000},  // token B
	}
	env := scopeEnv{
		scope:  RequestScope{Token: "A"},
		scoped: true,
		hdr:    hdr,
		tl:     tl,
	}
	out := FilterByScope(edges, env)
	if len(out) != 2 {
		t.Fatalf("token scope: got %d edges, want 2: %+v", len(out), out)
	}
}

func TestEnrichScopedEdgesFromGTChan(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: base.Format(time.RFC3339Nano),
	}
	tl := groundtruth.NewTimeline([]groundtruth.Record{
		{TS: base, Goid: 20, Token: "A", Site: groundtruth.SiteWorkerRecv},
	})
	edges := []WaitEdge{{
		From: 10, To: 20, EndNs: 1_050_000_000, BlockedNs: 500_000,
		WaitClass: event.WCUnknown,
		Meta:      attribution.AttributeTraceEdge(event.WCUnknown, 0, 0),
	}}
	env := scopeEnv{
		scope:  RequestScope{Token: "A"},
		scoped: true,
		hdr:    hdr,
		tl:     tl,
	}
	enrichScopedEdgesFromGT(edges, env)
	if edges[0].WaitClass != event.WCChan {
		t.Fatalf("wait_class=%v want WC_CHAN", edges[0].WaitClass)
	}
}

func TestMatchesEdgeRequestGoidsInWindow(t *testing.T) {
	base := time.Date(2026, 6, 1, 20, 0, 0, 0, time.UTC)
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: base.Format(time.RFC3339Nano),
	}
	from := base.Add(10 * time.Millisecond)
	to := base.Add(24 * time.Millisecond)
	kFrom, kTo, ok := applyScopeWindow(hdr, from, to, 5*time.Millisecond)
	if !ok {
		t.Fatal("applyScopeWindow")
	}
	tl := groundtruth.NewTimeline([]groundtruth.Record{
		{TS: from, Goid: 10, Token: "A", Site: groundtruth.SiteHandlerEntry},
		{TS: from.Add(time.Millisecond), Goid: 20, Token: "A", Site: groundtruth.SiteWorkerRecv},
	})
	env := scopeEnv{
		scope:        RequestScope{Token: "A"},
		scoped:       true,
		hdr:          hdr,
		tl:           tl,
		kFrom:        kFrom,
		kTo:          kTo,
		requestGoids: buildRequestGoids(tl, "A", from, to, 5*time.Millisecond),
	}
	e := WaitEdge{From: 99, To: 20, StartNs: kFrom, EndNs: kFrom + 1000}
	if !env.matchesEdge(e) {
		t.Fatal("expected worker goid in request span")
	}
	e2 := WaitEdge{From: 99, To: 30, StartNs: kFrom, EndNs: kFrom + 1000}
	if env.matchesEdge(e2) {
		t.Fatal("goid 30 not in request span")
	}
}

func TestEdgeInWindowKtimeOverlap(t *testing.T) {
	base := time.Date(2026, 6, 1, 20, 0, 0, 0, time.UTC)
	hdr := trace.Header{
		KtimeBaseNs: 1_000_000_000,
		WallBaseUTC: base.Format(time.RFC3339Nano),
	}
	from := base.Add(10 * time.Millisecond)
	to := base.Add(24 * time.Millisecond)
	kFrom, kTo, ok := applyScopeWindow(hdr, from, to, 5*time.Millisecond)
	if !ok {
		t.Fatal("applyScopeWindow")
	}
	env := scopeEnv{kFrom: kFrom, kTo: kTo}
	// Block spans window: starts before, ends inside.
	if !env.edgeInWindow(WaitEdge{StartNs: kFrom - 1000, EndNs: kFrom + 1000}) {
		t.Fatal("expected overlap")
	}
	// Block entirely after padded window.
	if env.edgeInWindow(WaitEdge{StartNs: kTo + 10_000_000, EndNs: kTo + 20_000_000}) {
		t.Fatal("expected no overlap")
	}
}

func TestAnalyzeTokenScopeRequiresGtLog(t *testing.T) {
	hdr := trace.Header{KtimeBaseNs: 1, WallBaseUTC: time.Now().UTC().Format(time.RFC3339Nano)}
	events := []event.Event{
		{TsNs: 2, Type: event.EVBlockEnd, TaskID: 1, WakerTaskID: 2, BlockedNs: 1000},
	}
	tf := &trace.File{Header: hdr, Events: events}
	_, err := Analyze(context.Background(), tf, Options{RequestScope: "token=A"})
	if err == nil {
		t.Fatal("expected error without gt-log")
	}
}
