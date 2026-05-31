//go:build !linux

package loader

import (
	"fmt"

	"github.com/cilium/ebpf"

	"github.com/criticast/criticast/internal/event"
)

// Collector is unavailable off Linux.
type Collector struct{}

func (c *Collector) Ringbuf() *ebpf.Map { return nil }

func (c *Collector) Stats() ([event.StatMax]uint64, error) {
	return [event.StatMax]uint64{}, fmt.Errorf("criticast bpf requires linux (5.8+ with BTF)")
}

func (c *Collector) Close() error { return nil }

// Load returns an error on non-Linux platforms.
func Load(uint32, Config, string) (*Collector, error) {
	return nil, fmt.Errorf("criticast bpf requires linux (5.8+ with BTF)")
}
