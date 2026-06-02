package symbolize

import (
	"fmt"
	"strings"
	"sync"

	"github.com/criticast/criticast/internal/trace"
)

// ChainedResolver uses trace stack PCs and optionally enriches with ELF symtab.
type ChainedResolver struct {
	trace *TraceResolver
	elf   *ELFSymbolizer
	once  sync.Once
	hint  string
}

// NewForTrace builds the default resolver for analyze/export.
func NewForTrace(tf *trace.File, targetBinary string) (Resolver, error) {
	return newPlatformResolver(tf, targetBinary)
}

// StrippedBinaryHint is printed once when ELF symbolization is unavailable.
const StrippedBinaryHint = "install debug symbols or record with --go-binary for uprobes; stacks use raw PCs"

func (c *ChainedResolver) Resolve(stackID int32) ([]Frame, error) {
	frames, err := c.trace.Resolve(stackID)
	if err != nil || c.elf == nil {
		return frames, err
	}
	for i := range frames {
		if !isPlaceholder(frames[i].Function) {
			continue
		}
		if sym, ok := c.elf.ResolvePC(frames[i].PC); ok {
			frames[i] = sym
		}
	}
	return frames, nil
}

func isPlaceholder(fn string) bool {
	return fn == "" || strings.HasPrefix(fn, "0x")
}

// PrintHintOnce writes the stripped-binary hint to stderr at most once.
func (c *ChainedResolver) PrintHintOnce(w interface{ Write([]byte) (int, error) }) {
	if c.hint == "" || w == nil {
		return
	}
	c.once.Do(func() {
		_, _ = fmt.Fprintf(w, "criticast: %s\n", c.hint)
	})
}
