package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/criticast/criticast/internal/analyzer"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/symbolize"
	"github.com/criticast/criticast/internal/trace"
)

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	request, topN, minConf, spuriousUS := traceAnalyzeFlags(fs)
	format := fs.String("format", "text", "output format: text|json")

	tracePath, flagArgs, err := parseTracePathArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if tracePath == "" {
		if fs.NArg() != 1 {
			return usageError("analyze", "usage: criticast analyze <trace> [--request …] [--format text|json]")
		}
		tracePath = fs.Arg(0)
	} else if fs.NArg() > 0 {
		return usageError("analyze", "unexpected extra arguments")
	}
	if *format != "text" && *format != "json" {
		return usageError("analyze", "--format must be text or json")
	}

	tf, err := trace.ReadPath(tracePath)
	if err != nil {
		return fmt.Errorf("analyze: load trace: %w", err)
	}

	opts := analyzer.Options{
		RequestScope:   *request,
		MinConfidence:  uint8(*minConf),
		TopN:           *topN,
		SpuriousWakeNs: *spuriousUS * 1000,
	}
	res, err := analyzer.Analyze(context.Background(), tf, opts)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	resolver := symbolize.NewTraceResolver(tf.Stacks)
	if *format == "json" {
		return writeAnalyzeJSON(os.Stdout, tf, res, resolver)
	}
	return writeAnalyzeText(os.Stdout, tf, res, resolver)
}

func writeAnalyzeText(w io.Writer, tf *trace.File, res *analyzer.Result, resolver symbolize.Resolver) error {
	fmt.Fprintf(w, "Trace: format=v%d tgid=%d events=%d edges=%d\n",
		tf.Format, tf.Header.Tgid, len(tf.Events), res.EdgeCount)
	if tf.Footer != nil {
		fmt.Fprintf(w, "Footer: userspace_received=%d bpf_emitted=%d bpf_ringbuf_drops=%d\n",
			tf.Footer.UserspaceReceived, tf.Footer.BPFEventsEmitted, tf.Footer.BPFRingbufDrops)
	}

	scope := "all requests (Tier-0/1)"
	if res.Scoped {
		if res.ScopeCookie != 0 {
			scope = fmt.Sprintf("cookie=0x%x", res.ScopeCookie)
		} else {
			scope = fmt.Sprintf("tid=%d", res.ScopeTid)
		}
	}
	pctAmb := float64(0)
	if res.CriticalPath.PathWeight+res.AmbiguousNs > 0 {
		pctAmb = 100 * float64(res.AmbiguousNs) / float64(res.CriticalPath.PathWeight+res.AmbiguousNs)
	}
	fmt.Fprintf(w, "\nRequest: %s\n", scope)
	fmt.Fprintf(w, "Path weight: %s", formatDuration(res.CriticalPath.PathWeight))
	if res.AmbiguousNs > 0 {
		fmt.Fprintf(w, "  |  Ambiguous: %s (%.0f%%)", formatDuration(res.AmbiguousNs), pctAmb)
	}
	fmt.Fprintln(w)

	if len(res.CriticalPath.Edges) > 0 {
		fmt.Fprintf(w, "\nCRITICAL PATH (confidence ≥ %d):\n", res.CriticalPath.Edges[0].Meta.Confidence)
		for _, pe := range res.CriticalPath.Edges {
			writePathEdge(w, pe, resolver)
		}
	} else {
		fmt.Fprintln(w, "\nCRITICAL PATH: (no edges above confidence threshold)")
	}

	if len(res.AmbiguousPath) > 0 {
		fmt.Fprintln(w, "\nAmbiguous waits (shared resource / low confidence):")
		limit := len(res.AmbiguousPath)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			writePathEdge(w, res.AmbiguousPath[i], resolver)
		}
	}

	fmt.Fprintf(w, "\nDominant waits (top %d):\n", len(res.DominantWaits))
	for _, rw := range res.DominantWaits {
		tag := ""
		if rw.Meta.Ambiguous {
			tag = "  ambiguous"
		}
		fmt.Fprintf(w, "  %s  %s  %s → %s  conf=%d%%%s  (×%d)\n",
			formatDuration(rw.BlockedNs), waitClassName(rw.WaitClass),
			analyzer.FormatNode(rw.From), analyzer.FormatNode(rw.To),
			rw.Meta.Confidence, tag, rw.Count)
	}
	return nil
}

func writePathEdge(w io.Writer, pe analyzer.PathEdge, resolver symbolize.Resolver) {
	tag := ""
	if pe.Meta.Ambiguous {
		tag = "  ambiguous"
		if pe.WaitClass == event.WCChan && pe.Aux == 0 {
			tag += " (no elem)"
		}
	}
	fmt.Fprintf(w, "  %s  %s  %s ← %s  conf=%d%%%s\n",
		formatDuration(pe.BlockedNs), waitClassName(pe.WaitClass),
		analyzer.FormatNode(pe.To), analyzer.FormatNode(pe.From),
		pe.Meta.Confidence, tag)
	if frames, _ := resolver.Resolve(pe.WakerStkID); len(frames) > 0 {
		fmt.Fprintf(w, "    waker: %s\n", frames[0].Function)
	}
}

type analyzeJSON struct {
	TraceVersion int                   `json:"trace_version"`
	Request      string                `json:"request_scope"`
	Summary      analyzer.Summary      `json:"summary"`
	PathWeight   uint64                `json:"path_weight_ns"`
	AmbiguousNs  uint64                `json:"ambiguous_ns"`
	CriticalPath []analyzeJSONPathEdge `json:"critical_path"`
	Dominant     []analyzeJSONRanked   `json:"dominant_waits"`
}

type analyzeJSONPathEdge struct {
	BlockedNs  uint64 `json:"blocked_ns"`
	WaitClass  string `json:"wait_class"`
	From       string `json:"from"`
	To         string `json:"to"`
	Confidence uint8  `json:"confidence"`
	Ambiguous  bool   `json:"ambiguous"`
}

type analyzeJSONRanked struct {
	analyzeJSONPathEdge
	Count int `json:"count"`
}

func writeAnalyzeJSON(w io.Writer, tf *trace.File, res *analyzer.Result, _ symbolize.Resolver) error {
	out := analyzeJSON{
		TraceVersion: tf.Format,
		Summary:      res.Summary,
		PathWeight:   res.CriticalPath.PathWeight,
		AmbiguousNs:  res.AmbiguousNs,
	}
	if res.Scoped {
		if res.ScopeCookie != 0 {
			out.Request = fmt.Sprintf("0x%x", res.ScopeCookie)
		} else {
			out.Request = fmt.Sprintf("tid:%d", res.ScopeTid)
		}
	}
	for _, pe := range res.CriticalPath.Edges {
		out.CriticalPath = append(out.CriticalPath, pathEdgeJSON(pe))
	}
	for _, rw := range res.DominantWaits {
		out.Dominant = append(out.Dominant, analyzeJSONRanked{
			analyzeJSONPathEdge: pathEdgeJSON(analyzer.PathEdge{WaitEdge: rw.WaitEdge}),
			Count:               rw.Count,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func pathEdgeJSON(pe analyzer.PathEdge) analyzeJSONPathEdge {
	return analyzeJSONPathEdge{
		BlockedNs:  pe.BlockedNs,
		WaitClass:  waitClassName(pe.WaitClass),
		From:       analyzer.FormatNode(pe.From),
		To:         analyzer.FormatNode(pe.To),
		Confidence: pe.Meta.Confidence,
		Ambiguous:  pe.Meta.Ambiguous,
	}
}

func formatDuration(ns uint64) string {
	if ns >= 1_000_000 {
		return fmt.Sprintf("%.2fms", float64(ns)/1e6)
	}
	if ns >= 1_000 {
		return fmt.Sprintf("%.2fus", float64(ns)/1e3)
	}
	return fmt.Sprintf("%dns", ns)
}

func waitClassName(wc event.WaitClass) string {
	names := []string{
		"WC_UNKNOWN", "WC_FUTEX", "WC_EPOLL", "WC_IOURING", "WC_NET", "WC_DISK",
		"WC_RUNQ", "WC_SLEEP", "WC_GC", "WC_CHAN", "WC_MUTEX", "WC_SELECT", "WC_SEMA", "WC_COND",
	}
	if int(wc) < len(names) {
		return names[wc]
	}
	return fmt.Sprintf("WC_%d", wc)
}
