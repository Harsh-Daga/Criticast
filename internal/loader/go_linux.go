//go:build linux

package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cilium/ebpf/link"
)

// AttachGoUprobes attaches runtime.casgstatus to lift goid into tid_to_task.
func (c *Collector) AttachGoUprobes(exePath string, goidOff uint32) error {
	if goidOff == 0 {
		return fmt.Errorf("goid offset is 0")
	}
	if c.objs.UpCasgstatus == nil {
		return fmt.Errorf("bpf object missing up_casgstatus (rebuild with bpf/go_probe.c)")
	}
	if c.objs.GoCfgMap == nil {
		return fmt.Errorf("bpf object missing go_cfg_map")
	}

	abs, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("go binary %s: %w", abs, err)
	}

	k := uint32(0)
	val := goCfgMapVal{GoidOff: goidOff}
	if err := c.objs.GoCfgMap.Put(&k, &val); err != nil {
		return fmt.Errorf("go_cfg_map: %w", err)
	}

	ex, err := link.OpenExecutable(abs)
	if err != nil {
		return fmt.Errorf("open executable: %w", err)
	}
	lnk, err := ex.Uprobe("runtime.casgstatus", c.objs.UpCasgstatus, nil)
	if err != nil {
		return fmt.Errorf("uprobe casgstatus: %w", err)
	}
	c.goLink = lnk
	return nil
}
