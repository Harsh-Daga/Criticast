package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/criticast/criticast/internal/analyzer"
	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

func runEval(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	gtLog := fs.String("gt-log", "", "path to app log with CRITICAST_GT lines (or -)")
	tracePath := fs.String("trace", "", "optional trace JSONL from record --out")
	modeStr := fs.String("mode", "e1-lineage", "e1-lineage|e2-sudog|e3-suppress|e4-naive|all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *gtLog == "" {
		return fmt.Errorf("--gt-log is required")
	}

	records, err := groundtruth.ParseLogFile(*gtLog)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no CRITICAST_GT records in %s", *gtLog)
	}

	modes, err := modesFromFlag(*modeStr)
	if err != nil {
		return err
	}
	mechs := defaultMechanisms()

	for _, mode := range modes {
		fmt.Printf("=== %s ===\n", attribution.ModeName(mode))
		matrix, err := attribution.RunExperiment(mode, records, mechs)
		if err != nil {
			return err
		}
		printMatrix(matrix)
	}

	if *tracePath != "" {
		tf, err := trace.ReadPath(*tracePath)
		if err != nil {
			return err
		}
		evalTraceJoin(records, tf.Header, tf.Events, modes[0], mechs)
	}
	return nil
}

func modesFromFlag(s string) ([]attribution.Mode, error) {
	if s == "all" {
		return []attribution.Mode{
			attribution.E1LineageOnly,
			attribution.E2SudogElem,
			attribution.E3ResourceSuppress,
			attribution.E4NaiveForward,
		}, nil
	}
	m, err := attribution.ParseMode(s)
	if err != nil {
		return nil, err
	}
	return []attribution.Mode{m}, nil
}

func defaultMechanisms() []string {
	return []string{
		attribution.MechSpawnLineage,
		attribution.MechChanWorkHandoff,
		attribution.MechConnPool,
		attribution.MechMutex,
		attribution.MechBroadcast,
		attribution.MechNetpoll,
	}
}

func evalTraceJoin(records []groundtruth.Record, hdr trace.Header, events []event.Event, mode attribution.Mode, mechs []string) {
	tl := groundtruth.NewTimeline(records)
	joinSt := attribution.JoinStatsFromTrace(hdr, events, tl)
	traceEdges := attribution.EdgesFromTrace(hdr, events)
	gold, edges := attribution.LabelTraceEdges(traceEdges, tl)
	if len(edges) == 0 {
		fmt.Fprintf(os.Stderr,
			"eval: no labeled trace edges (block_ends=%d with_goid=%d labeled=%d clock_corr=%v; re-record with --go-binary and a current trace file)\n",
			joinSt.BlockEnds, joinSt.WithGoid, joinSt.Labeled, joinSt.ClockCorrelated)
		return
	}
	fmt.Printf("trace join: block_ends=%d with_goid=%d labeled=%d clock_corr=%v\n",
		joinSt.BlockEnds, joinSt.WithGoid, joinSt.Labeled, joinSt.ClockCorrelated)
	eng, err := attribution.NewEngine(mode, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		return
	}
	eng.ReplayGroundTruth(records)
	pred := make([]attribution.PredictedEdge, 0, len(edges))
	for i, te := range edges {
		mech := tl.MechanismAt(te.WakeeGoid, te.Ts)
		if mech == "" {
			mech = attribution.MechChanWorkHandoff
		}
		se := groundtruth.SiteEdge{
			Goid:      te.WakeeGoid,
			Token:     gold[i].WakeeToken,
			Mechanism: mech,
			TS:        te.Ts,
		}
		pred = append(pred, eng.Attribute(se, te.WakerGoid, tl.TokenAt(te.WakerGoid, te.Ts), te.SudogElem))
	}
	matrix, err := attribution.NewEvaluator(mode).Score(gold, pred, mechs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		return
	}
	fmt.Println("=== trace-joined ===")
	printMatrix(matrix)
	fmt.Printf("critical-path edge keys: %d\n", len(analyzer.PathKeySet(analyzer.BuildGraphs(edges, gold))))
}

func printMatrix(m attribution.Matrix) {
	fmt.Printf("%-22s %10s %10s %10s %8s\n", "mechanism", "precision", "recall", "false-edge", "edges")
	for _, row := range m {
		fmt.Printf("%-22s %10.3f %10.3f %10.3f %8d\n",
			row.Mechanism, row.Precision, row.Recall, row.FalseEdgeRate, row.EdgeCount)
	}
}
