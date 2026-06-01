package analyzer

import (
	"context"
	"fmt"

	"github.com/criticast/criticast/internal/trace"
)

// Options configures trace analysis (charter Part E/J).
type Options struct {
	RequestScope   string
	MinConfidence  uint8
	TopN           int
	SpuriousWakeNs uint64
}

// Result is the full analysis output for CLI and export.
type Result struct {
	Summary       Summary
	CriticalPath  CriticalPath
	AmbiguousPath []PathEdge
	DominantWaits []RankedWait
	Scoped        bool
	ScopeCookie   uint64
	ScopeTid      uint32
	AmbiguousNs   uint64
	EdgeCount     int
}

// Analyze runs the Tier-0/1 pipeline on a decoded trace.
func Analyze(ctx context.Context, tf *trace.File, opts Options) (*Result, error) {
	if tf == nil {
		return nil, fmt.Errorf("analyzer: nil trace")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.SpuriousWakeNs == 0 {
		opts.SpuriousWakeNs = DefaultSpuriousWakeNs
	}
	if opts.TopN <= 0 {
		opts.TopN = 10
	}

	scopeCookie, scopeTid, scoped := ParseRequestScope(opts.RequestScope)
	byTask := BuildSegments(tf.Events)
	edges := BuildWaitEdges(tf.Events)
	edges = FilterByScope(edges, scopeCookie, scopeTid, scoped)
	edges = FilterSpuriousWakeups(edges, byTask, opts.SpuriousWakeNs)

	kept, ambEdges := FilterByConfidence(edges, opts.MinConfidence)
	comps := SCC(nil, kept)
	collapsed, _ := CollapseSCC(kept, comps)
	path := LongestPath(collapsed)

	var ambNs uint64
	var ambPath []PathEdge
	for _, e := range ambEdges {
		ambNs += e.BlockedNs
		ambPath = append(ambPath, PathEdge{WaitEdge: e})
	}

	res := &Result{
		Summary:       SummarizeSegments(byTask),
		CriticalPath:  path,
		AmbiguousPath: ambPath,
		DominantWaits: AggregateDominantWaits(edges, opts.TopN),
		Scoped:        scoped,
		ScopeCookie:   scopeCookie,
		ScopeTid:      scopeTid,
		AmbiguousNs:   ambNs,
		EdgeCount:     len(edges),
	}
	return res, nil
}
