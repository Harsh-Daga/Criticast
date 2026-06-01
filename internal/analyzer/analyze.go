package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/criticast/criticast/internal/trace"
)

// Options configures trace analysis (charter Part E/J).
type Options struct {
	RequestScope   string
	GtLog          string // required for token= scope (--gt-log)
	ScopeFromUTC   string // optional RFC3339 wall window (one request; with token=)
	ScopeToUTC     string
	ScopePad         time.Duration // padding each side of scope window (default 10ms)
	ScopeHandlerGoid uint64        // optional: pin single handler goid for Bar B window
	MinConfidence    uint8
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
	ScopeTaskID   uint64
	ScopeToken    string
	AmbiguousNs   uint64
	EdgeCount        int
	WindowEdgeCount  int // edges overlapping ktime scope window (pre-goid filter)
	RequestGoidCount int
	CalibratedGoids   bool
	UnscopedNote      string
	// EpochWallNs and HandlerOccupancyNs are set for Bar B literal (handler-pinned) runs only.
	EpochWallNs          uint64
	HandlerOccupancyNs   uint64
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

	scope, scoped := ParseRequestScope(opts.RequestScope)
	tl, err := loadScopeTimeline(opts.GtLog, scope)
	if err != nil {
		return nil, err
	}
	winFrom, winTo, err := parseScopeWindow(opts.ScopeFromUTC, opts.ScopeToUTC)
	if err != nil {
		return nil, err
	}
	env := scopeEnv{
		scope:       scope,
		scoped:      scoped,
		hdr:         tf.Header,
		tl:          tl,
		from:        winFrom,
		to:          winTo,
		tidToTaskID: buildTidToTaskID(tf.Events),
	}

	var epoch RequestEpoch
	epochLiteral := isRequestEpochLiteral(opts, scope, winFrom) && tl != nil
	pad := opts.ScopePad
	if pad == 0 {
		pad = defaultScopePad
	}

	if !winFrom.IsZero() {
		if epochLiteral {
			epoch, env.requestGoids, env.handlerSeeds, err = buildRequestEpoch(
				tf.Header, tf.Events, tl, scope.Token, opts.ScopeHandlerGoid, winFrom, winTo, pad,
			)
			if err != nil {
				return nil, err
			}
			env.handlerTask = epoch.HandlerTask
			env.kFrom, env.kTo = epoch.KPaddedFrom, epoch.KPaddedTo
			env.traceWaitCls = buildTraceWaitClassFromGT(
				tf.Header, tf.Events, tl, scope.Token, winFrom, winTo,
			)
		} else {
			env.kFrom, env.kTo, env.requestGoids, err = prepareTokenScopeEnv(
				tf.Header, tf.Events, tl, scope.Token, winFrom, winTo, pad,
			)
			if err != nil {
				return nil, err
			}
			if scope.Token != "" && tl != nil {
				env.traceWaitCls = buildTraceWaitClassFromGT(
					tf.Header, tf.Events, tl, scope.Token, winFrom, winTo,
				)
			}
		}
	}

	byTask := BuildSegments(tf.Events)
	allEdges := BuildWaitEdges(tf.Events)
	windowEdges := countWindowEdges(allEdges, env)
	calibrated := env.kTo > env.kFrom && scope.Token != "" && len(env.requestGoids) > 0

	var edges []WaitEdge
	switch {
	case epochLiteral:
		edges = filterEpochEdges(allEdges, env, epoch)
	case scope.Token != "" && env.tl != nil && env.kTo > env.kFrom:
		edges = FilterScopedToken(allEdges, env)
	default:
		edges = FilterByScope(allEdges, env)
		if env.scoped {
			edges = FilterScopedSubgraph(edges, env)
		}
	}
	edges = FilterSpuriousWakeups(edges, byTask, opts.SpuriousWakeNs)
	if scope.Token != "" && env.tl != nil {
		enrichScopedEdgesFromGT(edges, env)
	}

	var path CriticalPath
	var ambEdges []WaitEdge
	switch {
	case epochLiteral:
		path, err = analyzeRequestEpoch(epoch, env, edges, byTask, opts)
	case scope.Token != "" && env.tl != nil && env.kTo > env.kFrom:
		path, ambEdges, err = analyzeScopedToken(tf, env, edges, byTask, opts, winFrom, winTo)
	default:
		path, ambEdges, err = analyzeUnscoped(env, edges, opts, scoped)
	}
	if err != nil {
		return nil, err
	}

	res := buildAnalyzeResult(byTask, path, ambEdges, edges, env, scope, scoped, calibrated, windowEdges, opts.TopN)
	if epochLiteral {
		res.EpochWallNs = epoch.WallNs
		res.HandlerOccupancyNs = measuredHandlerOccupancyNs(
			byTask, epoch.HandlerTask, epoch.KStrictFrom, epoch.KStrictTo,
		)
	}
	return res, nil
}

func countWindowEdges(allEdges []WaitEdge, env scopeEnv) int {
	if env.kTo <= env.kFrom {
		return 0
	}
	n := 0
	for _, e := range allEdges {
		if env.edgeInWindow(e) {
			n++
		}
	}
	return n
}

func buildAnalyzeResult(
	byTask map[TaskKey][]Segment,
	path CriticalPath,
	ambEdges []WaitEdge,
	edges []WaitEdge,
	env scopeEnv,
	scope RequestScope,
	scoped bool,
	calibrated bool,
	windowEdges int,
	topN int,
) *Result {
	var ambNs uint64
	var ambPath []PathEdge
	for _, e := range ambEdges {
		ambNs += e.BlockedNs
		ambPath = append(ambPath, PathEdge{WaitEdge: e})
	}
	return &Result{
		Summary:          SummarizeSegments(byTask),
		CriticalPath:     path,
		AmbiguousPath:    ambPath,
		DominantWaits:    AggregateDominantWaits(edges, topN),
		Scoped:           scoped,
		ScopeCookie:      scope.Cookie,
		ScopeTid:         scope.Tid,
		ScopeTaskID:      scope.TaskID,
		ScopeToken:       scope.Token,
		AmbiguousNs:      ambNs,
		EdgeCount:        len(edges),
		WindowEdgeCount:  windowEdges,
		RequestGoidCount: len(env.requestGoids),
		CalibratedGoids:  calibrated,
		UnscopedNote:     unscopedModeNote(scoped),
	}
}

func unscopedModeNote(scoped bool) string {
	if scoped {
		return ""
	}
	return "process-wide dominant waits (Tier-0); use --request for per-request critical path"
}
