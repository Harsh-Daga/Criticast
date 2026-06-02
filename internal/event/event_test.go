package event

import (
	"testing"
	"unsafe"
)

func TestEventSizeMatchesKernel(t *testing.T) {
	if unsafe.Sizeof(Event{}) != Size {
		t.Fatalf("Go Event size %d != kernel %d", unsafe.Sizeof(Event{}), Size)
	}
}

func TestStatIndices(t *testing.T) {
	if StatMax != 11 {
		t.Fatalf("StatMax=%d want 11", StatMax)
	}
}
