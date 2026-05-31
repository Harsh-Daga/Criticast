package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/criticast/criticast/internal/gooffsets"
	"github.com/criticast/criticast/internal/loader"
)

func runGoSmoke(args []string) error {
	fs := flag.NewFlagSet("go-smoke", flag.ExitOnError)
	pid := fs.Uint("pid", 0, "target TGID")
	exe := fs.String("go-binary", "", "path to target executable (default /proc/PID/exe)")
	goVer := fs.String("go-version", "go1.22.0", "Go version key in bpf/offsets.json")
	durStr := fs.String("dur", "5s", "attach duration (5s, 30s, or plain seconds)")
	bpfObj := fs.String("bpf-object", "", "path to collector.bpf.o")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid == 0 {
		return fmt.Errorf("--pid is required")
	}
	dur, err := ParseCLIDuration(*durStr)
	if err != nil {
		return fmt.Errorf("--dur: %w", err)
	}
	if *exe == "" {
		*exe = fmt.Sprintf("/proc/%d/exe", *pid)
	}

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
	goidOff, err := db.GoidOffset(*goVer)
	if err != nil {
		return err
	}

	cfg := loader.Config{MinBlockNs: 1000, SampleMod: 1}
	coll, err := loader.Load(uint32(*pid), cfg, *bpfObj)
	if err != nil {
		return err
	}
	defer coll.Close()

	if err := coll.AttachGoUprobes(*exe, goidOff); err != nil {
		return fmt.Errorf("attach go uprobes: %w", err)
	}
	fmt.Printf("go-smoke: attached casgstatus (goid_off=%d) for %s\n", goidOff, *exe)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		fmt.Println("go-smoke: OK (no crash)")
		return nil
	}
}
