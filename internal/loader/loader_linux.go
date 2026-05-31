//go:build linux

package loader

import (
	"errors"
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"github.com/criticast/criticast/internal/event"
)

// bpfConfig mirrors struct config in bpf/collector.c.
type bpfConfig struct {
	MinBlockNs  uint64
	SampleMod   uint32
	Flags       uint32
	CookieTTLNs uint64
}

// goCfgMapVal mirrors struct go_cfg in bpf/go_cfg.h.
type goCfgMapVal struct {
	GoidOff uint32
	Pad     uint32
}

type bpfObjects struct {
	Events       *ebpf.Map     `ebpf:"events"`
	Cfg          *ebpf.Map     `ebpf:"cfg"`
	Targets      *ebpf.Map     `ebpf:"targets"`
	Stats        *ebpf.Map     `ebpf:"stats"`
	TidToTask    *ebpf.Map     `ebpf:"tid_to_task"`
	GoCfgMap     *ebpf.Map     `ebpf:"go_cfg_map"`
	HandleSwitch *ebpf.Program `ebpf:"handle_switch"`
	HandleWaking *ebpf.Program `ebpf:"handle_waking"`
	UpCasgstatus *ebpf.Program `ebpf:"up_casgstatus"`
}

// Collector holds loaded BPF objects and tracepoint links.
type Collector struct {
	objs    bpfObjects
	switchL link.Link
	wakingL link.Link
	goLink  link.Link
}

// Load attaches sched probes from a compiled BPF object (default bpf/collector.bpf.o).
func Load(targetTGID uint32, cfg Config, objectPath string) (*Collector, error) {
	path, err := ResolveObjectPath(objectPath)
	if err != nil {
		return nil, err
	}
	return loadObject(path, targetTGID, cfg)
}

func loadObject(path string, targetTGID uint32, cfg Config) (*Collector, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("rlimit memlock: %w", err)
	}

	spec, err := ebpf.LoadCollectionSpec(path)
	if err != nil {
		return nil, fmt.Errorf("load collection spec %s: %w", path, err)
	}

	var objs bpfObjects
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return nil, fmt.Errorf("assign bpf objects: %w", err)
	}

	k := uint32(0)
	bc := bpfConfig{
		MinBlockNs:  cfg.MinBlockNs,
		SampleMod:   cfg.SampleMod,
		Flags:       cfg.Flags,
		CookieTTLNs: cfg.CookieTTLNs,
	}
	if err := objs.Cfg.Put(&k, &bc); err != nil {
		return nil, errors.Join(fmt.Errorf("cfg map: %w", err), objs.close())
	}

	one := uint8(1)
	if err := objs.Targets.Put(&targetTGID, &one); err != nil {
		return nil, errors.Join(fmt.Errorf("targets map: %w", err), objs.close())
	}

	// tp_btf/* sections produce BPF_PROG_TYPE_TRACING — use AttachTracing, not RawTracepoint.
	sw, err := link.AttachTracing(link.TracingOptions{Program: objs.HandleSwitch})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("attach sched_switch: %w", err), objs.close())
	}

	wk, err := link.AttachTracing(link.TracingOptions{Program: objs.HandleWaking})
	if err != nil {
		sw.Close()
		return nil, errors.Join(fmt.Errorf("attach sched_waking: %w", err), objs.close())
	}

	return &Collector{
		objs:    objs,
		switchL: sw,
		wakingL: wk,
	}, nil
}

func (o *bpfObjects) close() error {
	var errs []error
	if o.Events != nil {
		errs = append(errs, o.Events.Close())
	}
	if o.Cfg != nil {
		errs = append(errs, o.Cfg.Close())
	}
	if o.Targets != nil {
		errs = append(errs, o.Targets.Close())
	}
	if o.Stats != nil {
		errs = append(errs, o.Stats.Close())
	}
	if o.TidToTask != nil {
		errs = append(errs, o.TidToTask.Close())
	}
	if o.GoCfgMap != nil {
		errs = append(errs, o.GoCfgMap.Close())
	}
	if o.HandleSwitch != nil {
		errs = append(errs, o.HandleSwitch.Close())
	}
	if o.HandleWaking != nil {
		errs = append(errs, o.HandleWaking.Close())
	}
	if o.UpCasgstatus != nil {
		errs = append(errs, o.UpCasgstatus.Close())
	}
	return errors.Join(errs...)
}

// Ringbuf returns the events ring buffer map.
func (c *Collector) Ringbuf() *ebpf.Map {
	return c.objs.Events
}

// Stats sums per-CPU stat counters (indices match bpf/event.h stat_idx).
func (c *Collector) Stats() ([event.StatMax]uint64, error) {
	var out [event.StatMax]uint64
	for i := uint32(0); i < event.StatMax; i++ {
		var perCPU []uint64
		if err := c.objs.Stats.Lookup(i, &perCPU); err != nil {
			return out, fmt.Errorf("stats lookup %d: %w", i, err)
		}
		for _, v := range perCPU {
			out[i] += v
		}
	}
	return out, nil
}

// Close detaches probes and releases maps.
func (c *Collector) Close() error {
	var errs []error
	if c.switchL != nil {
		errs = append(errs, c.switchL.Close())
	}
	if c.wakingL != nil {
		errs = append(errs, c.wakingL.Close())
	}
	if c.goLink != nil {
		errs = append(errs, c.goLink.Close())
	}
	errs = append(errs, c.objs.close())
	return errors.Join(errs...)
}
