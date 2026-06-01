package analyzer

import (
	"fmt"
	"time"

	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/mechanism"
	"github.com/criticast/criticast/internal/trace"
)

// scopeEnv carries GT join state for token-scoped analysis.
type scopeEnv struct {
	scope        RequestScope
	scoped       bool
	hdr          trace.Header
	tl           *groundtruth.Timeline
	from         time.Time // wall-clock window (Bar B: one handler entry→exit)
	to           time.Time
	kFrom        uint64 // bpf_ktime window (preferred for filtering block_ends)
	kTo          uint64
	requestGoids  map[uint64]struct{}       // calibrated BPF task_ids for this request span
	handlerSeeds  map[NodeID]struct{}       // handler node(s) for Bar B reachability root
	handlerTask   uint64                    // BPF task_id for pinned handler (Bar B timeline weight)
	tidToTaskID   map[uint32]uint64         // tid→goid from trace (when nodes use tid bit)
	traceWaitCls  map[uint64]event.WaitClass // GT site → wait_class on trace task_id
}

func parseScopeWindow(fromUTC, toUTC string) (from, to time.Time, err error) {
	if fromUTC == "" && toUTC == "" {
		return time.Time{}, time.Time{}, nil
	}
	if fromUTC == "" || toUTC == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("analyzer: scope window requires both --scope-from and --scope-to")
	}
	from, err = time.Parse(time.RFC3339Nano, fromUTC)
	if err != nil {
		from, err = time.Parse(time.RFC3339, fromUTC)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("analyzer: --scope-from: %w", err)
		}
	}
	to, err = time.Parse(time.RFC3339Nano, toUTC)
	if err != nil {
		to, err = time.Parse(time.RFC3339, toUTC)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("analyzer: --scope-to: %w", err)
		}
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("analyzer: scope window: to must be after from")
	}
	return from, to, nil
}

const defaultScopePad = 10 * time.Millisecond

func applyScopeWindow(hdr trace.Header, from, to time.Time, pad time.Duration) (kFrom, kTo uint64, ok bool) {
	if from.IsZero() && to.IsZero() {
		return 0, 0, false
	}
	if pad == 0 {
		pad = defaultScopePad
	}
	from = from.UTC().Add(-pad)
	to = to.UTC().Add(pad)
	kFrom, ok1 := hdr.WallToKtime(from)
	kTo, ok2 := hdr.WallToKtime(to)
	return kFrom, kTo, ok1 && ok2
}

// applyScopeWindowStrict maps handler entry→exit without padding (Bar B wall invariant).
func applyScopeWindowStrict(hdr trace.Header, from, to time.Time) (kFrom, kTo uint64, ok bool) {
	if from.IsZero() && to.IsZero() {
		return 0, 0, false
	}
	kFrom, ok1 := hdr.WallToKtime(from.UTC())
	kTo, ok2 := hdr.WallToKtime(to.UTC())
	return kFrom, kTo, ok1 && ok2
}

func buildRequestGoids(tl *groundtruth.Timeline, token string, from, to time.Time, pad time.Duration) map[uint64]struct{} {
	if tl == nil || token == "" || from.IsZero() {
		return nil
	}
	if pad == 0 {
		pad = defaultScopePad
	}
	return tl.GoidsWithTokenBetween(token, from.UTC().Add(-pad), to.UTC().Add(pad))
}

func (env scopeEnv) edgeInWindow(e WaitEdge) bool {
	if env.kTo != 0 || env.kFrom != 0 {
		if env.kTo < env.kFrom {
			return false
		}
		// Interval overlap in bpf_ktime domain.
		return e.EndNs >= env.kFrom && e.StartNs <= env.kTo
	}
	if env.from.IsZero() {
		return true
	}
	ts := env.hdr.EventWallTime(e.EndNs)
	return !ts.Before(env.from) && !ts.After(env.to)
}

func loadScopeTimeline(gtLog string, scope RequestScope) (*groundtruth.Timeline, error) {
	if scope.Token == "" {
		return nil, nil
	}
	if gtLog == "" {
		return nil, fmt.Errorf("analyzer: token=%q scope requires --gt-log", scope.Token)
	}
	recs, err := groundtruth.ParseLogFile(gtLog)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("analyzer: no CRITICAST_GT records in %s", gtLog)
	}
	return groundtruth.NewTimeline(recs), nil
}

func nodeGoid(n NodeID) (uint64, bool) {
	u := uint64(n)
	if u&tidNodeBit != 0 || u == 0 {
		return 0, false
	}
	return u, true
}

func edgeMatchesToken(e WaitEdge, token string, tl *groundtruth.Timeline, ts time.Time) bool {
	if g, ok := nodeGoid(e.To); ok && tl.TokenAt(g, ts) == token {
		return true
	}
	if g, ok := nodeGoid(e.From); ok && tl.TokenAt(g, ts) == token {
		return true
	}
	return false
}

// enrichScopedEdgesFromGT applies site→mechanism labels from GT for token scope (Bar B / eval).
// BPF wait_class is often WC_UNKNOWN on live captures without sudog.elem.
func enrichScopedEdgesFromGT(edges []WaitEdge, env scopeEnv) {
	if env.scope.Token == "" || env.tl == nil {
		return
	}
	if env.hdr.KtimeBaseNs == 0 || env.hdr.WallBaseUTC == "" {
		return
	}
	for i := range edges {
		wakee, ok := env.nodeTaskID(edges[i].To)
		if !ok {
			continue
		}
		if len(env.requestGoids) > 0 {
			if _, ok := env.requestGoids[wakee]; !ok {
				continue
			}
		} else if env.tl.TokenAt(wakee, env.hdr.EventWallTime(edges[i].EndNs)) != env.scope.Token {
			continue
		}
		ts := env.hdr.EventWallTime(edges[i].EndNs)
		mech := env.tl.MechanismAt(wakee, ts)
		wc, ok := env.traceWaitCls[wakee]
		if !ok {
			wc = waitClassForMechanism(mech)
		}
		if mech == mechanism.Unknown && wc == event.WCUnknown {
			continue
		}
		if mech == mechanism.Unknown {
			mech = attribution.MechanismFromWaitClass(wc)
		}
		edges[i].WaitClass = wc
		edges[i].Meta = attribution.MetaForMechanism(mech, wc, edges[i].Aux, edges[i].Cookie)
	}
}

func waitClassForMechanism(mech string) event.WaitClass {
	switch mech {
	case mechanism.ChanWorkHandoff:
		return event.WCChan
	case mechanism.Mutex:
		return event.WCMutex
	case mechanism.ConnPool:
		// Display: use DisplayWaitClass / mechanism field — do not diagnose pool as channel.
		return event.WCUnknown
	case mechanism.Netpoll:
		return event.WCNet
	default:
		return event.WCUnknown
	}
}

func (env scopeEnv) matchesEdge(e WaitEdge) bool {
	if !env.scoped {
		return true
	}
	if !env.edgeInWindow(e) {
		return false
	}
	if len(env.requestGoids) > 0 {
		return env.edgeTouchesRequestGoids(e)
	}
	if env.scope.Token != "" && env.tl != nil {
		if env.hdr.KtimeBaseNs == 0 || env.hdr.WallBaseUTC == "" {
			return false
		}
		if edgeMatchesToken(e, env.scope.Token, env.tl, env.hdr.EventWallTime(e.EndNs)) {
			return true
		}
	}
	return EdgeMatchesScope(e, env.scope, true)
}
