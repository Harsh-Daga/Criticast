package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "record":
		if err := runRecord(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	case "eval":
		if err := runEval(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	case "go-smoke":
		if err := runGoSmoke(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	case "probe-stats":
		if err := runProbeStats(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	case "analyze":
		if err := runAnalyze(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	case "export":
		if err := runExport(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(exitCode(err))
		}
	case "env":
		if err := runEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "criticast: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `criticast — per-request critical-path profiler

Usage:
  criticast env
  criticast record --pid <pid> --dur <sec> [--min-block 1us|50us] [--sample N] [--out trace.criticast]
  criticast analyze <trace> [--request cookie|tid] [--top N] [--format text|json]
  criticast export <trace> --pprof out.pb.gz [--request cookie|tid]
  criticast eval --gt-log <log> [--trace trace.criticast] [--mode e1-lineage|all]
  criticast go-smoke --pid <pid> [--go-binary /proc/PID/exe] [--go-version go1.22.0]
  criticast probe-stats --pid <pid> --dur 5s  (sched BPF counters; run wrk while waiting)

`)
}
