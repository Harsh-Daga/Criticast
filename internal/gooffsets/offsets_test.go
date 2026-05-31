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
}
