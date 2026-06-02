package attribution

import (
	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/mechanism"
)

// MetaForMechanism builds trace-edge metadata from GT mechanism name (Bar B display).
func MetaForMechanism(mech string, wc event.WaitClass, aux, cookie uint64) TraceEdgeMeta {
	if mech == mechanism.Unknown || mech == "" {
		return AttributeTraceEdge(wc, aux, cookie)
	}
	meta := TraceEdgeMeta{Mechanism: mech, Confidence: baseConfidence(wc), Ambiguous: false}
	if IsResourceMechanism(mech) {
		meta.Ambiguous = true
		meta.Confidence = capConfidence(meta.Confidence, 45)
		return meta
	}
	if mech == mechanism.ChanWorkHandoff {
		if aux == 0 {
			meta.Ambiguous = true
			meta.Confidence = 35
		} else {
			meta.Confidence = 82
		}
		return meta
	}
	if cookie != 0 {
		meta.Confidence = boostConfidence(meta.Confidence, 12)
	}
	return meta
}
