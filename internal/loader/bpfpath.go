//go:build linux

package loader

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveObjectPath returns an existing BPF object path or the default bpf/collector.bpf.o.
func ResolveObjectPath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("bpf object %s: %w", explicit, err)
		}
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return explicit, nil
		}
		return abs, nil
	}
	candidates := []string{
		"bpf/collector.bpf.o",
		filepath.Join("..", "bpf", "collector.bpf.o"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c, nil
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("bpf object not found (run: make bpf)")
}
