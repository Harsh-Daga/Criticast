package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	bpfObj := fs.String("bpf-object", "", "path to collector.bpf.o")
	outPath := fs.String("out", "", "write trace file (JSONL)")
	chanCap := fs.Int("event-chan", agent.DefaultEventChanCap, "bounded channel capacity for events")
	goBinary := fs.String("go-binary", "", "attach casgstatus uprobes to this executable")
	goVersion := fs.String("go-version", "go1.22.0", "Go version for bpf/offsets.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid == 0 {
		return fmt.Errorf("--pid is required")
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

	cfg := loader.Config{
		MinBlockNs: minNs,
		SampleMod:  uint32(*sample),
	}

	coll, err := loader.Load(uint32(*pid), cfg, *bpfObj)
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
		off, err := db.GoidOffset(*goVersion)
		if err != nil {
			return err
		}
		if err := coll.AttachGoUprobes(*goBinary, off); err != nil {
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
			drainRecorder(errCh, rec, coll, consumed)
			if *outPath != "" {
				if err := writeTrace(*outPath, uint32(*pid), minNs, uint32(*sample), wallBase, ktimeBase, captured); err != nil {
					return err
				}
			}
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case <-timer.C:
			stop()
			drainRecorder(errCh, rec, coll, consumed)
			if *outPath != "" {
				if err := writeTrace(*outPath, uint32(*pid), minNs, uint32(*sample), wallBase, ktimeBase, captured); err != nil {
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
			printSummary(coll, rec, consumed)
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if *outPath != "" {
				if werr := writeTrace(*outPath, uint32(*pid), minNs, uint32(*sample), wallBase, ktimeBase, captured); werr != nil {
					return werr
				}
			}
			return nil
		}
	}
}

func writeTrace(path string, tgid uint32, minBlock uint64, sample uint32, wallBase time.Time, ktimeBase uint64, events []event.Event) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := trace.Header{
		Tgid:      tgid,
		MinBlock:  minBlock,
		SampleMod: sample,
	}
	if ktimeBase != 0 && !wallBase.IsZero() {
		hdr.KtimeBaseNs = ktimeBase
		hdr.WallBaseUTC = wallBase.Format(time.RFC3339Nano)
		hdr.Started = hdr.WallBaseUTC
	}
	return trace.Write(f, hdr, events)
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
	us := rec.StatsSnapshot()
	fmt.Printf("userspace: consumed=%d received=%d chan_drops=%d read_errors=%d malformed=%d\n",
		consumed, us.Received, us.ChanDrops, us.ReadErrors, us.Malformed)
	if stats, err := coll.Stats(); err == nil {
		printBPFStats(stats)
	} else {
		fmt.Fprintf(os.Stderr, "criticast: bpf stats: %v\n", err)
	}
}

func printBPFStats(stats [event.StatMax]uint64) {
	fmt.Printf("bpf stats: ringbuf_drops=%d emitted=%d blocks=%d preempts=%d runq=%d short_filt=%d sampled_out=%d stack_fail=%d\n",
		stats[event.StatRingbufDrops],
		stats[event.StatEventsEmitted],
		stats[event.StatBlocksSeen],
		stats[event.StatPreempts],
		stats[event.StatRunQClosed],
		stats[event.StatShortFiltered],
		stats[event.StatSampledOut],
		stats[event.StatStackFail],
	)
}
