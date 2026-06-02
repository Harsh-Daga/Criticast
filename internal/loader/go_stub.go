//go:build !linux

package loader

import "fmt"

// AttachGoUprobes is only available on Linux.
func (c *Collector) AttachGoUprobes(string, GoProbeOffsets) error {
	return fmt.Errorf("criticast go uprobes require linux")
}
