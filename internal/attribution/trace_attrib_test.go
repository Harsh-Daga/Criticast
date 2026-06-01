package attribution

import (
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestAttributeTraceEdgeChanWithoutElem(t *testing.T) {
	m := AttributeTraceEdge(event.WCChan, 0, 0)
	if !m.Ambiguous || m.Confidence > 50 {
		t.Fatalf("got %+v", m)
	}
}

func TestAttributeTraceEdgeChanWithElem(t *testing.T) {
	m := AttributeTraceEdge(event.WCChan, 0xdead, 0)
	if m.Ambiguous || m.Confidence < 70 {
		t.Fatalf("got %+v", m)
	}
}

func TestAttributeTraceEdgeMutex(t *testing.T) {
	m := AttributeTraceEdge(event.WCMutex, 0, 0x1)
	if !m.Ambiguous {
		t.Fatalf("got %+v", m)
	}
}
