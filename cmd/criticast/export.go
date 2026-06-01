package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/criticast/criticast/internal/analyzer"
	"github.com/criticast/criticast/internal/export"
	"github.com/criticast/criticast/internal/symbolize"
	"github.com/criticast/criticast/internal/trace"
)

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	pprofPath := fs.String("pprof", "", "write gzip pprof profile to path")
	otlp := fs.Bool("otlp", false, "export OTLP-Profiles (not implemented in P1)")
	request, _, minConf, spuriousUS := traceAnalyzeFlags(fs)
	sampleIdx := fs.Int("sample-index", -1, "reserved: sample index for multi-profile export")

	tracePath, flagArgs, err := parseTracePathArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if tracePath == "" {
		if fs.NArg() != 1 {
			return usageError("export", "usage: criticast export <trace> --pprof out.pb.gz")
		}
		tracePath = fs.Arg(0)
	} else if fs.NArg() > 0 {
		return usageError("export", "unexpected extra arguments")
	}
	_ = sampleIdx

	if *otlp {
		return exitErr(2, "export: OTLP-Profiles export is planned for P3 (use --pprof today)")
	}
	if *pprofPath == "" {
		return usageError("export", "--pprof is required")
	}

	tf, err := trace.ReadPath(tracePath)
	if err != nil {
		return fmt.Errorf("export: load trace: %w", err)
	}
	opts := analyzer.Options{
		RequestScope:   *request,
		MinConfidence:  uint8(*minConf),
		SpuriousWakeNs: *spuriousUS * 1000,
	}
	res, err := analyzer.Analyze(context.Background(), tf, opts)
	if err != nil {
		return fmt.Errorf("export: analyze: %w", err)
	}
	edges := res.CriticalPath.Edges
	if len(edges) == 0 && len(res.DominantWaits) > 0 {
		for _, rw := range res.DominantWaits {
			edges = append(edges, analyzer.PathEdge{WaitEdge: rw.WaitEdge})
		}
	}
	in := export.PprofInput{
		Edges:    edges,
		Resolver: symbolize.NewTraceResolver(tf.Stacks),
	}
	if err := export.WritePprof(*pprofPath, in); err != nil {
		return fmt.Errorf("export: pprof: %w", err)
	}
	fmt.Fprintf(os.Stderr, "criticast: wrote pprof %s (%d samples)\n", *pprofPath, len(edges))
	return nil
}
