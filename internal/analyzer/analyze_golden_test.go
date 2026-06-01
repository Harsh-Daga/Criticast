package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/criticast/criticast/internal/trace"
)

func TestAnalyzeGoldenChain(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(root, "testdata", "traces", "golden_chain.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Skip(path, err)
	}
	defer f.Close()
	tf, err := trace.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Analyze(context.Background(), tf, Options{TopN: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.CriticalPath.PathWeight < 200 {
		t.Fatalf("path_weight=%d", res.CriticalPath.PathWeight)
	}
	if len(res.DominantWaits) < 1 {
		t.Fatal("expected dominant waits")
	}
}
