package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

// Concurrent handlers for the same token must not inflate epoch membership when one handler is pinned.
func TestBuildRequestEpochMembershipBoundedWithConcurrentHandlers(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tracePath := filepath.Join(root, "testdata", "traces", "bar_b_parallel_pool.jsonl")
	gtPath := filepath.Join(t.TempDir(), "gt.log")

	wallBase, _ := time.Parse(time.RFC3339Nano, "2026-06-01T12:00:00Z")
	entry := wallBase.Add(50 * time.Millisecond)
	exit := entry.Add(14 * time.Millisecond)

	var records []groundtruth.Record
	for g := uint64(10); g < 18; g++ {
		records = append(records,
			groundtruth.Record{Token: "A", Site: groundtruth.SiteHandlerEntry, Goid: g, TS: entry},
			groundtruth.Record{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: g, Extra: "x", TS: entry.Add(1 * time.Millisecond)},
			groundtruth.Record{Token: "A", Site: groundtruth.SiteHandlerExit, Goid: g, TS: exit},
		)
	}
	records = append(records,
		groundtruth.Record{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: 10, Extra: "1001", TS: entry.Add(5 * time.Millisecond)},
		groundtruth.Record{Token: "A", Site: groundtruth.SiteWorkerRecv, Goid: 99, Extra: "1001", TS: entry.Add(8 * time.Millisecond)},
	)
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

	_, err = Analyze(context.Background(), tf, Options{
		RequestScope:     "token=A",
		GtLog:            gtPath,
		ScopeFromUTC:     entry.UTC().Format(time.RFC3339Nano),
		ScopeToUTC:       exit.UTC().Format(time.RFC3339Nano),
		ScopeHandlerGoid: 10,
		ScopePad:         4 * time.Millisecond,
	})
	if err != nil && strings.Contains(err.Error(), "epoch membership") {
		t.Fatalf("membership pollution with pinned handler: %v", err)
	}
}
