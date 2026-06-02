package analyzer

import (
	"fmt"
	"time"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/trace"
)

const epochPathWeightSlackNs = 2_000_000

// analyzeRequestEpoch computes the critical path for a pinned handler epoch (Bar B literal).
// edges must already be epoch-filtered, spurious-filtered, and GT-enriched.
func analyzeRequestEpoch(
	epoch RequestEpoch,
	env scopeEnv,
	edges []WaitEdge,
	byTask map[TaskKey][]Segment,
	opts Options,
) (CriticalPath, error) {
	path, err := computeEpochCriticalPath(edges, byTask, epoch, env.handlerSeeds, opts.MinConfidence)
	if err != nil {
		return CriticalPath{}, err
	}
	enrichCriticalPathFromGT(&path, env)
	if !PathWeightInvariantOK(path, epoch.WallNs, epochPathWeightSlackNs) {
		return CriticalPath{}, fmt.Errorf(
			"analyzer: path_weight %dns exceeds scoped handler window %dns (+slack); temporal path or attribution scope leak",
			path.PathWeight, epoch.WallNs,
		)
	}
	return path, nil
}

// prepareTokenScopeEnv fills request goids for token= scope without a pinned handler.
func prepareTokenScopeEnv(
	hdr trace.Header,
	events []event.Event,
	tl *groundtruth.Timeline,
	token string,
	winFrom, winTo time.Time,
	pad time.Duration,
) (kFrom, kTo uint64, requestGoids map[uint64]struct{}, err error) {
	kFrom, kTo, ok := applyScopeWindow(hdr, winFrom, winTo, pad)
	if !ok {
		return 0, 0, nil, fmt.Errorf("analyzer: trace missing wall_base_utc/ktime_base_ns for scope window")
	}
	if token == "" || tl == nil {
		return kFrom, kTo, nil, nil
	}
	strictFrom, strictTo := winFrom, winTo
	gtGoids := buildRequestGoids(tl, token, winFrom, winTo, pad)
	requestGoids = calibrateRequestGoids(hdr, events, tl, token, gtGoids, strictFrom, strictTo)
	requestGoids = expandRequestGoidsByCookie(
		hdr, events, tl, token, requestGoids, strictFrom, strictTo, kFrom, kTo,
	)
	return kFrom, kTo, requestGoids, nil
}

// analyzeScopedToken computes critical path for token= scope with GT (not handler-pinned).
func analyzeScopedToken(
	tf *trace.File,
	env scopeEnv,
	edges []WaitEdge,
	byTask map[TaskKey][]Segment,
	opts Options,
	winFrom, winTo time.Time,
) (CriticalPath, []WaitEdge, error) {
	_ = byTask
	pathPol := PathPolicyForScope(true)
	pathEdges := FilterPathCandidates(edges, pathPol)
	if env.kTo > env.kFrom {
		pathEdges = ApplyWindowClip(pathEdges, env.kFrom, env.kTo)
	}
	kept, ambEdges := FilterByConfidence(pathEdges, opts.MinConfidence)
	kept = PreparePathEdgesMax(kept)
	collapsed, _ := CollapseSCC(kept, SCC(nil, kept))

	var path CriticalPath
	if len(env.handlerSeeds) > 0 {
		path = handlerTemporalCriticalPath(collapsed, env.handlerSeeds, DefaultTemporalTolNs)
	} else {
		path = LongestPathTemporal(collapsed, DefaultTemporalTolNs)
	}
	if env.scope.Token != "" && len(path.Edges) > 0 {
		enrichCriticalPathFromGT(&path, env)
	}
	if !winFrom.IsZero() && !winTo.IsZero() {
		if kStrictFrom, kStrictTo, ok := applyScopeWindowStrict(tf.Header, winFrom, winTo); ok && kStrictTo > kStrictFrom {
			if !PathWeightInvariantOK(path, kStrictTo-kStrictFrom, epochPathWeightSlackNs) {
				return CriticalPath{}, ambEdges, fmt.Errorf(
					"analyzer: path_weight %dns exceeds scoped handler window %dns (+slack); temporal path or attribution scope leak",
					path.PathWeight, kStrictTo-kStrictFrom,
				)
			}
		}
	}
	return path, ambEdges, nil
}

// analyzeUnscoped is Tier-0 process-wide dominant waits.
func analyzeUnscoped(
	env scopeEnv,
	edges []WaitEdge,
	opts Options,
	scoped bool,
) (CriticalPath, []WaitEdge, error) {
	pathPol := PathPolicyForScope(scoped)
	pathEdges := FilterPathCandidates(edges, pathPol)
	kept, ambEdges := FilterByConfidence(pathEdges, opts.MinConfidence)
	kept = PreparePathEdges(kept)
	collapsed, _ := CollapseSCC(kept, SCC(nil, kept))
	return LongestPath(collapsed), ambEdges, nil
}
