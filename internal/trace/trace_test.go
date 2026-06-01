package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/criticast/criticast/internal/event"
)

func TestWriteReadRoundTripV2(t *testing.T) {
	events := []event.Event{
		{TsNs: 1, Type: event.EVBlockEnd, Tgid: 100, Tid: 101, BlockedNs: 5000},
	}
	var buf bytes.Buffer
	hdr := Header{Tgid: 100, MinBlock: 1000, SampleMod: 1}
	opts := &WriteOptions{
		Stacks: map[int32][]uint64{3: {0x401000}},
		Footer: &Footer{UserspaceReceived: 1, BPFEventsEmitted: 2},
	}
	if err := Write(&buf, hdr, events, opts); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != Version2 || got.Header.Tgid != 100 {
		t.Fatalf("header: format=%d %+v", got.Format, got.Header)
	}
	if len(got.Events) != 1 || got.Events[0].BlockedNs != 5000 {
		t.Fatalf("events: %+v", got.Events)
	}
	if len(got.Stacks[3]) != 1 || got.Stacks[3][0] != 0x401000 {
		t.Fatalf("stacks: %+v", got.Stacks)
	}
	if got.Footer == nil || got.Footer.UserspaceReceived != 1 {
		t.Fatalf("footer: %+v", got.Footer)
	}
}

func TestReadV1Compat(t *testing.T) {
	// P0 JSONL: version-1 header + events only (no sections).
	var buf bytes.Buffer
	hdr := Header{Version: Version1, Tgid: 42, MinBlock: 100, SampleMod: 1}
	b, _ := json.Marshal(hdr)
	buf.WriteString(string(b) + "\n")
	enc := json.NewEncoder(&buf)
	_ = enc.Encode(event.Event{TsNs: 99, Type: event.EVBlockEnd, BlockedNs: 10})

	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != Version1 || len(got.Events) != 1 {
		t.Fatalf("format=%d events=%d", got.Format, len(got.Events))
	}
}

func TestV2HeaderMagic(t *testing.T) {
	var buf bytes.Buffer
	_ = Write(&buf, Header{Tgid: 1}, nil, nil)
	first := strings.SplitN(buf.String(), "\n", 2)[0]
	var h V2Header
	if err := json.Unmarshal([]byte(first), &h); err != nil {
		t.Fatal(err)
	}
	if h.Magic != Magic || h.Version != Version2 {
		t.Fatalf("header: %+v", h)
	}
}
