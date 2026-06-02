package analyzer

import (
	"testing"

	"github.com/criticast/criticast/internal/attribution"
	"github.com/criticast/criticast/internal/event"
)

func TestFilterByConfidenceZeroKeepsAmbiguous(t *testing.T) {
	edges := []WaitEdge{{
		From: 1, To: 2, BlockedNs: 1000,
		WaitClass: event.WCUnknown,
		Meta:      attribution.TraceEdgeMeta{Ambiguous: true, Confidence: 35},
	}}
	kept, amb := FilterByConfidence(edges, 0)
	if len(kept) != 1 || len(amb) != 0 {
		t.Fatalf("minConf=0: kept=%d amb=%d", len(kept), len(amb))
	}
}

func TestFilterByConfidenceHighExcludesAmbiguous(t *testing.T) {
	edges := []WaitEdge{{
		From: 1, To: 2, BlockedNs: 1000,
		Meta: attribution.TraceEdgeMeta{Ambiguous: true, Confidence: 35},
	}}
	kept, amb := FilterByConfidence(edges, 70)
	if len(kept) != 0 || len(amb) != 1 {
		t.Fatalf("minConf=70: kept=%d amb=%d", len(kept), len(amb))
	}
}
