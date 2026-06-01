package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/loader"
)

// probe-stats: attach sched BPF to a TGID, wait, print per-CPU stats (no ringbuf drain).
// Use with wrk/load in another terminal to verify the kernel path on this host.
func runProbeStats(args []string) error {
	fs := flag.NewFlagSet("probe-stats", flag.ExitOnError)
	pid := fs.Uint("pid", 0, "target PID (any thread in group)")
	durStr := fs.String("dur", "5s", "wait duration")
	bpfObj := fs.String("bpf-object", "", "path to collector.bpf.o")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid == 0 {
		return fmt.Errorf("--pid is required")
	}
	tgid, err := resolveTGID(int(*pid))
	if err != nil {
		return err
	}
	dur, err := ParseCLIDuration(*durStr)
	if err != nil {
		return err
	}

	coll, err := loader.Load(tgid, loader.Config{MinBlockNs: 1000, SampleMod: 1}, *bpfObj)
	if err != nil {
		return err
	}
	defer coll.Close()

	if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", *pid)); err == nil {
		fmt.Printf("probe-stats: comm=%q tgid=%d (generate load now, e.g. wrk)\n", strings.TrimSpace(string(comm)), tgid)
	} else {
		fmt.Printf("probe-stats: tgid=%d\n", tgid)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}

	stats, err := coll.Stats()
	if err != nil {
		return err
	}
	printBPFStats(stats)
	if stats[event.StatEventsEmitted] == 0 && stats[event.StatPreempts] == 0 && stats[event.StatShortFiltered] == 0 {
		fmt.Fprintln(os.Stderr, "probe-stats: no BPF activity — run ./scripts/sched-smoke.sh or check lockdown/paranoid")
		return fmt.Errorf("probe-stats: zero scheduler stats")
	}
	return nil
}
