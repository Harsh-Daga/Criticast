// Package symbolize resolves BPF stack IDs to frames (charter L1).
package symbolize

import (
	"fmt"
	"sync"
)

// Frame is one stack frame (symbol or raw PC).
type Frame struct {
	PC       uint64
	Function string
	File     string
	Line     int
	BuildID  string
}

// Resolver maps stack_id to frames. Implementations must be safe for concurrent use.
type Resolver interface {
	Resolve(stackID int32) ([]Frame, error)
}

// BuildIDCache maps build-id strings to module metadata (skeleton for P1).
type BuildIDCache interface {
	Lookup(buildID string) (modulePath string, ok bool)
	Store(buildID, modulePath string)
}

// MemoryBuildIDCache is an in-memory build-id cache.
type MemoryBuildIDCache struct {
	mu   sync.RWMutex
	byID map[string]string
}

// NewMemoryBuildIDCache returns an empty build-id cache.
func NewMemoryBuildIDCache() *MemoryBuildIDCache {
	return &MemoryBuildIDCache{byID: make(map[string]string)}
}

func (c *MemoryBuildIDCache) Lookup(buildID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.byID[buildID]
	return p, ok
}

func (c *MemoryBuildIDCache) Store(buildID, modulePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[buildID] = modulePath
}

// TraceResolver resolves stack IDs using PCs stored in the trace STACKS section.
type TraceResolver struct {
	stacks map[int32][]uint64
	cache  sync.Map // stackID -> []Frame
}

// NewTraceResolver builds a resolver from trace stack_id → PC lists.
func NewTraceResolver(stacks map[int32][]uint64) *TraceResolver {
	if stacks == nil {
		stacks = make(map[int32][]uint64)
	}
	return &TraceResolver{stacks: stacks}
}

// PCs returns program counters for a trace stack_id (empty if unknown).
func (r *TraceResolver) PCs(stackID int32) []uint64 {
	if stackID < 0 {
		return nil
	}
	pcs, ok := r.stacks[stackID]
	if !ok || len(pcs) == 0 {
		return nil
	}
	return pcs
}

// Resolve returns frames for stackID. Missing or negative IDs yield nil, nil.
// Use NewForTrace to enrich PCs via ELF when a target binary is available.
func (r *TraceResolver) Resolve(stackID int32) ([]Frame, error) {
	if stackID < 0 {
		return nil, nil
	}
	if cached, ok := r.cache.Load(stackID); ok {
		return cached.([]Frame), nil
	}
	pcs, ok := r.stacks[stackID]
	if !ok || len(pcs) == 0 {
		return nil, nil
	}
	frames := make([]Frame, len(pcs))
	for i, pc := range pcs {
		frames[i] = Frame{
			PC:       pc,
			Function: fmt.Sprintf("0x%x", pc),
		}
	}
	r.cache.Store(stackID, frames)
	return frames, nil
}
