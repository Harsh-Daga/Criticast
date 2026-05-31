// Package gooffsets loads runtime.g field offsets for Go uprobes (CHARTER Appendix Q.2).
package gooffsets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const schemaVersion = 1

type versionEntry struct {
	Goid       map[string]int `json:"g.goid"`
	ParentGoid map[string]int `json:"g.parentGoid"`
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

// GoidOffset returns the goid field offset for goVersion on the current GOARCH.
func (db *Database) GoidOffset(goVersion string) (uint32, error) {
	ent, ok := db.Versions[goVersion]
	if !ok {
		return 0, fmt.Errorf("offsets: no entry for %s", goVersion)
	}
	off, ok := ent.Goid[runtime.GOARCH]
	if !ok {
		return 0, fmt.Errorf("offsets: no g.goid for %s/%s", goVersion, runtime.GOARCH)
	}
	return uint32(off), nil
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
