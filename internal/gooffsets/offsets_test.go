package gooffsets

import (
	"path/filepath"
	"testing"
)

func TestLoadOffsets(t *testing.T) {
	root := filepath.Join("..", "..")
	path, err := ResolvePath(root)
	if err != nil {
		t.Skip(err)
	}
	db, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	off, err := db.GoidOffset("go1.22.0")
	if err != nil || off == 0 {
		t.Fatalf("goid off %d err %v", off, err)
	}
	probe, err := db.ProbeOffsets("go1.22.0")
	if err != nil {
		t.Fatal(err)
	}
	if probe.GoidOff == 0 || probe.WaitingOff == 0 {
		t.Fatalf("probe offsets %+v", probe)
	}
	key, err := db.ResolveGoVersion("go1.24.1")
	if err != nil {
		t.Fatal(err)
	}
	if key != "go1.24.0" && key != "go1.22.0" {
		t.Fatalf("resolve go1.24.1 -> %s", key)
	}
}
