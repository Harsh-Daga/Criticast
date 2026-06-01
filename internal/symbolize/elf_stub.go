//go:build !linux

package symbolize

import "fmt"

// ELFSymbolizer is a Linux-only type; stub for cross-compile.
type ELFSymbolizer struct{}

// ResolvePC is only available on Linux.
func (e *ELFSymbolizer) ResolvePC(uint64) (Frame, bool) { return Frame{}, false }

// OpenELF is only available on Linux.
func OpenELF(string) (*ELFSymbolizer, error) {
	return nil, fmt.Errorf("symbolize: ELF resolution requires linux")
}

// OpenELFIfExists is only available on Linux.
func OpenELFIfExists(string) (*ELFSymbolizer, error) {
	return nil, nil
}
