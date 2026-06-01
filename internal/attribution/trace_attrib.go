package attribution

import (
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/mechanism"
)

// TraceEdgeMeta is production attribution for one wait-for edge (Tier-0/1 default).
type TraceEdgeMeta struct {
	Mechanism  string
	Confidence uint8
	Ambiguous  bool
}

// MechanismFromWaitClass maps BPF wait_class to eval mechanism name.
func MechanismFromWaitClass(wc event.WaitClass) string {
	switch wc {
	case event.WCMutex, event.WCFutex, event.WCSema:
		return mechanism.Mutex
	case event.WCChan:
		return mechanism.ChanWorkHandoff
	case event.WCNet, event.WCEpoll:
		return mechanism.Netpoll
	default:
		return mechanism.Unknown
	}
}

// AttributeTraceEdge applies E3 resource suppression and Tier-2 elem gating.
// Never returns high confidence for chan without aux (sudog.elem).
// P2: replace flat confidence=60 fallback with charter C.3.6 model (see CHARTER.md).
func AttributeTraceEdge(wc event.WaitClass, aux uint64, cookie uint64) TraceEdgeMeta {
	mech := MechanismFromWaitClass(wc)
	meta := TraceEdgeMeta{Mechanism: mech, Confidence: 70, Ambiguous: false}

	if IsResourceMechanism(mech) {
		meta.Ambiguous = true
		meta.Confidence = 50
		return meta
	}
	if wc == event.WCChan {
		if aux == 0 {
			meta.Ambiguous = true
			meta.Confidence = 45
			return meta
		}
		meta.Confidence = 75
		return meta
	}
	if cookie != 0 {
		meta.Confidence = 85
		return meta
	}
	meta.Confidence = 60
	return meta
}
