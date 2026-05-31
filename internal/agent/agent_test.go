package agent

import (
	"testing"
	"unsafe"

	"github.com/criticast/criticast/internal/event"
)

func TestDecodeEventSize(t *testing.T) {
	if unsafe.Sizeof(event.Event{}) != event.Size {
		t.Fatalf("event size = %d want %d", unsafe.Sizeof(event.Event{}), event.Size)
	}
}

func TestDecodeEventTooShort(t *testing.T) {
	_, ok := decodeEvent(make([]byte, 10))
	if ok {
		t.Fatal("expected false for short buffer")
	}
}
