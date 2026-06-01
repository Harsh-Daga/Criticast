// Package gooffsets loads runtime.g field offsets for Go uprobes (CHARTER Appendix Q.2).
package gooffsets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const schemaVersion = 1

type versionEntry struct {
	Goid       map[string]int `json:"g.goid"`
	ParentGoid map[string]int `json:"g.parentGoid"`
	Waiting    map[string]int `json:"g.waiting"`
	SudogElem  map[string]int `json:"sudog.elem"`
}

// ProbeOffsets holds runtime.g / sudog field offsets for Go uprobes.
type ProbeOffsets struct {
	GoidOff      uint32
	WaitingOff   uint32
	SudogElemOff uint32
}

// Database is the parsed offsets.json file.
type Database struct {
	Versions map[string]versionEntry
}

// Load reads offsets.json from path, or CRITICAST_OFFSETS, or bpf/offsets.json.
func Load(path string) (*Database, error) {
	if path == "" {
		path = os.Getenv("CRITICAST_OFFSETS")
	}
	if path == "" {
		path = "bpf/offsets.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("offsets: read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("offsets: parse: %w", err)
	}
	var schema int
	if err := json.Unmarshal(raw["schema"], &schema); err == nil && schema != 0 && schema != schemaVersion {
		return nil, fmt.Errorf("offsets: unsupported schema %d", schema)
	}
	db := &Database{Versions: make(map[string]versionEntry)}
	for k, v := range raw {
		if k == "schema" {
			continue
		}
		var ent versionEntry
		if err := json.Unmarshal(v, &ent); err != nil {
			continue
		}
		db.Versions[k] = ent
	}
	return db, nil
}

func archOffset(m map[string]int, goVersion, field string) (uint32, error) {
	if len(m) == 0 {
		return 0, nil
	}
	off, ok := m[runtime.GOARCH]
	if !ok {
		return 0, fmt.Errorf("offsets: no %s for %s/%s", field, goVersion, runtime.GOARCH)
	}
	return uint32(off), nil
}

// GoidOffset returns the goid field offset for goVersion on the current GOARCH.
func (db *Database) GoidOffset(goVersion string) (uint32, error) {
	key, err := db.ResolveGoVersion(goVersion)
	if err != nil {
		return 0, err
	}
	return archOffset(db.Versions[key].Goid, key, "g.goid")
}

// ResolveGoVersion picks an offsets.json key for the target Go toolchain.
// Falls back to go1.22.0 when an exact row is missing (patch releases often share layout).
func (db *Database) ResolveGoVersion(goVersion string) (string, error) {
	if _, ok := db.Versions[goVersion]; ok {
		return goVersion, nil
	}
	// go1.24.1 -> go1.24.0
	if i := strings.LastIndex(goVersion, "."); i > 0 {
		majorMinor := goVersion[:i] + ".0"
		if _, ok := db.Versions[majorMinor]; ok {
			return majorMinor, nil
		}
	}
	for _, fb := range []string{"go1.22.0", "go1.21.0"} {
		if _, ok := db.Versions[fb]; ok {
			return fb, nil
		}
	}
	return "", fmt.Errorf("offsets: no entry for %s", goVersion)
}

// ProbeOffsets returns offsets for casgstatus and gopark/sudog capture.
func (db *Database) ProbeOffsets(goVersion string) (ProbeOffsets, error) {
	key, err := db.ResolveGoVersion(goVersion)
	if err != nil {
		return ProbeOffsets{}, err
	}
	ent := db.Versions[key]
	goid, err := archOffset(ent.Goid, key, "g.goid")
	if err != nil {
		return ProbeOffsets{}, err
	}
	waiting, err := archOffset(ent.Waiting, key, "g.waiting")
	if err != nil {
		return ProbeOffsets{}, err
	}
	elem, err := archOffset(ent.SudogElem, key, "sudog.elem")
	if err != nil {
		return ProbeOffsets{}, err
	}
	return ProbeOffsets{GoidOff: goid, WaitingOff: waiting, SudogElemOff: elem}, nil
}

// ResolvePath finds offsets.json under repo root.
func ResolvePath(root string) (string, error) {
	for _, c := range []string{
		filepath.Join(root, "bpf", "offsets.json"),
		filepath.Join(root, "offsets.json"),
	} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("offsets.json not found under %s", root)
}
