package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/criticast/criticast/internal/agent"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/gooffsets"
	"github.com/criticast/criticast/internal/loader"
	"github.com/criticast/criticast/internal/trace"
)

func runRecord(args []string) error {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	pid := fs.Uint("pid", 0, "target process TGID (pid of main thread)")
	durStr := fs.String("dur", "10s", "recording duration (30s, 5m, or plain seconds)")
	minBlock := fs.String("min-block", "1us", "minimum blocked duration to emit (1us|50us)")
	sample := fs.Uint("sample", 1, "emit 1/N blocks via random sampling")
	emitRunning := fs.Bool("emit-running", false, "emit EV_TASK_STATE RUNNING segments (diagnostics; increases event volume)")
	bpfObj := fs.String("bpf-object", "", "path to collector.bpf.o")
	outPath := fs.String("out", "", "write trace file (.criticast v2, or v1 .jsonl)")
	chanCap := fs.Int("event-chan", agent.DefaultEventChanCap, "bounded channel capacity for events")
	goBinary := fs.String("go-binary", "", "attach casgstatus uprobes to this executable")
	goVersion := fs.String("go-version", "go1.22.0", "Go version for bpf/offsets.json")
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
	if uint32(*pid) != tgid {
		fmt.Fprintf(os.Stderr, "criticast: --pid %d is thread; using tgid=%d for BPF target map\n", *pid, tgid)
	}
	if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", *pid)); err == nil {
		fmt.Fprintf(os.Stderr, "criticast: target comm=%q tgid=%d\n", strings.TrimSpace(string(comm)), tgid)
	}
	if *chanCap <= 0 {
		return fmt.Errorf("--event-chan must be > 0")
	}

	minNs, err := ParseDurationNS(*minBlock)
	if err != nil {
		return fmt.Errorf("--min-block: %w", err)
	}
	if *sample == 0 {
		return fmt.Errorf("--sample must be >= 1")
	}
	dur, err := ParseCLIDuration(*durStr)
	if err != nil {
		return fmt.Errorf("--dur: %w", err)
	}

	cfg := loader.DefaultConfig()
	cfg.MinBlockNs = minNs
	cfg.SampleMod = uint32(*sample)
	if *emitRunning {
		cfg.Flags |= loader.CfgEmitRunning
	}

	coll, err := loader.Load(tgid, cfg, *bpfObj)
	if err != nil {
		return err
	}
	defer coll.Close()

	if *goBinary != "" {
		root, err := repoRoot()
		if err != nil {
			return err
		}
		offPath, err := gooffsets.ResolvePath(root)
		if err != nil {
			return err
		}
		db, err := gooffsets.Load(offPath)
		if err != nil {
			return err
		}
		resolved, err := db.ResolveGoVersion(*goVersion)
		if err != nil {
			return err
		}
		if resolved != *goVersion {
			fmt.Fprintf(os.Stderr, "criticast: offsets: using %s for requested %s\n", resolved, *goVersion)
		}
		probeOff, err := db.ProbeOffsets(resolved)
		if err != nil {
			return err
		}
		if err := coll.AttachGoUprobes(*goBinary, loader.GoProbeOffsets(probeOff)); err != nil {
			return fmt.Errorf("go uprobes: %w", err)
		}
	}

	rec, err := agent.NewRecorder(coll.Ringbuf(), *chanCap)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- rec.Run(ctx)
	}()

	timer := time.NewTimer(dur)
	defer timer.Stop()

	var captured []event.Event
	var consumed uint64
	var wallBase time.Time
	var ktimeBase uint64
	for {
		select {
		case <-ctx.Done():
			stop()
			consumed, captured, wallBase, ktimeBase = drainEventCh(rec, consumed, captured, wallBase, ktimeBase, *outPath != "")
			drainRecorder(errCh, rec, coll, consumed)
			if *outPath != "" {
				if err := writeTrace(*outPath, tgid, minNs, uint32(*sample), wallBase, ktimeBase, captured, coll, rec, *goBinary); err != nil {
					return err
				}
			}
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case <-timer.C:
			stop()
			consumed, captured, wallBase, ktimeBase = drainEventCh(rec, consumed, captured, wallBase, ktimeBase, *outPath != "")
			drainRecorder(errCh, rec, coll, consumed)
			if *outPath != "" {
				if err := writeTrace(*outPath, tgid, minNs, uint32(*sample), wallBase, ktimeBase, captured, coll, rec, *goBinary); err != nil {
					return err
				}
			}
			return nil
		case ev, ok := <-rec.Events():
			if !ok {
				printSummary(coll, rec, consumed)
				return nil
			}
			consumed++
			if *outPath != "" {
				if ktimeBase == 0 {
					ktimeBase = ev.TsNs
					wallBase = time.Now().UTC()
				}
				captured = append(captured, ev)
			}
		case err := <-errCh:
			consumed, captured, wallBase, ktimeBase = drainEventCh(rec, consumed, captured, wallBase, ktimeBase, *outPath != "")
			printSummary(coll, rec, consumed)
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if *outPath != "" {
				if werr := writeTrace(*outPath, tgid, minNs, uint32(*sample), wallBase, ktimeBase, captured, coll, rec, *goBinary); werr != nil {
					return werr
				}
			}
			return nil
		}
	}
}

// drainEventCh reads remaining events after stop (non-blocking).
func drainEventCh(rec *agent.Recorder, consumed uint64, captured []event.Event, wallBase time.Time, ktimeBase uint64, keep bool) (uint64, []event.Event, time.Time, uint64) {
	for {
		select {
		case ev, ok := <-rec.Events():
			if !ok {
				return consumed, captured, wallBase, ktimeBase
			}
			consumed++
			if keep {
				if ktimeBase == 0 {
					ktimeBase = ev.TsNs
					wallBase = time.Now().UTC()
				}
				captured = append(captured, ev)
			}
		default:
			return consumed, captured, wallBase, ktimeBase
		}
	}
}

func writeTrace(path string, tgid uint32, minBlock uint64, sample uint32, wallBase time.Time, ktimeBase uint64, events []event.Event, coll *loader.Collector, rec *agent.Recorder, targetBinary string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := trace.Header{
		Tgid:         tgid,
		MinBlock:     minBlock,
		SampleMod:    sample,
		TargetBinary: targetBinary,
	}
	if ktimeBase != 0 && !wallBase.IsZero() {
		hdr.KtimeBaseNs = ktimeBase
		hdr.WallBaseUTC = wallBase.Format(time.RFC3339Nano)
		hdr.Started = hdr.WallBaseUTC
	}
	opts := &trace.WriteOptions{
		Stacks:  map[int32][]uint64{},
		Modules: captureTraceModules(tgid, targetBinary),
	}
	us := rec.StatsSnapshot()
	opts.Footer = &trace.Footer{
		UserspaceReceived:   us.Received,
		UserspaceChanDrops:  us.ChanDrops,
		UserspaceReadErrors: us.ReadErrors,
		UserspaceMalformed:  us.Malformed,
	}
	if stats, err := coll.Stats(); err == nil {
		opts.Footer.BPFRingbufDrops = stats[event.StatRingbufDrops]
		opts.Footer.BPFEventsEmitted = stats[event.StatEventsEmitted]
		opts.Footer.BPFBlocksSeen = stats[event.StatBlocksSeen]
		opts.Footer.BPFPreempts = stats[event.StatPreempts]
		opts.Footer.BPFRunQClosed = stats[event.StatRunQClosed]
		opts.Footer.BPFShortFiltered = stats[event.StatShortFiltered]
		opts.Footer.BPFSampledOut = stats[event.StatSampledOut]
		opts.Footer.BPFStackFail = stats[event.StatStackFail]
	}
	return trace.Write(f, hdr, events, opts)
}

func drainRecorder(errCh <-chan error, rec *agent.Recorder, coll *loader.Collector, consumed uint64) {
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "criticast: recorder: %v\n", err)
		}
	case <-time.After(3 * time.Second):
		fmt.Fprintln(os.Stderr, "criticast: recorder stop timed out after 3s")
	}
	printSummary(coll, rec, consumed)
}

func printSummary(coll *loader.Collector, rec *agent.Recorder, consumed uint64) {
	warnStackFail(coll)
	us := rec.StatsSnapshot()
	fmt.Fprintf(os.Stderr, "userspace: consumed=%d received=%d chan_drops=%d read_errors=%d malformed=%d\n",
		consumed, us.Received, us.ChanDrops, us.ReadErrors, us.Malformed)
	if stats, err := coll.Stats(); err == nil {
		printBPFStats(stats)
		warnIfNoEvents(stats, us.Received, consumed)
	} else {
		fmt.Fprintf(os.Stderr, "criticast: bpf stats: %v\n", err)
	}
}

func printBPFStats(stats [event.StatMax]uint64) {
	fmt.Fprintf(os.Stderr, "bpf stats: ringbuf_drops=%d emitted=%d blocks=%d preempts=%d runq=%d running=%d short_filt=%d sampled_out=%d stack_fail=%d switch_seen=%d target_prev=%d\n",
		stats[event.StatRingbufDrops],
		stats[event.StatEventsEmitted],
		stats[event.StatBlocksSeen],
		stats[event.StatPreempts],
		stats[event.StatRunQClosed],
		stats[event.StatRunningEmitted],
		stats[event.StatShortFiltered],
		stats[event.StatSampledOut],
		stats[event.StatStackFail],
		stats[event.StatSwitchSeen],
		stats[event.StatTargetPrev],
	)
}

func warnStackFail(coll *loader.Collector) {
	stats, err := coll.Stats()
	if err != nil {
		return
	}
	emitted := stats[event.StatEventsEmitted]
	if emitted == 0 {
		return
	}
	fail := stats[event.StatStackFail]
	if fail*100/emitted > 10 {
		fmt.Fprintf(os.Stderr,
			"criticast: warning: stack capture failed on %d/%d events (%.0f%%); waker stacks may be missing\n",
			fail, emitted, 100*float64(fail)/float64(emitted))
	}
}

func warnIfNoEvents(stats [event.StatMax]uint64, received, consumed uint64) {
	emitted := stats[event.StatEventsEmitted]
	if emitted == 0 && received == 0 && consumed == 0 {
		fmt.Fprintln(os.Stderr, "criticast: no BPF events captured — common causes:")
		fmt.Fprintln(os.Stderr, "  - wrong process (use bin/httpgo from this repo, not another binary named httpgo)")
		fmt.Fprintln(os.Stderr, "  - no load during record (run wrk/curl against the target while recording)")
		fmt.Fprintln(os.Stderr, "  - target idle with no scheduler activity in the window")
	}
}
