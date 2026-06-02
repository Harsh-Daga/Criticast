package analyzer

import (
	"time"

	"github.com/criticast/criticast/internal/attribution"
)

func chanHandoffsForScope(env scopeEnv) []attribution.ChanHandoff {
	if env.tl == nil {
		return nil
	}
	from, to := env.from, env.to
	if from.IsZero() && env.kTo > env.kFrom {
		if t, ok := env.hdr.KtimeToWall(env.kFrom); ok {
			from = t
		}
		if t, ok := env.hdr.KtimeToWall(env.kTo); ok {
			to = t
		}
	}
	pad := 50 * time.Millisecond
	if !from.IsZero() {
		from = from.Add(-pad)
	}
	if !to.IsZero() {
		to = to.Add(pad)
	}
	return attribution.BuildChanHandoffsBetween(env.tl.AllRecords(), from, to)
}

// FilterScopedToken scopes edges by GT token + sudog handoff attribution (no pinned handler).
// For Bar B literal validation use --scope-handler-goid (request epoch path).
func FilterScopedToken(allEdges []WaitEdge, env scopeEnv) []WaitEdge {
	if !env.scoped || len(allEdges) == 0 {
		return allEdges
	}
	if env.scope.Token == "" || env.tl == nil {
		return FilterByScope(allEdges, env)
	}
	lin := attribution.ReplayLineageFromTimeline(env.tl)
	handoffs := chanHandoffsForScope(env)
	var pool []WaitEdge
	for _, e := range allEdges {
		if !env.edgeInWindow(e) {
			continue
		}
		if edgeInCookieScope(e, env, lin, handoffs) {
			pool = append(pool, e)
		}
	}
	if len(pool) == 0 {
		return pool
	}
	if len(env.handlerSeeds) > 0 {
		if fwd := filterEdgesReachableFromSeeds(pool, env.handlerSeeds, true); len(fwd) > 0 {
			return fwd
		}
		if reach := filterEdgesReachableFromSeeds(pool, env.handlerSeeds, false); len(reach) > 0 {
			return reach
		}
		if inc := filterEdgesIncidentToSeeds(pool, env.handlerSeeds); len(inc) > 0 {
			return inc
		}
	}
	if both := filterEdgesBothInRequestGoids(pool, env); len(both) > 0 {
		return both
	}
	return nil
}

// FilterScopedAttributed is a legacy name for FilterScopedToken.
func FilterScopedAttributed(allEdges []WaitEdge, env scopeEnv) []WaitEdge {
	return FilterScopedToken(allEdges, env)
}

// FilterScopedBarB is deprecated; use FilterScopedToken or the request epoch path.
func FilterScopedBarB(allEdges []WaitEdge, env scopeEnv) []WaitEdge {
	return FilterScopedToken(allEdges, env)
}

func edgeInCookieScope(e WaitEdge, env scopeEnv, lin *attribution.LineageStore, handoffs []attribution.ChanHandoff) bool {
	if len(env.requestGoids) > 0 && env.edgeTouchesRequestGoids(e) {
		return true
	}
	if env.hdr.KtimeBaseNs == 0 || env.hdr.WallBaseUTC == "" {
		return false
	}
	wakee, ok := env.nodeTaskID(e.To)
	if !ok {
		return false
	}
	ts := env.hdr.EventWallTime(e.EndNs)
	tok := tokenForEdge(env, lin, handoffs, wakee, ts, e.Aux)
	if tok != env.scope.Token {
		return false
	}
	if len(env.requestGoids) > 0 {
		if waker, ok := env.nodeTaskID(e.From); ok {
			if _, in := env.requestGoids[waker]; in {
				return true
			}
		}
	}
	return true
}

func tokenForEdge(env scopeEnv, lin *attribution.LineageStore, handoffs []attribution.ChanHandoff, wakee uint64, ts time.Time, aux uint64) string {
	te := attribution.TraceEdge{WakeeGoid: wakee, Ts: ts, SudogElem: aux}
	tok := attribution.TokenForWakee(env.tl, lin, te)
	if tok != "" {
		return tok
	}
	return attribution.TokenForChanHandoff(handoffs, wakee, ts)
}
