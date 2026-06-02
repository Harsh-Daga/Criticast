package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

func TestAnalyzeRequestEpochParallelPoolTemporalInvariant(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tracePath := filepath.Join(root, "testdata", "traces", "bar_b_parallel_pool.jsonl")
	gtPath := filepath.Join(t.TempDir(), "gt.log")

	wallBase, _ := time.Parse(time.RFC3339Nano, "2026-06-01T12:00:00Z")
	records := []groundtruth.Record{
		{Token: "A", Site: groundtruth.SiteHandlerEntry, Goid: 42, TS: wallBase.Add(50 * time.Millisecond)},
		{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: 42, Extra: "1001", TS: wallBase.Add(55 * time.Millisecond)},
		{Token: "A", Site: groundtruth.SiteWorkerRecv, Goid: 99, Extra: "1001", TS: wallBase.Add(80 * time.Millisecond)},
		{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: 42, Extra: "1002", TS: wallBase.Add(56 * time.Millisecond)},
		{Token: "A", Site: groundtruth.SiteHandlerExit, Goid: 42, TS: wallBase.Add(64*time.Millisecond + 500*time.Microsecond)},
	}
	var gtLines string
	for _, rec := range records {
		line, err := rec.FormatLine()
		if err != nil {
			t.Fatal(err)
		}
		gtLines += line + "\n"
	}
	if err := os.WriteFile(gtPath, []byte(gtLines), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(tracePath)
	if err != nil {
		t.Skip(tracePath, err)
	}
	defer f.Close()
	tf, err := trace.Read(f)
	if err != nil {
		t.Fatal(err)
	}

	entry := wallBase.Add(50 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	exit := wallBase.Add(64*time.Millisecond + 500*time.Microsecond).UTC().Format(time.RFC3339Nano)

	res, err := Analyze(context.Background(), tf, Options{
		RequestScope:     "token=A",
		GtLog:            gtPath,
		ScopeFromUTC:     entry,
		ScopeToUTC:       exit,
		ScopeHandlerGoid: 42,
		ScopePad:         4 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	const wallNs = uint64(14_500_000)
	if res.CriticalPath.PathWeight > wallNs+2_000_000 {
		t.Fatalf("path_weight=%d exceeds wall+slack (970ms-class bug)", res.CriticalPath.PathWeight)
	}
	if !PathWeightInvariantOK(res.CriticalPath, wallNs, 2_000_000) {
		t.Fatalf("invariant failed path_weight=%d", res.CriticalPath.PathWeight)
	}
	if len(res.CriticalPath.Edges) == 0 {
		t.Fatal("expected at least one critical-path edge for scoped Bar B")
	}
	if res.EdgeCount == 0 {
		t.Fatal("expected scoped edges for token scope")
	}
}
