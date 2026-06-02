//go:build linux

package symbolize

import (
	"debug/buildinfo"
	"debug/elf"
	"fmt"
	"os"
	"sort"
	"sync"
)

type pcSymbol struct {
	addr uint64
	name string
}

// ELFSymbolizer resolves PCs from an on-disk executable (symtab / .gopclntab not required).
type ELFSymbolizer struct {
	buildID string
	base    uint64 // VMA start from /proc/maps for PIE
	syms    []pcSymbol
	cache   sync.Map // pc -> Frame
}

// OpenELF loads symbol table entries from path and caches by build-id when present.
func OpenELF(path string) (*ELFSymbolizer, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("symbolize: open elf %s: %w", path, err)
	}
	defer f.Close()

	var syms []pcSymbol
	if st, err := f.Symbols(); err == nil {
		for _, s := range st {
			if s.Section == elf.SHN_UNDEF || s.Value == 0 || s.Name == "" {
				continue
			}
			syms = append(syms, pcSymbol{addr: s.Value, name: s.Name})
		}
	}
	if len(syms) == 0 {
		if dyn, err := f.DynamicSymbols(); err == nil {
			for _, s := range dyn {
				if s.Section == elf.SHN_UNDEF || s.Value == 0 || s.Name == "" {
					continue
				}
				syms = append(syms, pcSymbol{addr: s.Value, name: s.Name})
			}
		}
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].addr < syms[j].addr })
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbolize: no symbols in %s (binary stripped?)", path)
	}

	bid := readELFBuildID(path)
	if bid == "" {
		if bi, err := buildinfo.ReadFile(path); err == nil && bi != nil && bi.GoVersion != "" {
			bid = "go:" + bi.GoVersion
		}
	}

	return &ELFSymbolizer{buildID: bid, syms: syms}, nil
}

// ResolvePC returns the best-effort symbol for a program counter.
func (e *ELFSymbolizer) ResolvePC(pc uint64) (Frame, bool) {
	if v, ok := e.cache.Load(pc); ok {
		fr := v.(Frame)
		return fr, fr.Function != ""
	}
	rel := pc
	if e.base != 0 && pc >= e.base {
		rel = pc - e.base
	}
	name, _ := e.lookupName(rel)
	fr := Frame{PC: pc, Function: name, File: "", BuildID: e.buildID}
	if name == "" {
		fr.Function = fmt.Sprintf("0x%x", pc)
		e.cache.Store(pc, fr)
		return fr, false
	}
	e.cache.Store(pc, fr)
	return fr, true
}

func (e *ELFSymbolizer) lookupName(pc uint64) (string, bool) {
	if len(e.syms) == 0 {
		return "", false
	}
	i := sort.Search(len(e.syms), func(i int) bool { return e.syms[i].addr > pc })
	if i == 0 {
		return "", false
	}
	return e.syms[i-1].name, true
}

// BuildID returns a best-effort build identifier for cache keys.
func (e *ELFSymbolizer) BuildID() string { return e.buildID }

// OpenELFIfExists returns nil, nil when path is empty or missing (not an error).
func OpenELFIfExists(path string) (*ELFSymbolizer, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	return OpenELF(path)
}
