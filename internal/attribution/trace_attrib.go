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

// AttributeTraceEdge applies charter C.3.6 confidence tiers (E3 suppress + Tier-2 elem gating).
// Never returns high confidence for chan without aux (sudog.elem).
func AttributeTraceEdge(wc event.WaitClass, aux uint64, cookie uint64) TraceEdgeMeta {
	mech := MechanismFromWaitClass(wc)
	meta := TraceEdgeMeta{Mechanism: mech, Confidence: baseConfidence(wc), Ambiguous: false}

	if IsResourceMechanism(mech) {
		meta.Ambiguous = true
		meta.Confidence = capConfidence(meta.Confidence, 45)
		return meta
	}

	switch wc {
	case event.WCChan:
		if aux == 0 {
			meta.Ambiguous = true
			meta.Confidence = 35
			return meta
		}
		meta.Confidence = 82
		return meta
	case event.WCUnknown:
		meta.Ambiguous = true
		meta.Confidence = capConfidence(meta.Confidence, 35)
	case event.WCRunQ:
		meta.Confidence = capConfidence(meta.Confidence, 50)
	default:
		if cookie != 0 {
			meta.Confidence = boostConfidence(meta.Confidence, 12)
		}
	}
	return meta
}

func baseConfidence(wc event.WaitClass) uint8 {
	switch wc {
	case event.WCNet, event.WCDisk, event.WCEpoll:
		return 68
	case event.WCChan, event.WCMutex:
		return 60
	case event.WCRunQ:
		return 48
	default:
		return 55
	}
}

func boostConfidence(c uint8, delta uint8) uint8 {
	if int(c)+int(delta) > 95 {
		return 95
	}
	return c + delta
}

func capConfidence(c, max uint8) uint8 {
	if c > max {
		return max
	}
	return c
}
