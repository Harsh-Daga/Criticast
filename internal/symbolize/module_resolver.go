//go:build linux

package symbolize

import (
	"fmt"
	"sync"

	"github.com/criticast/criticast/internal/trace"
)

type mappedELF struct {
	mod trace.Module
	elf *ELFSymbolizer
}

// ModuleResolver resolves stack PCs using trace stacks and /proc/maps modules.
type ModuleResolver struct {
	trace   *TraceResolver
	modules []mappedELF
	byPath  sync.Map // path -> *ELFSymbolizer
	cache   sync.Map // stackID -> []Frame
}

// NewModuleResolver builds a resolver from trace file contents.
func NewModuleResolver(tf *trace.File, targetBinary string) (Resolver, error) {
	tr := NewTraceResolver(tf.Stacks)
	if targetBinary == "" {
		targetBinary = tf.Header.TargetBinary
	}
	mods := tf.Modules
	if len(mods) == 0 && targetBinary != "" {
		elf, err := OpenELF(targetBinary)
		if err != nil {
			return &ChainedResolver{trace: tr, hint: StrippedBinaryHint}, nil
		}
		return &ChainedResolver{trace: tr, elf: elf}, nil
	}
	if len(mods) == 0 {
		return &ChainedResolver{trace: tr, hint: StrippedBinaryHint}, nil
	}
	mr := &ModuleResolver{trace: tr}
	for _, m := range mods {
		elf, err := mr.loadELF(m)
		if err != nil {
			continue
		}
		elf.base = m.Start
		mr.modules = append(mr.modules, mappedELF{mod: m, elf: elf})
	}
	if len(mr.modules) == 0 {
		return &ChainedResolver{trace: tr, hint: StrippedBinaryHint}, nil
	}
	return mr, nil
}

func (m *ModuleResolver) loadELF(mod trace.Module) (*ELFSymbolizer, error) {
	key := BuildIDKey(mod.Path, mod.BuildID)
	if v, ok := m.byPath.Load(key); ok {
		return v.(*ELFSymbolizer), nil
	}
	elf, err := OpenELF(mod.Path)
	if err != nil {
		return nil, err
	}
	m.byPath.Store(key, elf)
	return elf, nil
}

func (m *ModuleResolver) Resolve(stackID int32) ([]Frame, error) {
	if stackID < 0 {
		return nil, nil
	}
	if cached, ok := m.cache.Load(stackID); ok {
		return cached.([]Frame), nil
	}
	pcs := m.trace.PCs(stackID)
	if len(pcs) == 0 {
		return nil, nil
	}
	frames := make([]Frame, len(pcs))
	for i, pc := range pcs {
		frames[i] = m.resolvePC(pc)
	}
	m.cache.Store(stackID, frames)
	return frames, nil
}

func (m *ModuleResolver) resolvePC(pc uint64) Frame {
	for _, me := range m.modules {
		if pc < me.mod.Start || pc >= me.mod.End {
			continue
		}
		fr, ok := me.elf.ResolvePC(pc)
		if ok {
			return fr
		}
		return Frame{PC: pc, Function: fmt.Sprintf("0x%x", pc), BuildID: me.elf.BuildID()}
	}
	return Frame{PC: pc, Function: fmt.Sprintf("0x%x", pc)}
}
