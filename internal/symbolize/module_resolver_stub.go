//go:build !linux

package symbolize

import "github.com/criticast/criticast/internal/trace"

// NewModuleResolver falls back to trace-only resolver on non-Linux.
func NewModuleResolver(tf *trace.File, targetBinary string) (Resolver, error) {
	return newPlatformResolver(tf, targetBinary)
}
