package export

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"

	"github.com/criticast/criticast/internal/analyzer"
	"github.com/criticast/criticast/internal/symbolize"
)

func TestBuildPprofProfileRoundTrip(t *testing.T) {
	in := PprofInput{
		Edges: []analyzer.PathEdge{{
			WaitEdge: analyzer.WaitEdge{
				BlockedNs:  1_000_000,
				WakerStkID: 1,
			},
		}},
		Resolver: symbolize.NewTraceResolver(map[int32][]uint64{
			1: {0x401000},
		}),
	}
	p, err := BuildPprofProfile(in)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := profile.Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sample) != 1 || got.Sample[0].Value[0] != 1_000_000 {
		t.Fatalf("sample: %+v", got.Sample)
	}
}

func TestWritePprofGzipEmptyTrace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.pb.gz")
	if err := WritePprof(path, PprofInput{}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := profile.Parse(f); err != nil {
		t.Fatal(err)
	}
}

func TestWritePprofGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.pb.gz")
	in := PprofInput{
		Edges: []analyzer.PathEdge{{
			WaitEdge: analyzer.WaitEdge{BlockedNs: 500},
		}},
	}
	if err := WritePprof(path, in); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := profile.Parse(f); err != nil {
		t.Fatal(err)
	}
}
