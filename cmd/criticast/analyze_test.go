package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/trace"
)

func TestAnalyzeOnV2Trace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.criticast")
	var buf bytes.Buffer
	events := []event.Event{
		{TsNs: 2000, Type: event.EVBlockEnd, Tid: 1, TaskID: 10, BlockedNs: 1000, WakerTaskID: 20},
	}
	if err := trace.Write(&buf, trace.Header{Tgid: 1}, events, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAnalyze([]string{path, "--format", "json"}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeGoldenV1(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(self), "..", "..")
	path := filepath.Join(root, "testdata", "traces", "golden_chain.jsonl")
	if err := runAnalyze([]string{path, "--format", "json"}); err != nil {
		t.Fatal(err)
	}
}
