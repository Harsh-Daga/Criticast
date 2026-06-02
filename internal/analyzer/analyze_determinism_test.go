package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/trace"
)

func TestAnalyzeDeterministicCriticalPath(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "traces", "bar_b_scoped.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Skip(path, err)
	}
	defer f.Close()
	tf, err := trace.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{RequestScope: "0x1234", TopN: 5}
	r1, err := Analyze(context.Background(), tf, opts)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Analyze(context.Background(), tf, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.CriticalPath.Edges) != len(r2.CriticalPath.Edges) {
		t.Fatalf("edge count %d vs %d", len(r1.CriticalPath.Edges), len(r2.CriticalPath.Edges))
	}
	for i := range r1.CriticalPath.Edges {
		if r1.CriticalPath.Edges[i].Key != r2.CriticalPath.Edges[i].Key {
			t.Fatalf("edge %d key %q vs %q", i, r1.CriticalPath.Edges[i].Key, r2.CriticalPath.Edges[i].Key)
		}
	}
}

func TestAnalyzeBarBScopedHasChanClass(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "traces", "bar_b_scoped.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Skip(path, err)
	}
	defer f.Close()
	tf, err := trace.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Analyze(context.Background(), tf, Options{RequestScope: "0x1234", TopN: 5})
	if err != nil {
		t.Fatal(err)
	}
	foundChan := false
	for _, pe := range res.CriticalPath.Edges {
		if pe.WaitClass == event.WCChan {
			foundChan = true
		}
	}
	if !foundChan {
		t.Fatal("expected WC_CHAN on scoped critical path")
	}
}
