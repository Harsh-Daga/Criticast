package trace

import (
	"bytes"
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestWriteReadRoundTrip(t *testing.T) {
	events := []event.Event{
		{TsNs: 1, Type: event.EVBlockEnd, Tgid: 100, Tid: 101, BlockedNs: 5000},
	}
	var buf bytes.Buffer
	hdr := Header{Tgid: 100, MinBlock: 1000, SampleMod: 1}
	if err := Write(&buf, hdr, events); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 1 || got.Events[0].BlockedNs != 5000 {
		t.Fatalf("got %+v", got.Events)
	}
}
